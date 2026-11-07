package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"go-launcher/locales"
)

// iconPixels is the size icons are asked for, in image pixels: 64 DIPs, the
// size of the list's icon, on the sharpest display attached so the icon stays
// crisp when scaled.
func iconPixels() int {
	return iconPixelsFor(64)
}

// iconPixelsFor is the pixel size of a dip size on the sharpest display
// attached.
func iconPixelsFor(dip float32) int {
	scale := 1.0
	for _, d := range mygo.Screen.Displays() {
		scale = max(scale, d.ScaleFactor)
	}
	return int(dip * float32(scale))
}

// saveIcon writes image bytes into the cache folder under their own extension,
// and returns the path the store keeps (relative to dataBase, as icons always
// have been).
func saveIcon(data []byte, ext, nameBase string) string {
	return saveIconIn(iconsDir, data, ext, nameBase)
}

// saveCustomIcon writes an icon the user chose into the custom folder.
func saveCustomIcon(data []byte, ext, nameBase string) string {
	return saveIconIn(customIconsDir, data, ext, nameBase)
}

func saveIconIn(dir string, data []byte, ext, nameBase string) string {
	full := filepath.Join(dataBase(), dir)
	if err := os.MkdirAll(full, 0o755); err != nil {
		return ""
	}
	if ext == "" {
		ext = ".png"
	}
	if nameBase == "" {
		nameBase = "icon"
	}
	name := fmt.Sprintf("%s-%d%s", sanitizeBase(nameBase), time.Now().UnixNano(), ext)
	if err := os.WriteFile(filepath.Join(full, name), data, 0o644); err != nil {
		return ""
	}
	return "./" + filepath.ToSlash(filepath.Join(dir, name))
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

// iconImageExts are the files the icon field reads as images; other files hold
// icons in resources, which customIcon draws out.
var iconImageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".bmp": true, ".webp": true, ".svg": true,
}

// isIconImage reports whether path is an image the view shows as it is.
func isIconImage(path string) bool {
	return iconImageExts[strings.ToLower(filepath.Ext(path))]
}

// iconTakesIndex reports whether path holds icons to pick from: an .ico, or
// the icon resources of an .exe or .dll.
func iconTakesIndex(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ico", ".exe", ".dll":
		return true
	}
	return false
}

// The largest side of a custom icon, and the most its image may hold: a big
// picture is drawn down to this, so the cached icon stays small.
const (
	maxIconSide     = 256
	maxIconBytes    = 10 << 20
	maxIconPixels   = 40_000_000
	maxIconFileSize = 64 << 20
)

// customIcon copies the icon at path into the custom icon folder and returns
// its stored path: an image is drawn down to maxIconSide, and index picks the
// icon of an .ico, .exe or .dll. A file already in that folder, or an icon the
// launcher generated, is kept as it is.
func (a *App) customIcon(path string, index int) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if isCustomIcon(path) || isAutoIcon(path) {
		return path, nil
	}
	full := absPath(path)
	name := defaultTitle(filepath.Base(full))

	if isIconImage(path) {
		data, ext, err := normalizeIconImage(full)
		if err != nil {
			return "", err
		}
		if icon := saveCustomIcon(data, ext, name); icon != "" {
			return icon, nil
		}
		return "", errors.New(locales.IconFailed())
	}
	if iconTakesIndex(path) {
		data := extractIconResource(full, maxIconSide, index)
		if len(data) == 0 {
			return "", errors.New(locales.IconIndexInvalid())
		}
		if icon := saveCustomIcon(data, ".png", name); icon != "" {
			return icon, nil
		}
		return "", errors.New(locales.IconFailed())
	}
	// Any other file: the icon the shell draws for it.
	if a.win != nil {
		if data, err := mygo.Shell.FileIcon(full, maxIconSide); err == nil && len(data) > 0 {
			if icon := saveCustomIcon(data, ".png", name); icon != "" {
				return icon, nil
			}
		}
	}
	return "", errors.New(locales.IconInvalid())
}

// normalizeIconImage reads an image as PNG data no larger than maxIconSide,
// or an SVG as it is.
func normalizeIconImage(path string) ([]byte, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", errors.New(locales.IconInvalid())
	}
	if info.Size() > maxIconFileSize {
		return nil, "", errors.New(locales.IconTooLarge())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", errors.New(locales.IconInvalid())
	}
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		if len(data) > maxIconBytes {
			return nil, "", errors.New(locales.IconTooLarge())
		}
		return data, ".svg", nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", errors.New(locales.IconInvalid())
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxIconPixels {
		return nil, "", errors.New(locales.IconTooLarge())
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", errors.New(locales.IconInvalid())
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if max(w, h) > maxIconSide {
		scale := float64(maxIconSide) / float64(max(w, h))
		nw, nh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
		dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", errors.New(locales.IconInvalid())
	}
	if buf.Len() > maxIconBytes {
		return nil, "", errors.New(locales.IconTooLarge())
	}
	return buf.Bytes(), ".png", nil
}

// contentIcon draws the picture a file holds as its own icon: an image shows
// itself, and an .ico its first icon, rather than the icon of its file type. It
// returns "" when the file holds no picture the launcher can draw. The icon is
// generated, so it goes in the cache folder, as writeIcon's does.
func contentIcon(path string) string {
	name := defaultTitle(filepath.Base(path))
	if strings.EqualFold(filepath.Ext(path), ".ico") {
		if data := extractIconResource(path, maxIconSide, 0); len(data) > 0 {
			return saveIcon(data, ".png", name)
		}
		return ""
	}
	if !isIconImage(path) {
		return ""
	}
	data, ext, err := normalizeIconImage(path)
	if err != nil {
		return ""
	}
	return saveIcon(data, ext, name)
}

// iconSource decodes the image an item's icon field points at: a bitmap of any
// common format, or an SVG drawn in its own colors.
func iconSource(rel string) ui.ImageSource {
	if rel == "" {
		return nil
	}
	data, err := os.ReadFile(dataPath(rel))
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
// itself, and can extract again ("Update icon").
func isAutoIcon(icon string) bool {
	if icon == "" {
		return true
	}
	return strings.Contains(strings.ReplaceAll(icon, "\\", "/"), dataDir+"/cached-icons/")
}

// isCustomIcon reports whether an item's icon is one the user chose, kept in
// the custom folder.
func isCustomIcon(icon string) bool {
	return icon != "" && strings.Contains(strings.ReplaceAll(icon, "\\", "/"), customIconsDir+"/")
}

// allowedIcon reports whether the view shows icon: one the launcher generated,
// one the user chose, or none. Paths elsewhere are not the app's icons.
func allowedIcon(icon string) bool {
	return icon == "" || isAutoIcon(icon) || isCustomIcon(icon)
}

// pruneIcons removes icons no item refers to any more, from both folders.
func (a *App) pruneIcons() {
	referenced := make(map[string]bool)
	for _, app := range a.store.Apps {
		if app != nil && app.Icon != "" {
			referenced[filepath.Base(app.Icon)] = true
		}
	}
	for _, dir := range []string{iconsDir, customIconsDir} {
		full := filepath.Join(dataBase(), dir)
		entries, err := os.ReadDir(full)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || referenced[e.Name()] {
				continue
			}
			_ = os.Remove(filepath.Join(full, e.Name()))
		}
	}
}
