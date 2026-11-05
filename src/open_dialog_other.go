//go:build !windows

package main

import "github.com/egoist/mygo"

// openFilesDialog asks for files with MyGo's dialog: shortcuts are a Windows
// concern.
func openFilesDialog(parent *mygo.Window, title string) ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: title, Multiple: true})
}
