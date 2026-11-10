//go:build !windows

package main

import "github.com/egoist/mygo"

// pickIconFromShell is Windows only: other platforms have no shell icon picker.
func pickIconFromShell(parent *mygo.Window, path string, index int) (string, int, bool) {
	return "", 0, false
}
