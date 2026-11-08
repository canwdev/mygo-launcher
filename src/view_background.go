package main

import (
	"bytes"
	"image"
	"os"

	"github.com/egoist/mygo/ui"
)

// backgroundBitmap decodes the chosen background once, and keeps it until the
// preference points elsewhere. A picture that cannot be read draws nothing.
func (v *view) backgroundBitmap() *ui.Bitmap {
	path := v.prefs.Background
	if path == "" {
		v.bgImageKey, v.bgImage = "", nil
		return nil
	}
	if v.bgImageKey == path {
		return v.bgImage
	}
	v.bgImageKey = path
	v.bgImage = v.loadBackgroundBitmap(path)
	return v.bgImage
}

// backgroundShown reports whether a background picture will actually paint:
// the preference names a file that loads, at a visible opacity.
func (v *view) backgroundShown() bool {
	return v.prefs.Background != "" && v.prefs.BackgroundOpacity > 0 && v.backgroundBitmap() != nil
}

// loadBackgroundBitmap decodes the stored picture, kept to the same size the
// chooser would have drawn it to, so a file put there by hand cannot take more
// memory than the app's own copies.
func (v *view) loadBackgroundBitmap(rel string) *ui.Bitmap {
	data, err := os.ReadFile(dataPath(rel))
	if err != nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return ui.NewBitmap(shrinkBackground(img, v.app.backgroundTargetSide()))
}

// drawBackground paints the part of the chosen picture that lands in r, a
// part of the window: the picture covers the whole window, so the header and
// the content make one continuous background rather than two crops.
func (v *view) drawBackground(p *ui.Painter, r ui.Rect) {
	bmp := v.backgroundBitmap()
	if bmp == nil || r.W <= 0 || r.H <= 0 || v.bgWindowW <= 0 || v.bgWindowH <= 0 {
		return
	}
	alpha := float32(min(1, max(0, v.prefs.BackgroundOpacity)))
	if alpha <= 0 {
		return
	}
	w, h := bmp.Size()
	if w <= 0 || h <= 0 {
		return
	}
	winW, winH := v.bgWindowW, v.bgWindowH
	scale := max(winW/float32(w), winH/float32(h))
	if scale <= 0 {
		return
	}
	srcW, srcH := winW/scale, winH/scale
	src := ui.Rect{
		X: (float32(w)-srcW)/2 + r.X/scale,
		Y: (float32(h)-srcH)/2 + r.Y/scale,
		W: r.W / scale,
		H: r.H / scale,
	}
	p.ImageRegion(bmp, src, r, ui.ImageOptions{Tint: ui.RGB(255, 255, 255).Alpha(alpha)})
}
