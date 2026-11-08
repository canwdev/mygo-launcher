package main

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"golang.org/x/image/draw"

	"go-launcher/locales"
)

// A background picture is kept no larger than it can show: the longest side
// follows the sharpest display, within a cap that keeps its pixels reasonable.
// Pictures without transparency become JPEG, whose quality keeps them sharp
// while the file and its decoding stay small.
const (
	maxBackgroundSide     = 2560
	minBackgroundSide     = 1280
	defaultBackgroundSide = 1920
	backgroundJPEGQuality = 90
	maxBackgroundPixels   = 40_000_000
	maxBackgroundFileSize = 64 << 20
	maxBackgroundBytes    = 32 << 20
)

// backgroundTargetSide is the longest side a background needs on the displays
// in use, capped. Without a window, as in tests, the default is enough.
func (a *App) backgroundTargetSide() int {
	side := defaultBackgroundSide
	if a.win != nil {
		side = minBackgroundSide
		for _, d := range mygo.Screen.Displays() {
			px := int(float64(max(d.Bounds.Width, d.Bounds.Height)) * d.ScaleFactor)
			side = max(side, px)
		}
	}
	return min(maxBackgroundSide, side)
}

// shrinkBackground draws img down so its longer side is at most side, with a
// smooth filter, or returns it as it is when it already fits.
func shrinkBackground(img image.Image, side int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if max(w, h) <= side || w < 1 || h < 1 {
		return img
	}
	scale := float64(side) / float64(max(w, h))
	nw, nh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

// opaqueBackground reports whether every pixel is opaque, so that JPEG can
// keep the picture without losing anything.
func opaqueBackground(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}

// backgroundImageExts are the pictures a launcher background may be: anything
// the view can decode as a bitmap. An SVG is not one: it would have to be
// drawn at the window's size, not scaled as a picture.
var backgroundImageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".bmp": true, ".webp": true,
}

// customBackground copies the picture at path into the custom folder, sized
// for the displays and encoded compactly, and returns the stored path.
func (a *App) customBackground(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if !backgroundImageExts[strings.ToLower(filepath.Ext(path))] {
		return "", errors.New(locales.BackgroundInvalid())
	}
	full := absPath(path)
	info, err := os.Stat(full)
	if err != nil {
		return "", errors.New(locales.BackgroundInvalid())
	}
	if info.Size() > maxBackgroundFileSize {
		return "", errors.New(locales.IconTooLarge())
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", errors.New(locales.BackgroundInvalid())
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", errors.New(locales.BackgroundInvalid())
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxBackgroundPixels {
		return "", errors.New(locales.IconTooLarge())
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errors.New(locales.BackgroundInvalid())
	}
	img = shrinkBackground(img, a.backgroundTargetSide())

	var buf bytes.Buffer
	ext := ".png"
	if opaqueBackground(img) {
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: backgroundJPEGQuality}); err == nil {
			ext = ".jpg"
		} else {
			buf.Reset()
		}
	}
	if ext == ".png" {
		if err := png.Encode(&buf, img); err != nil {
			return "", errors.New(locales.BackgroundInvalid())
		}
	}
	if buf.Len() > maxBackgroundBytes {
		return "", errors.New(locales.IconTooLarge())
	}
	if stored := saveCustomIcon(buf.Bytes(), ext, "background"); stored != "" {
		return stored, nil
	}
	return "", errors.New(locales.BackgroundInvalid())
}
