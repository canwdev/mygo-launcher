package main

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// palette holds the colors the window draws with, for the light and the dark
// appearance. It follows MyGo Music's palette, so the two apps look alike.
type palette struct {
	dark     bool
	bg       ui.Color // the content's background
	sidebar  ui.Color
	text     ui.Color
	muted    ui.Color
	faint    ui.Color
	hairline ui.Color
	hover    ui.Color
	press    ui.Color
	accent   ui.Color
	onAccent ui.Color
	shade    ui.Color // under cards and lifted artwork
}

// The colors of MyGO!!!!!'s members: Tomori, Anon, Rāna, Soyo and Taki.
//
// accentPresets are the accent colors the settings offer, from the website's
// --bang-1 to --bang-5, each with its light and dark appearance. The id is what
// ui-state.json keeps; the custom option keeps a #rrggbb instead.
var accentPresets = []struct {
	id, label   string
	light, dark ui.Color
}{
	{"bang-1", "Tomori", ui.Oklch(0.66, 0.084, 231), ui.Oklch(0.76, 0.084, 231)},
	{"bang-2", "Anon", ui.Oklch(0.69, 0.145, 12), ui.Oklch(0.76, 0.145, 12)},
	{"bang-3", "Rāna", ui.Oklch(0.65, 0.169, 144), ui.Oklch(0.81, 0.169, 144)},
	{"bang-4", "Soyo", ui.Oklch(0.67, 0.111, 88), ui.Oklch(0.91, 0.111, 88)},
	{"bang-5", "Taki", ui.Oklch(0.59, 0.078, 284), ui.Oklch(0.59, 0.078, 284)},
}

// defaultAccent is the preset used when none is chosen: --bang-1.
const defaultAccent = "bang-1"

func paletteFor(t *ui.Theme, accent string) palette {
	p := palette{dark: t.Dark}
	if t.Dark {
		p.bg = ui.Hex("#141416")
		p.sidebar = ui.Hex("#1c1c1f")
		p.text = ui.Hex("#f5f5f7")
		p.muted = ui.RGBA(235, 235, 245, 0.56)
		p.faint = ui.RGBA(235, 235, 245, 0.3)
		p.hairline = ui.RGBA(255, 255, 255, 0.07)
		p.hover = ui.RGBA(255, 255, 255, 0.06)
		p.press = ui.RGBA(255, 255, 255, 0.1)
		p.shade = ui.RGBA(0, 0, 0, 0.45)
	} else {
		p.bg = ui.Hex("#ffffff")
		p.sidebar = ui.Hex("#f4f4f6")
		p.text = ui.Hex("#1d1d1f")
		p.muted = ui.RGBA(60, 60, 67, 0.62)
		p.faint = ui.RGBA(60, 60, 67, 0.32)
		p.hairline = ui.RGBA(0, 0, 0, 0.07)
		p.hover = ui.RGBA(0, 0, 0, 0.04)
		p.press = ui.RGBA(0, 0, 0, 0.08)
		p.shade = ui.RGBA(0, 0, 0, 0.16)
	}
	p.accent = accentColor(accent, t.Dark)
	p.onAccent = accentText(p.accent)
	return p
}

// accentColor is the accent the preference asks for: a preset's color for that
// appearance, a custom #rrggbb, or the default --bang-1.
func accentColor(pref string, dark bool) ui.Color {
	if pref == "" {
		pref = defaultAccent
	}
	if c, ok := presetColor(pref, dark); ok {
		return c
	}
	if c, ok := parseAccentHex(pref); ok {
		return c
	}
	c, _ := presetColor(defaultAccent, dark)
	return c
}

// presetColor returns a preset's color for the light or dark appearance.
func presetColor(id string, dark bool) (ui.Color, bool) {
	for _, a := range accentPresets {
		if a.id == id {
			if dark {
				return a.dark, true
			}
			return a.light, true
		}
	}
	return ui.Color{}, false
}

// validAccent reports whether pref names a preset or a #rrggbb color, or is
// empty for the default.
func validAccent(pref string) bool {
	if pref == "" {
		return true
	}
	for _, a := range accentPresets {
		if a.id == pref {
			return true
		}
	}
	_, ok := parseAccentHex(pref)
	return ok
}

func parseAccentHex(s string) (ui.Color, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return ui.Color{}, false
	}
	for _, r := range s[1:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return ui.Color{}, false
		}
	}
	return ui.Hex(s), true
}

// hexColor spells a color as #rrggbb, which the preference keeps.
func hexColor(c ui.Color) string {
	const digits = "0123456789abcdef"
	out := []byte{'#', 0, 0, 0, 0, 0, 0}
	for i, v := range []uint8{c.R, c.G, c.B} {
		out[1+2*i] = digits[v>>4]
		out[2+2*i] = digits[v&0xf]
	}
	return string(out)
}

// accentText is the text color on an accent fill: dark on colors light enough
// that white would be hard to read, white on the rest.
func accentText(c ui.Color) ui.Color {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	lum := 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
	if lum > 0.35 {
		return ui.Hex("#1d1d1f")
	}
	return ui.Hex("#ffffff")
}

// applyPalette colors the frame's theme with the palette and returns it. The
// appearance itself (system, light or dark) is the window's, set by applyTheme.
func applyPalette(c *ui.Context, accent string) palette {
	t := *c.Theme()
	p := paletteFor(&t, accent)
	t.Accent, t.AccentText = p.accent, p.onAccent
	t.Selection = p.accent.Alpha(0.3)
	c.SetTheme(&t)
	return p
}
