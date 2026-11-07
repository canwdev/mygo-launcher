package main

import (
	"log"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"go-launcher/locales"
)

// Window geometry of the first launch, before any state was kept: wide enough
// for the item dialog's fields to fit without scrolling.
const (
	// appName is the app's name, shown in its title bars and menus.
	appName = "MyGo Launcher"

	defaultWindowWidth  = 720
	defaultWindowHeight = 520
)

func main() {
	// Where the store, the icon cache and the window's preferences live: in
	// the user's folder for an installed copy, beside the executable for a
	// portable one. Everything below reads the data.
	prepareDataBase()

	// The renderer is chosen when the window opens: draw in memory unless the
	// GPU drawing preference, off by default, asks for the GPU. MYGO_GPU still
	// overrides the preference for development.
	if os.Getenv("MYGO_GPU") == "" {
		if loadPrefs().GPU {
			_ = os.Setenv("MYGO_GPU", "1")
		} else {
			_ = os.Setenv("MYGO_GPU", "0")
		}
	}

	// Single-instance guard, scoped per install directory: a second copy from
	// the same directory is refused (and focuses the existing window), while
	// copies in different install directories may run simultaneously.
	key := installKey()
	if ok, err := acquireSingleton(key); err != nil {
		log.Fatalf("single-instance check failed: %v", err)
	} else if !ok {
		activateExistingInstance()
		return
	}
	defer releaseSingleton()

	app := NewApp()
	app.useUpdater()

	mygo.App.SetName(appName)
	mygo.App.SetVersion(appVersion)
	// The window's geometry is kept beside the store, in mygo-launcher-data,
	// rather than in MyGo's own file in the user's config folder.
	mygo.App.SetWindowStateStore(windowStateStore{})

	mygo.App.WhenReady(func() {
		v := newView(app)
		v.applyTheme()
		v.applyLanguage()
		mygo.App.OnLocaleChanged(func(locale string) {
			if app.settingsWin != nil {
				app.settingsWin.SetTitle(locales.In(locale).Settings())
			}
			v.syncTray() // the tray's menu says its items in the new language
			app.refresh()
		})
		// A quit, from the tray or the system, closes the window instead of
		// hiding it in the tray; the tray goes with the app.
		// The system's appearance picks the lighter or darker accent color,
		// so the icons follow it while the theme follows the system.
		mygo.Theme.OnUpdated(func() {
			if v.prefs.Theme == "auto" && !v.app.quitting {
				v.app.dispatch(v.updateAppIcons)
			}
		})
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { app.quitting = true })
		mygo.App.OnQuit(v.destroyTray)
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     appName,
			Width:     defaultWindowWidth,
			Height:    defaultWindowHeight,
			MinWidth:  420,
			MinHeight: 300,
			// A remembered size, position and maximized state are applied on
			// top of these, from window-state.json.
			StateKey: windowStateKey,
			// The store's always-on-top setting applies from the first frame.
			AlwaysOnTop: app.store.Settings.AlwaysOnTop,
			Content:     ui.View(v.view),
		})

		// The view builds after NewWindow returned, on the main loop's next
		// turn, so both hold the window by then.
		app.win, v.win = win, win
		v.updateAppIcons()

		win.OnClose(func(e *mygo.CloseEvent) { v.handleClose(e) })
		win.OnFileDrop(func(e *mygo.FileDropEvent) {
			app.addPaths(e.Paths, v.prefs.ActiveTab)
		})
		v.syncTray()
		v.registerSavedShortcut()

		app.startClock()
		app.startSpecialPoll()
		app.startStoreWatcher()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}

	app.stopStoreWatcher()
}
