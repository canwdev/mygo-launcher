package main

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// listWindows is the picker's source of running windows, replaced in tests.
var listWindows = listRunningWindows

// pickerMenuKind is the second-level list the add menu shows.
type pickerMenuKind uint8

const (
	pickerClosed pickerMenuKind = iota
	pickerWindows
	pickerLocations
)

// pickerEntry is one choice of a second-level list: a running program or a
// system location, with the path its icon comes from.
type pickerEntry struct {
	Path     string
	Name     string
	Title    string // a second line, for a running window
	Icon     *ui.SVG
	Existing string // the guid of the item already on the page, if any
	Add      func()
}

// openWindowPicker shows the add menu above the add button. The running
// windows are read when their list opens, not here.
func (v *view) openWindowPicker() {
	v.pickerOpen = true
	v.pickerMenu = pickerClosed
	v.pickerWindows = nil
	v.pickerScroll.ScrollTo(0, 0, ui.Animation{})
	v.loadPickerIcons()
	v.refresh()
}

func (v *view) closeWindowPicker() {
	v.pickerOpen = false
	v.pickerMenu = pickerClosed
	v.refresh()
}

// loadPickerIcons reads the icons of the listed programs off the main thread,
// as they take a few milliseconds each.
func (v *view) loadPickerIcons() {
	if v.app.win == nil {
		return
	}
	if v.pickerLoading {
		// A list opened while the icons still load: read the paths it added
		// once the running batch is done.
		v.pickerIconReload = true
		return
	}
	v.pickerIconReload = false
	if v.pickerIcons == nil {
		v.pickerIcons = map[string]ui.ImageSource{}
	}
	var missing []string
	seen := map[string]bool{}
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		if _, ok := v.pickerIcons[path]; !ok {
			missing = append(missing, path)
		}
	}
	for _, w := range v.pickerWindows {
		add(w.Path)
	}
	for _, loc := range systemLocations {
		add(loc.Path)
	}
	if len(missing) == 0 {
		return
	}
	v.pickerLoading = true
	go func() {
		type loadedIcon struct {
			path string
			data []byte
		}
		var got []loadedIcon
		for _, path := range missing {
			if data, err := mygo.Shell.FileIcon(path, iconPixelsFor(20)); err == nil && len(data) > 0 {
				got = append(got, loadedIcon{path, data})
			}
		}
		v.app.dispatch(func() {
			if v.pickerIcons == nil {
				v.pickerIcons = map[string]ui.ImageSource{}
			}
			for _, icon := range got {
				if src, err := ui.DecodeImage(icon.data); err == nil {
					v.pickerIcons[icon.path] = src
				}
			}
			v.pickerLoading = false
			if v.pickerIconReload {
				v.loadPickerIcons()
			}
			v.refresh()
		})
	}()
}

// windowPicker shows the add menu above the add button: its two lists open as
// second-level panels beside the entry that asks for them, and files or
// hand-made items are added below them.
func (v *view) windowPicker(c *ui.Context, anchor ui.Element) {
	if !v.pickerOpen {
		return
	}
	p := v.pal
	m := v.msgs()
	width, _ := c.Size()
	panelWidth := min(220, width-40)

	ui.Overlay(c, func() {
		back := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Modal()
		if back.OverlayShortcut(0, ui.KeyEscape) {
			v.closeWindowPicker()
		}
		back.OnClick(v.closeWindowPicker)
		back.Children(func() {
			panel := ui.Column(c).Width(panelWidth).Padding(6).Radius(12).
				Background(p.bg).Border(1, p.hairline).
				Shadow(0, 12, 30, -6, ui.RGBA(0, 0, 0, 0.3)).
				Role(ui.RolePopup).OnClick(func() {}).
				AttachTo(anchor, ui.AnchorTopRight, ui.AnchorBottomRight).Margin(0, 0, 8, 0)
			var windowsRow, locationsRow ui.Element
			panel.Children(func() {
				windowsRow = v.pickerCategory(c, iconBox, m.RunningWindows(), v.pickerMenu == pickerWindows, func() {
					v.openPickerMenu(pickerWindows)
				})
				locationsRow = v.pickerCategory(c, iconFolder, m.SystemLocations(), v.pickerMenu == pickerLocations, func() {
					v.openPickerMenu(pickerLocations)
				})
				ui.Box(c).Height(1).Margin(4, 6).Background(p.hairline)
				v.pickerAction(c, iconBox, m.PickFiles(), func() {
					v.pickerOpen = false
					v.later(func() { v.app.addFiles(v.tabGUID()) })
				})
				v.pickerAction(c, iconEdit, m.Create(), func() {
					v.pickerOpen = false
					v.later(v.openCreate)
				})
			})
			switch v.pickerMenu {
			case pickerWindows:
				v.pickerSubmenu(c, windowsRow, m.RunningWindows(), v.windowEntries(), m.NoRunningWindows())
			case pickerLocations:
				v.pickerSubmenu(c, locationsRow, m.SystemLocations(), v.locationEntries(), "")
			}
		})
	})
}

// openPickerMenu shows a second-level list, or closes it when it already
// shows. The running windows are read again each time their list opens, so
// windows that appeared or closed show as they are now.
func (v *view) openPickerMenu(kind pickerMenuKind) {
	if v.pickerMenu == kind {
		v.pickerMenu = pickerClosed
		v.pickerScroll.ScrollTo(0, 0, ui.Animation{})
		v.refresh()
		return
	}
	if kind == pickerWindows {
		v.pickerWindows = listWindows()
		v.loadPickerIcons()
	}
	v.pickerMenu = kind
	v.pickerScroll.ScrollTo(0, 0, ui.Animation{})
	v.refresh()
}

// pickerCategory is a menu row that opens a second-level list, and returns the
// element the list is attached to.
func (v *view) pickerCategory(c *ui.Context, icon *ui.SVG, label string, active bool, action func()) ui.Element {
	p := v.pal
	row := ui.Row(c).Height(40).Padding(0, 8).Gap(10).Radius(8).AlignItems(ui.Center)
	if active {
		row.Background(p.press)
	} else if row.Hovered() {
		row.Background(p.hover)
	}
	if row.Clicked() {
		action()
	}
	row.Children(func() {
		ui.Icon(c, icon).Size(16, 16).TextColor(p.muted)
		ui.Text(c, label).Grow(1).SingleLine().FontSize(13).TextColor(p.text)
		ui.Icon(c, iconChevronRight).Size(14, 14).TextColor(p.faint)
	})
	return row
}

// pickerSubmenu shows one second-level list beside the row that opened it.
// Both lists use pickerRow, so they scroll and add items the same way.
func (v *view) pickerSubmenu(c *ui.Context, anchor ui.Element, title string, entries []pickerEntry, empty string) {
	p := v.pal
	width, height := c.Size()
	panelWidth := min(320, width-40)
	listMax := min(float32(288), height-40)
	if listMax < 44 {
		listMax = 44
	}
	// Cut at a row's edge, so the list does not end halfway down one.
	listMax = float32(int(listMax/44)) * 44
	if listMax < 44 {
		listMax = 44
	}
	listHeight := min(float32(len(entries))*44, listMax)
	if listHeight < 44 {
		listHeight = 44
	}

	panel := ui.Column(c).Width(panelWidth).Padding(6).Radius(12).
		Background(p.bg).Border(1, p.hairline).
		Shadow(0, 12, 30, -6, ui.RGBA(0, 0, 0, 0.3)).
		Role(ui.RolePopup).OnClick(func() {}).
		AttachTo(anchor, ui.AnchorTopLeft, ui.AnchorTopRight).Margin(0, 8, 0, 0)
	panel.Children(func() {
		ui.Text(c, title).FontSize(11).TextColor(p.muted).Padding(2, 8)
		ui.Scroll(c).TrackScroll(&v.pickerScroll).Height(listHeight).Children(func() {
			ui.Column(c).Children(func() {
				if len(entries) == 0 && empty != "" {
					ui.Text(c, empty).FontSize(12).TextColor(p.muted).Padding(10, 8)
				}
				for _, entry := range entries {
					v.pickerRow(c, entry)
				}
			})
		})
	})
}

// pickerRow is one choice of a second-level list: its icon, its name and, for
// a running program, its window title. Choosing it adds the item and closes
// the menu.
func (v *view) pickerRow(c *ui.Context, entry pickerEntry) {
	p := v.pal
	row := ui.Row(c).Height(44).Padding(0, 8).Gap(10).Radius(8).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(p.hover)
	}
	if row.Clicked() {
		v.pickerOpen = false
		v.pickerMenu = pickerClosed
		if entry.Existing != "" {
			v.later(func() { v.locateItem(entry.Existing, v.tabGUID()) })
		} else {
			v.later(entry.Add)
		}
	}
	row.Children(func() {
		if src := v.pickerIcons[entry.Path]; src != nil {
			ui.Image(c, src).Size(20, 20)
		} else {
			icon := entry.Icon
			if icon == nil {
				icon = iconBox
			}
			ui.Box(c).Size(20, 20).Radius(6).Background(p.hover).Center().Children(func() {
				ui.Icon(c, icon).Size(12, 12).TextColor(p.faint)
			})
		}
		if entry.Title != "" {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, entry.Name).SingleLine().FontSize(13).TextColor(p.text)
				ui.Text(c, entry.Title).SingleLine().FontSize(11).TextColor(p.muted)
			})
		} else {
			ui.Text(c, entry.Name).SingleLine().FontSize(13).TextColor(p.text).Grow(1)
		}
		if entry.Existing != "" {
			ui.Icon(c, iconCheck).Size(14, 14).TextColor(p.accent)
		}
	})
}

// windowEntries lists the running programs as second-level entries.
func (v *view) windowEntries() []pickerEntry {
	tab := v.tabGUID()
	entries := make([]pickerEntry, 0, len(v.pickerWindows))
	for _, w := range v.pickerWindows {
		w := w
		entries = append(entries, pickerEntry{
			Path: w.Path, Name: w.Name, Title: w.Title, Icon: iconBox,
			Add: func() { v.app.addRunningWindow(w, tab) },
		})
	}
	return entries
}

// locationEntries lists the system folders as second-level entries.
func (v *view) locationEntries() []pickerEntry {
	tab := v.tabGUID()
	m := v.msgs()
	entries := make([]pickerEntry, 0, len(systemLocations))
	for _, loc := range systemLocations {
		loc := loc
		name := loc.Name(m)
		existing := ""
		if item := v.tabItemByPath(tab, loc.Path); item != nil {
			existing = item.GUID
		}
		entries = append(entries, pickerEntry{
			Path: loc.Path, Name: name, Icon: iconFolder, Existing: existing,
			Add: func() { v.app.addSystemLocation(loc, name, tab) },
		})
	}
	return entries
}

// tabItemByPath returns the item of a tab with path, so the picker can mark a
// location the page already holds and locate it instead of adding it twice.
func (v *view) tabItemByPath(tabGUID, path string) *AppItem {
	tab := v.app.tab(tabGUID)
	if tab == nil {
		return nil
	}
	for _, slot := range tab.Slots {
		if slot == nil {
			continue
		}
		if item := v.app.item(*slot); item != nil && strings.EqualFold(item.Path, path) {
			return item
		}
	}
	return nil
}

// pickerAction is a row of the picker below the windows, as the add menu's
// items were.
func (v *view) pickerAction(c *ui.Context, icon *ui.SVG, label string, action func()) {
	p := v.pal
	row := ui.Row(c).Height(40).Padding(0, 8).Gap(10).Radius(8).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(p.hover)
	}
	if row.Clicked() {
		action()
	}
	row.Children(func() {
		ui.Icon(c, icon).Size(16, 16).TextColor(p.muted)
		ui.Text(c, label).FontSize(13).TextColor(p.text)
	})
}
