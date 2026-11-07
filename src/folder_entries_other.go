//go:build !windows

package main

import (
	"io/fs"
	"strings"
)

// skipFolderEntry reports whether the folder panel leaves an entry out. Elsewhere
// the system file names are known by name alone, and files starting with a dot
// are hidden, as the platform has no file attributes to read.
func skipFolderEntry(name string, info fs.FileInfo) bool {
	switch {
	case strings.EqualFold(name, "desktop.ini"), strings.EqualFold(name, "thumbs.db"),
		name == ".DS_Store":
		return true
	}
	return strings.HasPrefix(name, ".")
}
