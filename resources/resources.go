// Package resources embeds the app's own files for use at run time.
package resources

import _ "embed"

// IconPNG is icon.png in this directory, the app's icon of mygo.json.
//
//go:embed icon.png
var IconPNG []byte
