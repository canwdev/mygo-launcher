package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// openSettings shows the settings window, opening it the first time. In tests
// there is no window to open: settingsView is driven directly instead.
func (v *view) openSettings() {
	if v.app.win == nil {
		return
	}
	if w := v.app.settingsWin; w != nil {
		w.Show()
		w.Focus()
		w.Invalidate()
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:           "Settings",
		Width:           560,
		Height:          720,
		MinWidth:        480,
		MinHeight:       420,
		StateKey:        "settings",
		DisableMaximize: true,
		DisableMinimize: true,
		AutoHideMenuBar: true,
		BackgroundColor: "light-dark(#f4f4f6, #1c1c1f)",
		Content:         ui.View(v.settingsView),
	})
	v.app.settingsWin = w
	w.OnClosed(func() { v.app.settingsWin = nil })
}

// settingsView is the content of the settings window: the choices the launcher
// keeps in its store and in ui-state.json, grouped in cards as MyGo Music
// groups its own.
func (v *view) settingsView(c *ui.Context) {
	p := applyPalette(c, v.prefs.Accent)
	v.pal = p
	c.Root().Background(p.sidebar)
	if w := v.app.settingsWin; w != nil {
		c.OnShortcut(ui.Cmd, ui.KeyW, func() { w.Close() })
	}
	s := &v.app.store.Settings

	ui.Scroll(c).Fill().Padding(24, 28, 32, 28).Gap(8).Children(func() {
		ui.Text(c, "Settings").FontSize(22).FontWeight(700).LetterSpacing(-0.3).Margin(0, 0, 10, 0)

		group := func(title string, rows func()) {
			ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(p.muted).Margin(14, 4, 4, 4)
			ui.Column(c).Radius(12).Background(p.bg).Border(1, p.hairline).
				Dividers(1, p.hairline).Children(rows)
		}
		row := func(label, desc string, control func()) {
			ui.Row(c).Padding(12, 16).Gap(16).AlignItems(ui.Center).MinHeight(48).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, label).FontSize(13.5).FontWeight(500)
					if desc != "" {
						ui.Text(c, desc).FontSize(12).TextColor(p.muted).LineHeight(1.4)
					}
				})
				control()
			})
		}

		group("Appearance", func() {
			row("Theme", "Auto follows the system's light or dark appearance", func() {
				modes := []string{"auto", "light", "dark"}
				i := 0
				for k, mode := range modes {
					if mode == v.prefs.Theme {
						i = k
					}
				}
				if segmented(c.Key("theme"), &i, "System", "Light", "Dark") {
					v.changeTheme(modes[i])
				}
			})
			row("Accent color", "Buttons, selections and highlights follow it", func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					for _, a := range accentPresets {
						color := a.light
						if p.dark {
							color = a.dark
						}
						sw := ui.ButtonBase(c).Size(20, 20).Radius(10).Background(color).
							Label(a.label).FocusRing(false)
						chosen := v.prefs.Accent == a.id || v.prefs.Accent == "" && a.id == defaultAccent
						if chosen {
							sw.Border(2, p.text)
						}
						if sw.Hovered() && !chosen {
							sw.Border(2, p.faint)
						}
						if sw.Clicked() {
							v.prefs.Accent = a.id
							v.changedSettings()
						}
					}
					custom := accentColor(v.prefs.Accent, p.dark)
					well := ui.ColorWell(c, &custom).Label("Custom accent color")
					if well.Changed() {
						v.prefs.Accent = hexColor(custom)
						v.changedSettings()
					}
				})
			})
		})

		group("Behavior", func() {
			row("Auto timing", "Add a program's running time to its total while it runs", func() {
				if ui.Switch(c, &s.AutoTime).Label("Toggle auto timing").Changed() {
					v.app.save()
				}
			})
			row("Auto-hide the launcher", "Minimize the window after launching an item", func() {
				if ui.Switch(c, &s.AutoHide).Label("Toggle auto-hide").Changed() {
					v.app.save()
				}
			})
			row("Always on top", "Keep the window above the other windows", func() {
				if ui.Switch(c, &s.AlwaysOnTop).Label("Toggle always on top").Changed() {
					v.app.setAlwaysOnTop(s.AlwaysOnTop)
				}
			})
		})

		group("Paths", func() {
			row("Absolute paths for new items", "Store new items with full paths instead of paths relative to the launcher", func() {
				if ui.Switch(c, &s.AbsolutePaths).Label("Toggle absolute paths").Changed() {
					v.app.save()
				}
			})
			row("Convert all paths", "Rewrite the paths of every item at once", func() {
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c, "To absolute").Clicked() {
						v.convertPaths(true)
					}
					if ui.Button(c, "To relative").Clicked() {
						v.convertPaths(false)
					}
				})
			})
		})

		group("Data", func() {
			row("Refresh", "Apply an edit made to the data file by hand", func() {
				if ui.Button(c, "Refresh").Clicked() {
					v.refreshData()
				}
			})
			row("Data directory", "The store, cached icons and window state live here", func() {
				if ui.Button(c, "Open folder").Clicked() {
					v.app.openDataDir()
				}
			})
			row("Icons", "Extract every auto-generated icon from its file again", func() {
				if ui.Button(c, "Update icons").Clicked() {
					v.batchUpdateIcons()
				}
			})
		})

		group("About", func() {
			row("MyGo Launcher", "Version "+appVersion, func() {
				if ui.Button(c, "GitHub").Clicked() {
					mygo.Shell.OpenExternal("https://github.com/canwdev/go-launcher")
				}
			})
		})
	})
}

// changeTheme makes the appearance the preference asks for. The theme source
// is set outside the frame: SetSource posts to the main thread and must not run
// while one is built.
func (v *view) changeTheme(mode string) {
	v.prefs.Theme = mode
	savePrefs(v.prefs)
	if v.app.win == nil {
		return // headless: there is no theme source to change
	}
	v.app.dispatch(func() {
		v.applyTheme()
		v.app.refresh()
	})
}

// changedSettings keeps the view's own settings and redraws both windows.
func (v *view) changedSettings() {
	savePrefs(v.prefs)
	v.app.refresh()
}
