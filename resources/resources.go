// Package resources embeds the app's own files for use at run time.
package resources

import _ "embed"

// IconBasePNG is the icon's white rounded background and border for the light
// appearance, without the shapes that take the accent color.
//
//go:embed icon-base.png
var IconBasePNG []byte

// IconBaseDarkPNG is the icon's dark rounded background and border, drawn
// instead of IconBasePNG in the dark appearance.
//
//go:embed icon-base-dark.png
var IconBaseDarkPNG []byte

// IconAccentPNG is a white mask of the icon's accent shapes, the translucent
// arrow included, which the app tints with the accent color.
//
//go:embed icon-accent.png
var IconAccentPNG []byte

// IconGearPNG is the settings window's gear badge, a white mask on a
// transparent background that the app tints for the appearance.
//
//go:embed icon-gear.png
var IconGearPNG []byte
