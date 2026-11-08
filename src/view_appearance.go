package main

// chooseAppIcon asks for a program icon, takes it into the custom folder and
// shows it everywhere the app's icon appears. The system dialog blocks, so it
// runs on a goroutine; the result comes back on the main thread.
func (v *view) chooseAppIcon() {
	go func() {
		path := v.app.pickAppIcon("")
		if path == "" {
			return
		}
		icon, err := v.app.customIcon(path, 0)
		v.app.dispatch(func() {
			if err != nil {
				v.app.notify(err.Error())
				return
			}
			v.setAppIcon(icon)
		})
	}()
}

func (v *view) setAppIcon(icon string) {
	v.prefs.AppIcon = icon
	v.savePrefs()
	v.updateAppIcons()
	v.app.pruneIcons()
	v.app.refresh()
}

// clearAppIcon goes back to the icon drawn from the accent color, and drops
// the file no longer used.
func (v *view) clearAppIcon() {
	if v.prefs.AppIcon == "" {
		return
	}
	v.prefs.AppIcon = ""
	v.savePrefs()
	v.updateAppIcons()
	v.app.pruneIcons()
	v.app.refresh()
}

// chooseBackground asks for a picture, keeps a normalized copy in the custom
// folder, and shows it behind the main window.
func (v *view) chooseBackground() {
	go func() {
		path := v.app.pickBackground("")
		if path == "" {
			return
		}
		stored, err := v.app.customBackground(path)
		v.app.dispatch(func() {
			if err != nil {
				v.app.notify(err.Error())
				return
			}
			v.setBackground(stored)
		})
	}()
}

func (v *view) setBackground(stored string) {
	v.prefs.Background = stored
	v.bgImageKey, v.bgImage = "", nil
	v.savePrefs()
	v.app.pruneIcons()
	v.app.refresh()
}

func (v *view) clearBackground() {
	if v.prefs.Background == "" {
		return
	}
	v.prefs.Background = ""
	v.bgImageKey, v.bgImage = "", nil
	v.savePrefs()
	v.app.pruneIcons()
	v.app.refresh()
}
