//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// uninstallKey is the app's Settings > Apps entry, which the installer
// writes with the identifier of mygo.json.
const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\com.canwdev.golauncher`

// installedDataBase returns the folder an installed copy keeps its data in,
// and whether this executable is that copy: the uninstall entry points at
// this folder, which a copy of the installed files elsewhere does not.
func installedDataBase() (string, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, uninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	loc, _, err := key.GetStringValue("InstallLocation")
	if err != nil || !strings.EqualFold(filepath.Clean(loc), filepath.Clean(installDir())) {
		return "", false
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, err = os.UserCacheDir()
		if err != nil {
			return "", false
		}
	}
	return filepath.Join(base, appName), true
}
