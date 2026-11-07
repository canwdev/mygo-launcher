package main

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// dataBaseOverride is where mygo-launcher-data lives when it is not beside
// the executable: %LOCALAPPDATA%\MyGo Launcher for an installed copy.
var dataBaseOverride string

// dataLegacyDir is the folder older installed versions kept the data in,
// when startup moved it there: absolute icon paths under it are rewritten.
var dataLegacyDir string

// dataBase is the folder holding mygo-launcher-data: the working directory
// in development and beside the executable for a portable copy, the user's
// local application data for an installed copy.
func dataBase() string {
	if dataBaseOverride != "" {
		return dataBaseOverride
	}
	return absBase
}

// dataPath resolves a path stored relative to dataBase, which icons are.
func dataPath(p string) string {
	if p == "" || filepath.IsAbs(p) || isShellTarget(p) {
		return p
	}
	return filepath.Join(dataBase(), p)
}

// prepareDataBase picks the data folder before anything reads it. A
// `mygo build` binary runs from its own folder: beside it for a portable
// copy, and in %LOCALAPPDATA%\MyGo Launcher for an installed one, which the
// installer records in the uninstall entry. Development keeps the data in
// the working directory, as before.
func prepareDataBase() {
	if mygo.IsDev() {
		return
	}
	absBase = installDir()
	base, ok := installedDataBase()
	if !ok {
		return
	}
	legacy := filepath.Join(absBase, dataDir)
	if !migrateDataDir(legacy, filepath.Join(base, dataDir)) {
		return // keep reading the old folder rather than start empty
	}
	dataBaseOverride = base
	dataLegacyDir = legacy
}

// migrateDataDir moves the data folder installed versions kept beside the
// executable into the user's folder. It reports whether the data is ready
// at target: no folder yet and an old one that could not be moved leave the
// caller on the old path instead of an empty store.
func migrateDataDir(legacy, target string) bool {
	if _, err := os.Stat(target); err == nil {
		return true
	}
	if _, err := os.Stat(legacy); err != nil {
		return true // first run: there is nothing to move
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false
	}
	if err := os.Rename(legacy, target); err == nil {
		return true
	}
	// Another volume: copy beside the target and rename that into place, so
	// target is either complete or absent.
	tmp := target + ".migrating"
	_ = os.RemoveAll(tmp)
	if err := copyTree(legacy, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return false
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.RemoveAll(tmp)
		return false
	}
	_ = os.RemoveAll(legacy)
	return true
}

// rebaseIcons points absolute icon paths at the new data folder after a
// move. It reports whether the store changed.
func (a *App) rebaseIcons() bool {
	if dataLegacyDir == "" {
		return false
	}
	from := dataLegacyDir
	to := filepath.Join(dataBase(), dataDir)
	changed := false
	for _, item := range a.store.Apps {
		if item == nil || item.Icon == "" {
			continue
		}
		if p, ok := rebasePath(item.Icon, from, to); ok {
			item.Icon = p
			changed = true
		}
	}
	return changed
}

// rebasePath moves an absolute path under from to the same path under to.
func rebasePath(p, from, to string) (string, bool) {
	if !filepath.IsAbs(p) {
		return p, false
	}
	rel, err := filepath.Rel(from, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return p, false
	}
	return filepath.Join(to, rel), true
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	info, err := in.Stat()
	if err != nil {
		in.Close()
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		in.Close()
		return err
	}
	_, err = io.Copy(out, in)
	in.Close()
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
