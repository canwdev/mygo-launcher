package main

import "go-launcher/locales"

// systemLocation is a shell namespace folder the add picker offers: an object
// with no path in the file system, as the Recycle Bin. Its path is a parsing
// name the shell opens.
type systemLocation struct {
	Path string
	Name func(locales.Messages) string
}
