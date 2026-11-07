//go:build !windows

package main

// systemLocations has nothing to offer where the shell has no such folders.
var systemLocations []systemLocation
