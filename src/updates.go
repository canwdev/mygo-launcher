package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"

	"go-launcher/locales"
)

// The updater plugin's functions, which tests swap: the plugin panics before
// useUpdater has run.
var (
	checkForUpdates    = updater.CheckForUpdates
	updateChecks       = updater.AutomaticChecks
	setUpdateChecks    = updater.SetAutomaticChecks
	updateDownloads    = updater.AutomaticDownloads
	setUpdateDownloads = updater.SetAutomaticDownloads
	lastUpdateCheck    = updater.LastCheck
	availableUpdate    = updater.AvailableVersion
)

// useUpdater checks GitHub's releases for a new version soon after launch and
// then once a day, and adds the native update window. It must run before
// App.Run.
func (a *App) useUpdater() {
	mygo.Use(native.New(updater.Options{}))
	a.updates = true
	// A check, or an answer in the update window, changes what Settings shows.
	updater.OnChange(func() { a.refresh() })
}

// updateStatus says what the About row shows under Updates: the version found,
// when the last check was, or that none ran.
func updateStatus(m locales.Messages) string {
	if version := availableUpdate(); version != "" {
		return m.UpdateAvailable(version)
	}
	if at := lastUpdateCheck(); !at.IsZero() {
		return m.LastChecked(at.Local().Format("2006-01-02 15:04"))
	}
	return m.NotCheckedYet()
}
