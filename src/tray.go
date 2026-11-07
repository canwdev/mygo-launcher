package main

import (
	"github.com/egoist/mygo"

	"go-launcher/locales"
)

// syncTray shows the tray icon while the app keeps running in the background,
// and takes it away otherwise. Its menu follows the app's language.
func (v *view) syncTray() {
	if v.app.win == nil {
		return // tests: there is no app to add a tray icon to
	}
	if !v.app.store.Settings.BackgroundOnClose {
		v.destroyTray()
		return
	}
	menu := v.trayMenu()
	// The window title no longer carries the install directory, so the tray's
	// hover text is where it stays.
	tip := instanceTitle(installDir())
	if v.tray == nil {
		tray, err := mygo.NewTray(mygo.TrayOptions{
			ID:      "main",
			Icon:    v.trayIcon(),
			ToolTip: tip,
			Menu:    menu,
		})
		if err != nil {
			v.app.notifyErr(err)
			return
		}
		v.tray = tray
		// A click brings the window back; Windows opens the menu on a right
		// click, as the app asks.
		tray.OnClick(v.showMain)
		return
	}
	v.tray.SetMenu(menu)
	v.tray.SetToolTip(tip)
}

// destroyTray removes the icon, if there is one.
func (v *view) destroyTray() {
	if v.tray != nil {
		v.tray.Destroy()
		v.tray = nil
	}
}

// trayMenu is the menu of the tray icon: the main window, the settings window,
// and quitting.
func (v *view) trayMenu() *mygo.Menu {
	m := locales.In(mygo.App.Locale())
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: m.OpenMain(), Accelerator: v.prefs.Shortcut, Click: func(*mygo.MenuItem, *mygo.Window) { v.showMain() }},
		{Label: m.Settings(), Click: func(*mygo.MenuItem, *mygo.Window) { v.openSettings() }},
		{Type: mygo.MenuItemSeparator},
		{Label: m.Exit(), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
}

// showMain brings the main window back from the tray. A minimized window is
// restored first: Show alone would leave it minimized.
func (v *view) showMain() {
	if v.app.win == nil {
		return
	}
	v.app.win.Restore()
	v.app.win.Show()
}

// handleClose hides the main window in the tray while the app is set to keep
// running in the background. Otherwise closing it quits: the settings window
// and any tray icon left over from an earlier setting go with it, so nothing
// keeps the app alive.
func (v *view) handleClose(e *mygo.CloseEvent) {
	if !v.app.keepInBackground() {
		v.destroyTray()
		if v.app.settingsWin != nil {
			v.app.settingsWin.Close()
		}
		return
	}
	e.PreventDefault()
	v.app.hideWindow()
}

// keepInBackground reports whether closing the main window should hide it in
// the tray rather than quit the app.
func (a *App) keepInBackground() bool {
	return a.store.Settings.BackgroundOnClose && !a.quitting
}

// hideWindow hides the main window, if there is one.
func (a *App) hideWindow() {
	if a.win != nil {
		a.win.Hide()
		releaseMemory()
	}
}

// setBackgroundOnClose applies the setting: the tray icon follows it, and
// turning it off brings a hidden window back, since nothing would bring it
// back once the tray is gone.
func (v *view) setBackgroundOnClose(on bool) {
	v.app.store.Settings.BackgroundOnClose = on
	v.app.save()
	v.syncTray()
	if !on && v.app.win != nil && !v.app.win.IsVisible() {
		v.showMain()
	}
}
