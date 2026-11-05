package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// iconPixels is the size icons are asked for, in image pixels: 64 DIPs, the
// size of the list's icon, on the sharpest display attached so the icon stays
// crisp when scaled.
func iconPixels() int {
	scale := 1.0
	for _, d := range mygo.Screen.Displays() {
		scale = max(scale, d.ScaleFactor)
	}
	return int(64 * scale)
}

// saveIcon writes image bytes into the cache folder under their own extension,
// and returns the path the store keeps (relative to absBase, as icons always
// have been).
func saveIcon(data []byte, ext, nameBase string) string {
	dir := filepath.Join(absBase, iconsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	if ext == "" {
		ext = ".png"
	}
	name := fmt.Sprintf("%s-%d%s", sanitizeBase(nameBase), time.Now().UnixNano(), ext)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return ""
	}
	return "./" + filepath.ToSlash(filepath.Join(iconsDir, name))
}

// writeIcon extracts the icon the file manager shows for path and caches it as
// a PNG.
func writeIcon(path string) string {
	data, err := mygo.Shell.FileIcon(path, iconPixels())
	if err != nil || len(data) == 0 {
		return ""
	}
	return saveIcon(data, ".png", filepath.Base(path))
}

// iconSource decodes the image an item's icon field points at: a bitmap of any
// common format, or an SVG drawn in its own colors.
func iconSource(rel string) ui.ImageSource {
	if rel == "" {
		return nil
	}
	data, err := os.ReadFile(absPath(rel))
	if err != nil {
		return nil
	}
	src, err := ui.DecodeImage(data)
	if err != nil {
		return nil
	}
	return src
}

// isAutoIcon reports whether an item's icon is one the launcher extracted
// itself, and can extract again ("Update icon"). Icons of builds before 0.2.0
// name the old data folder, and are the launcher's own too.
func isAutoIcon(icon string) bool {
	if icon == "" {
		return true
	}
	slashed := strings.ReplaceAll(icon, "\\", "/")
	return strings.Contains(slashed, dataDir+"/cached-icons/") ||
		strings.Contains(slashed, legacyDataDir+"/cached-icons/")
}

// pruneIconFiles removes cached icons no item refers to any more.
func (a *App) pruneIconFiles() {
	referenced := make(map[string]bool)
	for _, app := range a.store.Apps {
		if app != nil && app.Icon != "" {
			referenced[filepath.Base(app.Icon)] = true
		}
	}
	entries, err := os.ReadDir(filepath.Join(absBase, iconsDir))
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || referenced[e.Name()] {
			continue
		}
		_ = os.Remove(filepath.Join(absBase, iconsDir, e.Name()))
	}
}
