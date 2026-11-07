package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// listWindows is the picker's source of running windows, replaced in tests.
var listWindows = listRunningWindows

// openWindowPicker lists the programs with a window open and shows the picker.
func (v *view) openWindowPicker() {
	v.pickerOpen = true
	v.pickerWindows = listWindows()
	v.pickerScroll.ScrollTo(0, 0, ui.Animation{})
	v.loadPickerIcons()
	v.refresh()
}

func (v *view) closeWindowPicker() {
	v.pickerOpen = false
	v.refresh()
}

// loadPickerIcons reads the icons of the listed programs off the main thread,
// as they take a few milliseconds each.
func (v *view) loadPickerIcons() {
	if v.app.win == nil || v.pickerLoading {
		return
	}
	if v.pickerIcons == nil {
		v.pickerIcons = map[string]ui.ImageSource{}
	}
	var missing []runningWindow
	for _, w := range v.pickerWindows {
		if _, ok := v.pickerIcons[w.Path]; !ok {
			missing = append(missing, w)
		}
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
		for _, w := range missing {
			if data, err := mygo.Shell.FileIcon(w.Path, iconPixelsFor(20)); err == nil && len(data) > 0 {
				got = append(got, loadedIcon{w.Path, data})
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
			v.refresh()
		})
	}()
}

// windowPicker shows the programs with a window open above the add button,
// with the file and hand-made ways to add an item below them.
func (v *view) windowPicker(c *ui.Context, anchor ui.Element) {
	if !v.pickerOpen {
		return
	}
	p := v.pal
	m := v.msgs()
	width, height := c.Size()
	panelWidth := min(360, width-40)
	// Leave the add button and the window's edges room; the list takes the
	// rest, and scrolls when it is taller.
	listMax := min(float32(288), height-78-130)
	if listMax < 44 {
		listMax = 44
	}
	// Cut at a row's edge, so the list does not end halfway down one.
	listMax = float32(int(listMax/44)) * 44
	if listMax < 44 {
		listMax = 44
	}
	listHeight := min(float32(len(v.pickerWindows))*44, listMax)
	if listHeight < 44 {
		listHeight = 44
	}

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
			panel.Children(func() {
				ui.Text(c, m.RunningWindows()).FontSize(11).TextColor(p.muted).Padding(2, 8)

				ui.Scroll(c).TrackScroll(&v.pickerScroll).Height(listHeight).Children(func() {
					ui.Column(c).Children(func() {
						if len(v.pickerWindows) == 0 {
							ui.Text(c, m.NoRunningWindows()).FontSize(12).TextColor(p.muted).Padding(10, 8)
						}
						for _, w := range v.pickerWindows {
							v.windowPickerRow(c, w)
						}
					})
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
		})
	})
}

// windowPickerRow is one running program: choosing it adds the program to the
// page shown.
func (v *view) windowPickerRow(c *ui.Context, w runningWindow) {
	p := v.pal
	row := ui.Row(c).Height(44).Padding(0, 8).Gap(10).Radius(8).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(p.hover)
	}
	if row.Clicked() {
		tab := v.tabGUID()
		v.pickerOpen = false
		v.later(func() { v.app.addRunningWindow(w, tab) })
	}
	row.Children(func() {
		if src := v.pickerIcons[w.Path]; src != nil {
			ui.Image(c, src).Size(20, 20)
		} else {
			ui.Box(c).Size(20, 20).Radius(6).Background(p.hover).Center().Children(func() {
				ui.Icon(c, iconBox).Size(12, 12).TextColor(p.faint)
			})
		}
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, w.Name).SingleLine().FontSize(13).TextColor(p.text)
			ui.Text(c, w.Title).SingleLine().FontSize(11).TextColor(p.muted)
		})
	})
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
