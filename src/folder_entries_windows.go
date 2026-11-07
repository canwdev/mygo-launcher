package main

import (
	"io/fs"
	"strings"
	"syscall"
)

// skipFolderEntry reports whether the folder panel leaves an entry out: the
// system's hidden and system files, as Explorer hides them, which a folder
// shows as desktop.ini and Thumbs.db. The names cover copies that lost their
// attributes.
func skipFolderEntry(name string, info fs.FileInfo) bool {
	switch {
	case strings.EqualFold(name, "desktop.ini"), strings.EqualFold(name, "thumbs.db"):
		return true
	}
	if info == nil {
		return false
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false
	}
	const hide = syscall.FILE_ATTRIBUTE_HIDDEN | syscall.FILE_ATTRIBUTE_SYSTEM
	return data.FileAttributes&hide != 0
}
