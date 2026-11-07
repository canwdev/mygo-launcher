package main

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"strconv"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	xdraw "golang.org/x/image/draw"

	"go-launcher/resources"
)

const (
	// windowIconPx is the icon the windows keep, and trayIconPx the tray's:
	// Windows scales them to the taskbar and title bar.
	windowIconPx = 256
	trayIconPx   = 64

	// gearBadgePx and gearMarginPx place the settings gear in the 512 pixel
	// icon the assets are drawn at.
	gearBadgePx  = 200
	gearMarginPx = 16
)

// iconAssets are the parts the app icons are drawn from: the background, the
// accent shapes, and the gear badge of the settings window.
type iconAssets struct {
	base, accent, gear *image.NRGBA
}

var (
	iconsOnce  sync.Once
	iconsParts iconAssets
)

func loadIconAssets() iconAssets {
	iconsOnce.Do(func() {
		iconsParts.base = decodeIconPNG(resources.IconBasePNG)
		iconsParts.accent = decodeIconPNG(resources.IconAccentPNG)
		iconsParts.gear = decodeIconPNG(resources.IconGearPNG)
	})
	return iconsParts
}

// decodeIconPNG decodes a PNG into an image the composition can change.
func decodeIconPNG(data []byte) *image.NRGBA {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return image.NewNRGBA(image.Rect(0, 0, 0, 0))
	}
	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// appIcon draws the app's icon in accent at the assets' size, with the
// settings gear in the corner when gear is true. The accent mask keeps the
// arrow's own translucency, so it stays a tint of the background rather than
// a solid shape.
func appIcon(accent ui.Color, gear bool) *image.NRGBA {
	parts := loadIconAssets()
	if parts.base.Bounds().Empty() || parts.accent.Bounds().Empty() {
		return nil
	}
	dst := image.NewNRGBA(parts.base.Bounds())
	copy(dst.Pix, parts.base.Pix)

	tinted := image.NewNRGBA(parts.accent.Bounds())
	for i := 0; i < len(parts.accent.Pix); i += 4 {
		alpha := parts.accent.Pix[i+3]
		if alpha == 0 {
			continue
		}
		tinted.Pix[i] = accent.R
		tinted.Pix[i+1] = accent.G
		tinted.Pix[i+2] = accent.B
		tinted.Pix[i+3] = alpha
	}
	draw.Draw(dst, dst.Bounds(), tinted, image.Point{}, draw.Over)

	if gear {
		badge := image.NewNRGBA(image.Rect(0, 0, gearBadgePx, gearBadgePx))
		xdraw.CatmullRom.Scale(badge, badge.Bounds(), parts.gear, parts.gear.Bounds(), draw.Over, nil)
		x := dst.Bounds().Dx() - gearBadgePx - gearMarginPx
		y := dst.Bounds().Dy() - gearBadgePx - gearMarginPx
		draw.Draw(dst, image.Rect(x, y, x+gearBadgePx, y+gearBadgePx), badge, image.Point{}, draw.Over)
		// The badge may reach past the rounded corner: clip it to the icon.
		for i := 3; i < len(dst.Pix); i += 4 {
			if parts.base.Pix[i] == 0 {
				dst.Pix[i] = 0
			}
		}
	}
	return dst
}

// encodeIcon scales the drawn icon to px and encodes it as a PNG, or returns
// nil when the assets could not be read.
func encodeIcon(src *image.NRGBA, px int) []byte {
	if src == nil {
		return nil
	}
	var img image.Image = src
	if src.Bounds().Dx() != px {
		scaled := image.NewNRGBA(image.Rect(0, 0, px, px))
		xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), draw.Over, nil)
		img = scaled
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// iconAccent is the accent the app icons are drawn in for the current
// appearance.
func (v *view) iconAccent() ui.Color {
	return accentColor(v.prefs.Accent, mygo.Theme.IsDark())
}

// updateAppIcons redraws the window, the settings window and the tray in the
// accent color.
func (v *view) updateAppIcons() {
	if v.app.win == nil {
		return // not ready, or a test: there is no icon to set
	}
	accent := v.iconAccent()
	plain := appIcon(accent, false)
	if data := encodeIcon(plain, windowIconPx); data != nil {
		_ = v.app.win.SetIcon(data)
	}
	if w := v.app.settingsWin; w != nil {
		if data := encodeIcon(appIcon(accent, true), windowIconPx); data != nil {
			_ = w.SetIcon(data)
		}
	}
	if v.tray != nil {
		if data := encodeIcon(plain, trayIconPx); data != nil {
			_ = v.tray.SetIcon(data, false)
		}
	}
}

// trayIcon is the tray's icon in the accent color.
func (v *view) trayIcon() []byte {
	return encodeIcon(appIcon(v.iconAccent(), false), trayIconPx)
}

// aboutIconImage is the app's icon in the appearance the settings window
// shows, for its About row. It is drawn once and kept until the accent or
// appearance changes.
func (v *view) aboutIconImage() ui.ImageSource {
	key := v.prefs.Accent + "/" + strconv.FormatBool(v.pal.dark)
	if v.aboutIcon != nil && v.aboutIconKey == key {
		return v.aboutIcon
	}
	src := appIcon(accentColor(v.prefs.Accent, v.pal.dark), false)
	if src == nil {
		return nil
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), draw.Over, nil)
	v.aboutIcon, v.aboutIconKey = ui.NewBitmap(scaled), key
	return v.aboutIcon
}

// scheduleAppIconUpdate redraws the icons once the accent picker settles: a
// color dragged through the picker would otherwise set an icon per frame.
func (v *view) scheduleAppIconUpdate() {
	if v.app.win == nil {
		return
	}
	if v.iconTimer != nil {
		v.iconTimer.Stop()
	}
	v.iconTimer = time.AfterFunc(180*time.Millisecond, func() {
		if v.app.quitting {
			return
		}
		v.app.dispatch(func() {
			if !v.app.quitting {
				v.updateAppIcons()
			}
		})
	})
}
