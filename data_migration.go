package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
)

// prepareDataDir makes sure this build's data folder is there, and brings the
// one of earlier builds into place: the entries, the cached icons and the
// window's geometry live beside the executable, so a renamed folder would hide
// them all. It then rewrites the store file's references to the old name, so the
// file on disk agrees with what the launcher reads back.
func prepareDataDir() {
	target := filepath.Join(absBase, dataDir)
	legacy := filepath.Join(absBase, legacyDataDir)

	if _, err := os.Stat(legacy); err == nil {
		if _, err := os.Stat(target); err != nil {
			if err := os.Rename(legacy, target); err != nil {
				// A file in the old folder is held open (an icon being read,
				// say): copy what is there instead, and leave the old folder
				// for the user to delete.
				copyTree(legacy, target)
			}
		}
	}
	_ = os.MkdirAll(target, 0o755)
	migrateStoreFile()
}

// migrateStoreFile replaces the old folder's name in the store file: the icon
// paths it holds are relative, so they would point into a folder that no longer
// exists. Parsing does this in memory too, for paths added by hand later.
func migrateStoreFile() {
	path := filepath.Join(absBase, saveFile)
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte(legacyDataDir)) {
		return
	}
	data = bytes.ReplaceAll(data, []byte(legacyDataDir), []byte(dataDir))
	_ = os.WriteFile(path, data, 0o644)
}

// copyTree copies a folder's files and subfolders into to, creating it. Files it
// cannot read are skipped: a missing icon is better than a launcher that does
// not start.
func copyTree(from, to string) {
	_ = filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return nil
		}
		dst := filepath.Join(to, rel)
		if entry.IsDir() {
			_ = os.MkdirAll(dst, 0o755)
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		_ = os.WriteFile(dst, data, 0o644)
		return nil
	})
}
