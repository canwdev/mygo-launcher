package main

import (
	"github.com/egoist/mygo"
	"go-launcher/locales"
)

// The MyGo calls starting at login lives on, replaced in tests: the real ones
// read and write the user's Run entry.
var (
	setOpenAtLogin = func(on bool) error { return mygo.App.SetOpenAtLogin(on) }
	openAtLogin    = func() bool { return mygo.App.OpenAtLogin() }
)

// refreshOpenAtLogin reads whether the system will start the launcher at login.
// Task Manager can turn the entry off without the launcher knowing.
func (v *view) refreshOpenAtLogin() {
	v.openAtLogin = openAtLogin()
}

// changeOpenAtLogin adds or removes the launcher's Run entry. A failed change
// leaves the switch as it was.
func (v *view) changeOpenAtLogin(on bool) {
	if err := setOpenAtLogin(on); err != nil {
		v.app.notify(locales.OpenAtLoginFailed())
		return
	}
	v.openAtLogin = on
	v.app.refresh()
}
