package main

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"os"
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

// iconAssets are the parts the app icons are drawn from: the light and the
// dark background, the accent shapes, and the gear badge of the settings
// window.
type iconAssets struct {
	base, baseDark, accent, gear *image.NRGBA
}

var (
	iconsOnce  sync.Once
	iconsParts iconAssets
)

// gearInk is the gear badge's stroke: dark on the light background, light on
// the dark one.
var (
	gearInkLight = ui.Hex("#1d1d1f")
	gearInkDark  = ui.Hex("#f5f5f7")
)

func loadIconAssets() iconAssets {
	iconsOnce.Do(func() {
		iconsParts.base = decodeIconPNG(resources.IconBasePNG)
		iconsParts.baseDark = decodeIconPNG(resources.IconBaseDarkPNG)
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

// tintMask colors a white mask in c, keeping the mask's alpha.
func tintMask(mask *image.NRGBA, c ui.Color) *image.NRGBA {
	out := image.NewNRGBA(mask.Bounds())
	for i := 0; i < len(mask.Pix); i += 4 {
		alpha := mask.Pix[i+3]
		if alpha == 0 {
			continue
		}
		out.Pix[i] = c.R
		out.Pix[i+1] = c.G
		out.Pix[i+2] = c.B
		out.Pix[i+3] = alpha
	}
	return out
}

// appIcon draws the app's icon in accent at the assets' size, on the light or
// the dark background, with the settings gear in the corner when gear is
// true. The accent mask keeps the arrow's own translucency, so it stays a tint
// of the background rather than a solid shape.
func appIcon(accent ui.Color, dark, gear bool) *image.NRGBA {
	parts := loadIconAssets()
	base := parts.base
	if dark {
		base = parts.baseDark
	}
	if base.Bounds().Empty() || parts.accent.Bounds().Empty() {
		return nil
	}
	dst := image.NewNRGBA(base.Bounds())
	copy(dst.Pix, base.Pix)
	draw.Draw(dst, dst.Bounds(), tintMask(parts.accent, accent), image.Point{}, draw.Over)

	if gear {
		ink := gearInkLight
		if dark {
			ink = gearInkDark
		}
		mask := tintMask(parts.gear, ink)
		badge := image.NewNRGBA(image.Rect(0, 0, gearBadgePx, gearBadgePx))
		xdraw.CatmullRom.Scale(badge, badge.Bounds(), mask, mask.Bounds(), draw.Over, nil)
		x := dst.Bounds().Dx() - gearBadgePx - gearMarginPx
		y := dst.Bounds().Dy() - gearBadgePx - gearMarginPx
		draw.Draw(dst, image.Rect(x, y, x+gearBadgePx, y+gearBadgePx), badge, image.Point{}, draw.Over)
		// The badge may reach past the rounded corner: clip it to the icon.
		for i := 3; i < len(dst.Pix); i += 4 {
			if base.Pix[i] == 0 {
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

// iconDark is whether the app icons are drawn for the dark appearance.
func (v *view) iconDark() bool {
	return mygo.Theme.IsDark()
}

// iconAccent is the accent the app icons are drawn in for the current
// appearance.
func (v *view) iconAccent() ui.Color {
	return accentColor(v.prefs.Accent, v.iconDark())
}

// customAppIcon loads the chosen program icon as a square image, or nil when
// there is none or it cannot be read.
func (v *view) customAppIcon() *image.NRGBA {
	if v.prefs.AppIcon == "" {
		return nil
	}
	data, err := os.ReadFile(dataPath(v.prefs.AppIcon))
	if err != nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	// A picture put there by hand is bounded like the chooser's own copies.
	img = shrinkBackground(img, maxIconSide)
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil
	}
	side := max(w, h)
	dst := image.NewNRGBA(image.Rect(0, 0, side, side))
	at := image.Rect((side-w)/2, (side-h)/2, (side-w)/2+w, (side-h)/2+h)
	draw.Draw(dst, at, img, b.Min, draw.Over)
	return dst
}

// appIconFor draws the icon the windows keep: the user's picture when one is
// chosen, otherwise the icon of the accent color. A window of the settings
// always carries the gear, whichever icon it is.
func (v *view) appIconFor(gear bool) *image.NRGBA {
	return v.appIconForDark(gear, v.iconDark())
}

func (v *view) appIconForDark(gear, dark bool) *image.NRGBA {
	if custom := v.customAppIcon(); custom != nil {
		if gear {
			return gearIcon(custom, dark)
		}
		return custom
	}
	return appIcon(accentColor(v.prefs.Accent, dark), dark, gear)
}

// gearIcon puts the settings badge in the corner of src, at a size that
// follows the picture's.
func gearIcon(src *image.NRGBA, dark bool) *image.NRGBA {
	parts := loadIconAssets()
	if src.Bounds().Empty() || parts.gear.Bounds().Empty() {
		return src
	}
	side := src.Bounds().Dx()
	badgePx := max(1, side*gearBadgePx/512)
	margin := max(1, side*gearMarginPx/512)
	ink := gearInkLight
	if dark {
		ink = gearInkDark
	}
	mask := tintMask(parts.gear, ink)
	badge := image.NewNRGBA(image.Rect(0, 0, badgePx, badgePx))
	xdraw.CatmullRom.Scale(badge, badge.Bounds(), mask, mask.Bounds(), draw.Over, nil)
	dst := image.NewNRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	x, y := side-badgePx-margin, side-badgePx-margin
	draw.Draw(dst, image.Rect(x, y, x+badgePx, y+badgePx), badge, image.Point{}, draw.Over)
	for i := 3; i < len(dst.Pix); i += 4 {
		if src.Pix[i] == 0 {
			dst.Pix[i] = 0
		}
	}
	return dst
}

// updateAppIcons redraws the window, the settings window and the tray in the
// accent color, on the background of the current appearance.
func (v *view) updateAppIcons() {
	if v.app.win == nil {
		return // not ready, or a test: there is no icon to set
	}
	plain := v.appIconFor(false)
	if data := encodeIcon(plain, windowIconPx); data != nil {
		_ = v.app.win.SetIcon(data)
	}
	if w := v.app.settingsWin; w != nil {
		if data := encodeIcon(v.appIconFor(true), windowIconPx); data != nil {
			_ = w.SetIcon(data)
		}
	}
	if v.tray != nil {
		if data := encodeIcon(plain, trayIconPx); data != nil {
			_ = v.tray.SetIcon(data, false)
		}
	}
}

// trayIcon is the tray's icon in the accent color, or the chosen program icon.
func (v *view) trayIcon() []byte {
	return encodeIcon(v.appIconFor(false), trayIconPx)
}

// aboutIconImage is the app's icon in the appearance the settings window
// shows, for its About row. It is drawn once and kept until the accent,
// appearance or chosen icon changes.
func (v *view) aboutIconImage() ui.ImageSource {
	key := v.prefs.Accent + "/" + strconv.FormatBool(v.pal.dark) + "/" + v.prefs.AppIcon
	if v.aboutIcon != nil && v.aboutIconKey == key {
		return v.aboutIcon
	}
	src := v.appIconForDark(false, v.pal.dark)
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
