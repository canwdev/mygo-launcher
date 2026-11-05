package main

import (
	"bytes"
	"image"
	"image/png"
	"sync"

	"github.com/egoist/mygo"
	"golang.org/x/image/draw"

	"go-launcher/locales"
	"go-launcher/resources"
)

// trayIconPx is the size of the tray icon: enough for the largest the
// notification area draws it at, on a high-DPI screen.
const trayIconPx = 64

var (
	trayIconOnce sync.Once
	trayIconData []byte
)

// trayIconPNG is the app's icon at the size the tray shows it.
func trayIconPNG() []byte {
	trayIconOnce.Do(func() {
		trayIconData = resources.IconPNG
		src, err := png.Decode(bytes.NewReader(resources.IconPNG))
		if err != nil {
			return
		}
		dst := image.NewNRGBA(image.Rect(0, 0, trayIconPx, trayIconPx))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
		var b bytes.Buffer
		if err := png.Encode(&b, dst); err != nil {
			return
		}
		trayIconData = b.Bytes()
	})
	return trayIconData
}

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
	if v.tray == nil {
		tray, err := mygo.NewTray(mygo.TrayOptions{
			ID:      "main",
			Icon:    trayIconPNG(),
			ToolTip: mygo.App.Name(),
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
		{Label: m.OpenMain(), Click: func(*mygo.MenuItem, *mygo.Window) { v.showMain() }},
		{Label: m.Settings(), Click: func(*mygo.MenuItem, *mygo.Window) { v.openSettings() }},
		{Type: mygo.MenuItemSeparator},
		{Label: m.Exit(), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
}

// showMain brings the main window back from the tray.
func (v *view) showMain() {
	if v.app.win != nil {
		v.app.win.Show()
	}
}

// handleClose hides the main window in the tray while the app is set to keep
// running in the background. A quit closes it as usual.
func (v *view) handleClose(e *mygo.CloseEvent) {
	if !v.app.keepInBackground() {
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
		v.app.win.Show()
	}
}
