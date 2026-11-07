// Package resources embeds the app's own files for use at run time.
package resources

import _ "embed"

// IconBasePNG is the icon's white rounded background and border, without the
// shapes that take the accent color.
//
//go:embed icon-base.png
var IconBasePNG []byte

// IconAccentPNG is a white mask of the icon's accent shapes, the translucent
// arrow included, which the app tints with the accent color.
//
//go:embed icon-accent.png
var IconAccentPNG []byte

// IconGearPNG is the gear badge the settings window's icon carries.
//
//go:embed icon-gear.png
var IconGearPNG []byte
