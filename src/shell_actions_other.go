//go:build !windows

package main

import "errors"

// canUseShellActions has no shell to act on here.
func canUseShellActions(string) bool { return false }

func showItemProperties(string) error {
	return errors.New("shell properties are not available")
}

func showShellMenu(string) error {
	return errors.New("the shell menu is not available")
}
