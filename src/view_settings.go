package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"go-launcher/locales"
)

// relaunchApp restarts the app with the preferences it now has. A setting that
// is read once, as the drawing backend or the window's material, needs the new
// process; tests replace it, as they do the system dialogs.
var relaunchApp = func() { mygo.App.Relaunch() }

// openSettings shows the settings window, opening it the first time. In tests
// there is no window to open: settingsView is driven directly instead.
func (v *view) openSettings() {
	if v.app.win == nil {
		return
	}
	v.refreshOpenAtLogin()
	if w := v.app.settingsWin; w != nil {
		w.Show()
		w.Focus()
		w.Invalidate()
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:           v.msgs().Settings() + " - " + v.appTitle(),
		Width:           780,
		Height:          640,
		MinWidth:        640,
		MinHeight:       420,
		StateKey:        "settings-sections",
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

// settingsRow adds a row to a settings card: what it is, what it means, and
// the control that changes it.
type settingsRow func(label, desc string, control func())

// settingsGroup draws a card of rows, under a secondary title when it has
// one.
type settingsGroup func(title string, rows func())

// settingsSection is a page of the settings window: what its sidebar entry
// says and the icon it shows.
type settingsSection struct {
	id, title string
	icon      *ui.SVG
}

// settingsSidebarWidth is the width of the list of the settings' pages.
const settingsSidebarWidth = 180

// settingsView is the content of the settings window: a sidebar of pages and
// the page shown, whose choices the launcher keeps in its store and in
// ui-state.json, grouped in cards as MyGo Music groups its own.
func (v *view) settingsView(c *ui.Context) {
	v.lang = c.Locale()
	// The settings window has no material of its own.
	p := applyPalette(c, v.prefs.Accent)
	v.pal = p
	m := v.msgs()
	c.Root().Background(p.bg)
	if w := v.app.settingsWin; w != nil {
		c.OnShortcut(ui.Cmd, ui.KeyW, func() { w.Close() })
	}

	row := settingsRow(func(label, desc string, control func()) {
		ui.Row(c).Padding(8, 14).Gap(12).AlignItems(ui.Center).MinHeight(42).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, label).FontSize(13).FontWeight(500)
				if desc != "" {
					ui.Text(c, desc).FontSize(11.5).TextColor(p.muted).LineHeight(1.35)
				}
			})
			control()
		})
	})
	group := settingsGroup(func(title string, rows func()) {
		if title != "" {
			ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(p.muted).Margin(14, 4, 4, 4)
		}
		ui.Column(c).Radius(10).Background(p.bg).Border(1, p.hairline).
			Dividers(1, p.hairline).Children(rows)
	})

	sections := []settingsSection{
		{"appearance", m.Appearance(), iconPalette},
		{"behavior", m.Behavior(), iconSliders},
		{"data", m.Data(), iconBox},
		{"about", m.About(), iconProperties},
	}
	current := sections[0]
	for _, sec := range sections {
		if sec.id == v.settingsPage {
			current = sec
		}
	}

	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(settingsSidebarWidth).Shrink(0).Padding(12, 12, 14, 12).Gap(2).
			Background(p.sidebar).Children(func() {
			for _, sec := range sections {
				v.settingsNavItem(c, p, sec, sec.id == current.id)
			}
		})
		ui.Box(c).Width(1).Shrink(0).Background(p.hairline)
		ui.Scroll(c.Key("settings-page-"+current.id)).Fill().Padding(20, 28, 28, 28).Gap(6).Children(func() {
			ui.Text(c, current.title).FontSize(20).FontWeight(700).LetterSpacing(-0.3).Margin(0, 0, 6, 0)
			switch current.id {
			case "appearance":
				v.settingsAppearance(c, m, group, row)
			case "behavior":
				v.settingsBehavior(c, m, group, row)
			case "data":
				v.settingsData(c, m, group, row)
			case "about":
				v.settingsAbout(c, m, group, row)
			}
		})
	})
	v.titleDialog(c)
}

// settingsNavItem is a page in the settings sidebar; clicking it shows that
// page.
func (v *view) settingsNavItem(c *ui.Context, p palette, sec settingsSection, selected bool) {
	b := ui.ButtonBase(c.Key("settings-nav-" + sec.id)).Height(32).PaddingX(9).Gap(9).Radius(8).
		AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(sec.title)
	switch {
	case selected:
		b.Background(p.press)
	case b.Hovered():
		b.Background(p.hover)
	}
	color := p.muted
	if selected {
		color = p.accent
	}
	b.Children(func() {
		ui.Icon(c, sec.icon).Size(15, 15).TextColor(color)
		ui.Text(c, sec.title).FontSize(13).FontWeight(500).SingleLine().Shrink(1).MinWidth(0)
	})
	if b.Clicked() {
		v.settingsPage = sec.id
		v.app.refresh()
	}
}

// settingsAppearance is the page of the language, the theme and the accent, the
// window's material, opacity and background, and then the launcher's own marks:
// the grid's icon size, its icon and its window title.
func (v *view) settingsAppearance(c *ui.Context, m locales.Messages, group settingsGroup, row settingsRow) {
	p := v.pal
	// The page's own title already says appearance; only the card below it
	// carries one.
	group("", func() {
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
		row(m.WindowMaterial(), m.WindowMaterialHint(), func() {
			materials := []string{"", "mica", "acrylic"}
			i := 0
			for k, material := range materials {
				if material == v.prefs.WindowMaterial {
					i = k
				}
			}
			if segmented(c.Key("window-material"), &i, m.MaterialNone(), m.MaterialMica(), m.MaterialAcrylic()) {
				v.prefs.WindowMaterial = materials[i]
				v.savePrefs()
				// The window is created with its material, so the new one
				// needs a fresh window: the app restarts itself.
				relaunchApp()
			}
		})
		// The window's own opacity comes first: it fades everything, the
		// picture below included, and needs no picture to say something.
		row(m.WindowOpacity(), "", func() {
			value := windowTransparency(v.prefs.WindowOpacity)
			slider := ui.Slider(c.Key("window-opacity"), &value, 0, 1-minWindowOpacity).
				Width(150).Label(m.WindowOpacity())
			if slider.Changed() {
				v.setWindowOpacity(windowOpacityFrom(value))
			}
		})
		row(m.Background(), "", func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, m.ChooseBackground()).Clicked() {
					v.chooseBackground()
				}
				if v.prefs.Background != "" {
					if ui.Button(c, m.Clear()).Clicked() {
						v.clearBackground()
					}
				}
			})
		})
		// The picture's opacity only says something once there is a picture.
		if v.prefs.Background != "" {
			row(m.BackgroundOpacity(), "", func() {
				value := v.prefs.BackgroundOpacity
				slider := ui.Slider(c.Key("background-opacity"), &value, 0.05, 0.9).
					Width(150).Label(m.BackgroundOpacity())
				if slider.Changed() {
					v.setBackgroundOpacity(value)
				}
			})
		}
	})
	// The launcher's own marks: the grid's icons, the icon it shows in the
	// system, and the title of its window.
	group(m.Custom(), func() {
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
		row(m.ProgramIcon(), "", func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if icon := v.aboutIconImage(); icon != nil {
					ui.Image(c, icon).Size(22, 22).Fit(ui.Contain)
				}
				if iconButton(c, iconGrid, 16, p.muted, m.SystemIcon()).
					Tooltip(m.SystemIcon()).Clicked() {
					v.chooseAppIconFromShell()
				}
				if ui.Button(c, m.ChooseIcon()).Clicked() {
					v.chooseAppIcon()
				}
				if v.prefs.AppIcon != "" {
					if ui.Button(c, m.Clear()).Clicked() {
						v.clearAppIcon()
					}
				}
			})
		})
		row(m.WindowTitle(), v.appTitle(), func() {
			if ui.Button(c, m.Edit()).Clicked() {
				v.openTitleDialog()
			}
		})
	})
}

// settingsBehavior is the page of starting and summoning the launcher, the
// window it shows, using its items, and how it keeps running.
func (v *view) settingsBehavior(c *ui.Context, m locales.Messages, group settingsGroup, row settingsRow) {
	s := &v.app.store.Settings
	group(m.StartupAndWindow(), func() {
		row(m.OpenAtLogin(), m.OpenAtLoginHint(), func() {
			on := v.openAtLogin
			if ui.Switch(c, &on).Label(m.ToggleOpenAtLogin()).Changed() {
				v.changeOpenAtLogin(on)
			}
		})
		row(m.GlobalShortcut(), "", func() {
			v.shortcutControl(c)
		})
		row(m.WindowPosition(), m.WindowPositionHint(), func() {
			places := []string{"last", "cursor", "center"}
			i := 0
			for k, place := range places {
				if place == v.prefs.WindowPosition {
					i = k
				}
			}
			if segmented(c.Key("window-position"), &i, m.LastPosition(), m.AtCursor(), m.ScreenCenter()) {
				v.prefs.WindowPosition = places[i]
				v.changedSettings()
			}
		})
		row(m.AlwaysOnTop(), "", func() {
			if ui.Switch(c, &s.AlwaysOnTop).Label(m.ToggleAlwaysOnTop()).Changed() {
				v.app.setAlwaysOnTop(s.AlwaysOnTop)
			}
		})
	})
	group(m.Actions(), func() {
		row(m.AutoHide(), "", func() {
			if ui.Switch(c, &s.AutoHide).Label(m.ToggleAutoHide()).Changed() {
				v.app.save()
			}
		})
		row(m.WheelPages(), m.WheelPagesHint(), func() {
			if ui.Switch(c, &v.prefs.WheelPages).Label(m.ToggleWheelPages()).Changed() {
				v.setWheelPages(v.prefs.WheelPages)
			}
		})
		row(m.AutoTiming(), m.AutoTimingHint(), func() {
			if ui.Switch(c, &s.AutoTime).Label(m.ToggleAutoTiming()).Changed() {
				if !s.AutoTime {
					// Everything the timer did stops at once: its dialog,
					// the tracked processes and the redraws that counted.
					v.runtimeOpen = false
					v.app.stopTiming()
				}
				v.app.save()
			}
		})
	})
	group(m.BackgroundAndPerformance(), func() {
		row(m.GpuDrawing(), m.GpuDrawingHint(), func() {
			if ui.Switch(c, &v.prefs.GPU).Label(m.ToggleGpuDrawing()).Changed() {
				v.changedSettings()
				// The backend is picked before the window opens, so the new
				// one needs a fresh process: the app restarts itself.
				relaunchApp()
			}
		})
		row(m.BackgroundOnClose(), "", func() {
			if ui.Switch(c, &s.BackgroundOnClose).Label(m.ToggleBackgroundOnClose()).Changed() {
				v.setBackgroundOnClose(s.BackgroundOnClose)
			}
		})
		row(m.TrayOnly(), "", func() {
			on := v.prefs.TrayOnly
			sw := ui.Switch(c, &on).Label(m.ToggleTrayOnly()).Disabled(!s.BackgroundOnClose)
			if sw.Changed() {
				v.setTrayOnly(on)
			}
		})
	})
}

// settingsData is the page of how item paths are stored, and of the
// launcher's data: reloading it, its folder, the icons, the availability check
// and the CLaunch migration.
func (v *view) settingsData(c *ui.Context, m locales.Messages, group settingsGroup, row settingsRow) {
	s := &v.app.store.Settings
	group(m.Paths(), func() {
		row(m.AbsolutePaths(), m.AbsolutePathsHint(), func() {
			if ui.Switch(c, &s.AbsolutePaths).Label(m.ToggleAbsolutePaths()).Changed() {
				v.app.save()
			}
		})
		row(m.ConvertAllPaths(), "", func() {
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
		row(m.CheckAvailability(), m.CheckAvailabilityHint(), func() {
			if ui.Button(c, m.CheckAvailability()).Clicked() {
				v.checkAvailability()
			}
		})
		row(m.MigrateCLaunch(), v.migrateHint(m), func() {
			if ui.Button(c, m.ChooseCLaunchIni()).Clicked() {
				v.migrateCLaunch()
			}
		})
	})
}

// settingsAbout is the page of the app's name and version, its updates, and
// restarting or quitting it.
func (v *view) settingsAbout(c *ui.Context, m locales.Messages, group settingsGroup, row settingsRow) {
	p := v.pal
	group("", func() {
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
		ui.Row(c).Padding(8, 14).Gap(8).Justify(ui.End).MinHeight(42).Children(func() {
			if ui.Button(c, m.Restart()).Clicked() {
				mygo.App.Relaunch()
			}
			if ui.Button(c, m.Exit()).Clicked() {
				mygo.App.Quit()
			}
		})
	})
}

// openTitleDialog shows the window title dialog, starting from the title in
// use: none keeps the app's name.
func (v *view) openTitleDialog() {
	v.titleDraft = v.prefs.Title
	v.titleOpen = true
	v.app.refresh()
}

// titleDialog edits the title the windows and the tray icon show. An empty
// title puts the app's name back.
func (v *view) titleDialog(c *ui.Context) {
	if !v.titleOpen {
		return
	}
	m := v.msgs()
	ui.Modal(c, &v.titleOpen, func() {
		ui.Column(c).Width(dialogWidth(c, 380)).Gap(12).Children(func() {
			ui.Text(c, m.WindowTitle()).FontSize(17).FontWeight(700)
			if ui.TextInput(c, &v.titleDraft).AutoFocus().Placeholder(appName).Submitted() {
				v.applyTitle()
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				ui.Button(c, m.Cancel()).OnClick(func() { v.titleOpen = false; v.app.refresh() })
				ui.PrimaryButton(c, m.Ok()).OnClick(v.applyTitle)
			})
		})
	})
}

// applyTitle saves the typed title and puts it on the main window, the
// settings window and the tray icon.
func (v *view) applyTitle() {
	v.prefs.Title = strings.TrimSpace(v.titleDraft)
	v.titleOpen = false
	savePrefs(v.prefs)
	title := v.appTitle()
	if v.app.win != nil {
		v.app.win.SetTitle(title)
	}
	if w := v.app.settingsWin; w != nil {
		w.SetTitle(locales.In(mygo.App.Locale()).Settings() + " - " + title)
	}
	v.syncTray()
	v.app.refresh()
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
			w.SetTitle(locales.In(mygo.App.Locale()).Settings() + " - " + v.appTitle())
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
