package main

import (
	"os"
	"path/filepath"
	"strconv"

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
	w.OnClosed(func() {
		v.app.settingsWin = nil
		v.shortcutRecording = false
	})
	v.updateAppIcons() // the settings window carries the gear badge
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

	ui.Scroll(c).Fill().Padding(16, 20, 24, 20).Gap(6).Children(func() {
		ui.Text(c, m.Settings()).FontSize(20).FontWeight(700).LetterSpacing(-0.3).Margin(0, 0, 6, 0)

		group := func(title string, rows func()) {
			ui.Text(c, title).FontSize(11.5).FontWeight(600).TextColor(p.muted).Margin(10, 2, 2, 2)
			ui.Column(c).Radius(10).Background(p.bg).Border(1, p.hairline).
				Dividers(1, p.hairline).Children(rows)
		}
		row := func(label, desc string, control func()) {
			ui.Row(c).Padding(8, 14).Gap(12).AlignItems(ui.Center).MinHeight(42).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, label).FontSize(13).FontWeight(500)
					if desc != "" {
						ui.Text(c, desc).FontSize(11.5).TextColor(p.muted).LineHeight(1.35)
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
							Label(a.label).Tooltip(a.label).FocusRing(false)
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
							v.scheduleAppIconUpdate()
						}
					}
					// The custom accent is the accent in use with an edit icon on
					// it: it opens the color picker, which starts from that color.
					custom := accentColor(v.prefs.Accent, p.dark)
					well := ui.ButtonBase(c).Size(20, 20).Radius(10).Background(custom).
						Label(m.CustomAccent()).Tooltip(m.CustomAccent()).FocusRing(false)
					if well.Hovered() {
						well.Background(custom.Mix(ui.RGB(255, 255, 255), 0.18))
					}
					if well.Clicked() {
						v.customAccentOpen = !v.customAccentOpen
					}
					well.Children(func() {
						ui.Icon(c, iconEdit).Size(11, 11).TextColor(accentText(custom))
					})
					ui.Popover(c, well, &v.customAccentOpen, func() {
						if ui.ColorPicker(c, &custom).Changed() {
							v.prefs.Accent = hexColor(custom)
							v.changedSettings()
							v.scheduleAppIconUpdate()
						}
					})
				})
			})
			row(m.IconSize(), m.IconSizeHint(), func() {
				sizes := []string{"small", "medium", "large"}
				i := 1
				for k, size := range sizes {
					if size == v.prefs.IconSize {
						i = k
					}
				}
				if segmented(c.Key("icon-size"), &i, m.Small(), m.Medium(), m.Large()) {
					v.prefs.IconSize = sizes[i]
					v.changedSettings()
				}
			})
		})

		group(m.Behavior(), func() {
			row(m.GlobalShortcut(), "", func() {
				v.shortcutControl(c)
			})
			row(m.AutoTiming(), m.AutoTimingHint(), func() {
				if ui.Switch(c, &s.AutoTime).Label(m.ToggleAutoTiming()).Changed() {
					if !s.AutoTime {
						v.runtimeOpen = false // the timer's dialog goes with it
					}
					v.app.save()
				}
			})
			row(m.AutoHide(), m.AutoHideHint(), func() {
				if ui.Switch(c, &s.AutoHide).Label(m.ToggleAutoHide()).Changed() {
					v.app.save()
				}
			})
			row(m.AlwaysOnTop(), "", func() {
				if ui.Switch(c, &s.AlwaysOnTop).Label(m.ToggleAlwaysOnTop()).Changed() {
					v.app.setAlwaysOnTop(s.AlwaysOnTop)
				}
			})
			row(m.WheelPages(), m.WheelPagesHint(), func() {
				if ui.Switch(c, &v.prefs.WheelPages).Label(m.ToggleWheelPages()).Changed() {
					v.setWheelPages(v.prefs.WheelPages)
				}
			})
			row(m.GpuDrawing(), m.GpuDrawingHint(), func() {
				if ui.Switch(c, &v.prefs.GPU).Label(m.ToggleGpuDrawing()).Changed() {
					v.changedSettings()
					v.app.notify(m.RestartToApply())
				}
			})
			row(m.BackgroundOnClose(), m.BackgroundOnCloseHint(), func() {
				if ui.Switch(c, &s.BackgroundOnClose).Label(m.ToggleBackgroundOnClose()).Changed() {
					v.setBackgroundOnClose(s.BackgroundOnClose)
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
			row(m.DataDirectory(), filepath.Join(dataBase(), dataDir), func() {
				if ui.Button(c, m.RevealInFolder()).Clicked() {
					v.app.openDataDir()
				}
			})
			row(m.Icons(), m.IconsHint(), func() {
				if ui.Button(c, m.UpdateIcons()).Clicked() {
					v.batchUpdateIcons()
				}
			})
			row(m.MigrateCLaunch(), v.migrateHint(m), func() {
				if ui.Button(c, m.ChooseCLaunchIni()).Clicked() {
					v.migrateCLaunch()
				}
			})
		})

		group(m.About(), func() {
			ui.Row(c).Padding(8, 14).Gap(12).AlignItems(ui.Center).MinHeight(42).Children(func() {
				if icon := v.aboutIconImage(); icon != nil {
					ui.Image(c, icon).Size(28, 28).Fit(ui.Contain)
				}
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, appName).FontSize(13).FontWeight(500)
					ui.Text(c, m.Version(appVersion)).FontSize(11.5).TextColor(p.muted).LineHeight(1.35)
				})
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

// migrateHint is the migration row's description: what the row does, or what the
// last import said.
func (v *view) migrateHint(m locales.Messages) string {
	if v.migrateStatus != "" {
		return v.migrateStatus
	}
	return m.MigrateCLaunchHint()
}

// migrateCLaunch asks for a CLaunch.ini and adds its pages as new tabs. The
// system dialog blocks and the icons are read from the disk, so both run on a
// goroutine; the store is changed on the main thread, where it lives.
func (v *view) migrateCLaunch() {
	if v.app.win == nil {
		return // headless: there is no window to open the dialog on
	}
	absolute := v.app.store.Settings.AbsolutePaths
	v.migrateStatus = v.msgs().MigratingCLaunch()
	v.app.setBusy(v.migrateStatus)
	// The settings window is where the button was clicked: put the dialog over
	// it, not over the launcher behind it.
	parent := v.app.settingsWin
	if parent == nil {
		parent = v.app.win
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  parent,
			Title:   locales.SelectCLaunchIniTitle(),
			Filters: []mygo.FileFilter{{Name: locales.CLaunchIniFilter(), Extensions: []string{"ini"}}},
		})
		if err != nil || len(paths) == 0 {
			v.app.dispatch(func() {
				v.app.setBusy("")
				v.migrateStatus = ""
				v.app.refresh()
			})
			return
		}
		imp, err := importCLaunchFile(paths[0], absolute, true)
		v.app.dispatch(func() {
			v.app.setBusy("")
			if err != nil {
				v.migrateStatus = v.msgs().CLaunchImportFailed(err.Error())
				v.app.notify(v.migrateStatus)
				v.app.refresh()
				return
			}
			v.applyCLaunchImport(imp)
		})
	}()
}

// applyCLaunchImport adds the tabs an import produced, shows the first of them
// and says what came of the import. It runs on the main thread.
func (v *view) applyCLaunchImport(imp cLaunchImport) {
	first, tabs, items := v.app.addCLaunchImport(imp)
	m := v.msgs()
	switch {
	case tabs == 0:
		v.migrateStatus = m.CLaunchImportEmpty()
	case imp.Skipped > 0:
		v.migrateStatus = m.CLaunchImported(strconv.Itoa(tabs), strconv.Itoa(items)) + " · " + m.CLaunchImportSkipped(imp.Skipped)
	default:
		v.migrateStatus = m.CLaunchImported(strconv.Itoa(tabs), strconv.Itoa(items))
	}
	v.app.notify(v.migrateStatus)
	if first != "" {
		v.prefs.ActiveTab = first
		v.savePrefs()
	}
	v.app.refresh()
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
		v.updateAppIcons()
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

// setWheelPages keeps the wheel preference and forgets the hover it was
// suppressing.
func (v *view) setWheelPages(on bool) {
	v.prefs.WheelPages = on
	v.hoverIgnoredTab = ""
	v.wheelSum = 0
	v.savePrefs()
}
