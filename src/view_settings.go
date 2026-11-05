package main

import (
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"go-launcher/locales"
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
		Title:           v.msgs().Settings(),
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
	v.lang = c.Locale()
	p := applyPalette(c, v.prefs.Accent)
	v.pal = p
	m := v.msgs()
	c.Root().Background(p.sidebar)
	if w := v.app.settingsWin; w != nil {
		c.OnShortcut(ui.Cmd, ui.KeyW, func() { w.Close() })
	}
	s := &v.app.store.Settings

	ui.Scroll(c).Fill().Padding(24, 28, 32, 28).Gap(8).Children(func() {
		ui.Text(c, m.Settings()).FontSize(22).FontWeight(700).LetterSpacing(-0.3).Margin(0, 0, 10, 0)

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

		group(m.Appearance(), func() {
			row(m.Language(), "", func() {
				chinese, english := m.SimplifiedChinese(), m.English()
				// The stored choice is what the dropdown shows; with none,
				// it shows the language the system chose.
				choice := english
				if v.prefs.Language == "zh-Hans" || v.prefs.Language == "auto" && v.lang == "zh-Hans" {
					choice = chinese
				}
				if ui.Select(c, &choice, []string{chinese, english}).Label(m.Language()).Changed() {
					lang := "en"
					if choice == chinese {
						lang = "zh-Hans"
					}
					v.changeLanguage(lang)
				}
			})
			row(m.Theme(), "", func() {
				modes := []string{"auto", "light", "dark"}
				i := 0
				for k, mode := range modes {
					if mode == v.prefs.Theme {
						i = k
					}
				}
				if segmented(c.Key("theme"), &i, m.System(), m.Light(), m.Dark()) {
					v.changeTheme(modes[i])
				}
			})
			row(m.AccentColor(), "MyGo!!!!!", func() {
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
					well := ui.ColorWell(c, &custom).Label(m.CustomAccent())
					if well.Changed() {
						v.prefs.Accent = hexColor(custom)
						v.changedSettings()
					}
				})
			})
			row(m.BackgroundOnClose(), m.BackgroundOnCloseHint(), func() {
				if ui.Switch(c, &s.BackgroundOnClose).Label(m.ToggleBackgroundOnClose()).Changed() {
					v.setBackgroundOnClose(s.BackgroundOnClose)
				}
			})
		})

		group(m.Behavior(), func() {
			row(m.AutoTiming(), m.AutoTimingHint(), func() {
				if ui.Switch(c, &s.AutoTime).Label(m.ToggleAutoTiming()).Changed() {
					v.app.save()
				}
			})
			row(m.AutoHide(), m.AutoHideHint(), func() {
				if ui.Switch(c, &s.AutoHide).Label(m.ToggleAutoHide()).Changed() {
					v.app.save()
				}
			})
			row(m.AlwaysOnTop(), m.AlwaysOnTopHint(), func() {
				if ui.Switch(c, &s.AlwaysOnTop).Label(m.ToggleAlwaysOnTop()).Changed() {
					v.app.setAlwaysOnTop(s.AlwaysOnTop)
				}
			})
		})

		group(m.Paths(), func() {
			row(m.AbsolutePaths(), m.AbsolutePathsHint(), func() {
				if ui.Switch(c, &s.AbsolutePaths).Label(m.ToggleAbsolutePaths()).Changed() {
					v.app.save()
				}
			})
			row(m.ConvertAllPaths(), m.ConvertAllPathsHint(), func() {
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c, m.ToAbsolute()).Clicked() {
						v.convertPaths(true)
					}
					if ui.Button(c, m.ToRelative()).Clicked() {
						v.convertPaths(false)
					}
				})
			})
		})

		group(m.Data(), func() {
			row(m.Refresh(), m.RefreshHint(), func() {
				if ui.Button(c, m.Refresh()).Clicked() {
					v.refreshData()
				}
			})
			row(m.DataDirectory(), m.DataDirectoryHint(), func() {
				if ui.Button(c, m.OpenFolder()).Clicked() {
					v.app.openDataDir()
				}
			})
			row(m.Icons(), m.IconsHint(), func() {
				if ui.Button(c, m.UpdateIcons()).Clicked() {
					v.batchUpdateIcons()
				}
			})
		})

		group(m.About(), func() {
			row("MyGo Launcher", m.Version(appVersion), func() {
				if ui.Button(c, m.Github()).Clicked() {
					mygo.Shell.OpenExternal("https://github.com/canwdev/mygo-launcher")
				}
			})
			if v.app.updates {
				row(m.AutoCheckUpdates(), m.AutoCheckUpdatesHint(), func() {
					on := updateChecks()
					if ui.Switch(c, &on).Label(m.ToggleAutoCheckUpdates()).Changed() {
						setUpdateChecks(on)
					}
				})
				row(m.AutoDownloadUpdates(), m.AutoDownloadUpdatesHint(), func() {
					on := updateDownloads()
					sw := ui.Switch(c, &on).Label(m.ToggleAutoDownloadUpdates()).Disabled(!updateChecks())
					if sw.Changed() {
						setUpdateDownloads(on)
					}
				})
				row(m.Updates(), updateStatus(m), func() {
					if ui.Button(c, m.CheckForUpdates()).Clicked() {
						checkForUpdates()
					}
				})
			}
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

// changeLanguage makes the app show the language the preference asks for. The
// language is set outside the frame, as the theme is.
func (v *view) changeLanguage(lang string) {
	v.prefs.Language = lang
	savePrefs(v.prefs)
	if v.app.win == nil {
		return // headless: there is no app locale to change
	}
	v.app.dispatch(func() {
		mygo.App.SetLocale(localeTag(lang))
		if w := v.app.settingsWin; w != nil {
			w.SetTitle(locales.In(mygo.App.Locale()).Settings())
		}
		v.app.refresh()
	})
}

// applyLanguage makes the app show the language the preference asks for at
// startup: "auto" is "" to follow the user's languages. MYGO_LOCALE (mygo dev
// -lang) keeps its choice for the run.
func (v *view) applyLanguage() {
	if os.Getenv("MYGO_LOCALE") != "" {
		return
	}
	mygo.App.SetLocale(localeTag(v.prefs.Language))
}

// localeTag is the tag of a language preference: "auto" follows the user's
// languages, which SetLocale says with "".
func localeTag(pref string) string {
	switch pref {
	case "en", "zh-Hans":
		return pref
	}
	return ""
}

// changedSettings keeps the view's own settings and redraws both windows.
func (v *view) changedSettings() {
	savePrefs(v.prefs)
	v.app.refresh()
}
