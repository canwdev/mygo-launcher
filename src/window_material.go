package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// windowMaterial is the translucent material the window is created with, from
// the preference's id. Windows 11 draws Mica and Acrylic; where the system has
// no backdrop, as on Windows 10, MyGo shows none and the view keeps painting
// its own background.
func windowMaterial(id string) mygo.Vibrancy {
	switch id {
	case "mica":
		return mygo.VibrancyMica
	case "acrylic":
		return mygo.VibrancyAcrylic
	}
	return mygo.VibrancyNone
}

// materialTintAlpha is how much of the palette's background the content tints
// over the material: the texts and icons need a known tone under them, light
// over a light appearance and dark over a dark one, while the material still
// shows through the rest. Raise it for a calmer, more opaque content; lower it
// to let more of the wallpaper through.
const materialTintAlpha = 0.4

// windowFill is the color the content lays over the material: the palette's
// background, part see-through so the material shows through, at
// materialTintAlpha. The top bar above it paints nothing instead, and shows
// the material as the system draws it.
func (v *view) windowFill(color ui.Color) ui.Color {
	if !v.material {
		return color
	}
	return color.Alpha(materialTintAlpha)
}

// surface is the color a pane paints. With a material behind the window the
// panes paint nothing and the material shows as it is, except the content,
// which tints it (see windowFill); without a material they paint the color
// itself.
func (v *view) surface(color ui.Color) ui.Color {
	if !v.material {
		return color
	}
	return ui.Transparent
}
