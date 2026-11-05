package main

import "github.com/egoist/mygo/ui"

// header is the strip along the top of the window: the tabs and the window's
// controls. It is the window's navigation, drawn on the sidebar's background
// as a toolbar.
func (v *view) header(c *ui.Context) {
	p := v.pal
	m := v.msgs()
	ui.Row(c).Height(56).Padding(0, 16).Gap(10).AlignItems(ui.Center).
		Background(p.sidebar).BorderWidth(0, 0, 1, 0).BorderColor(p.hairline).
		DragWindow().Children(func() {
		ui.ScrollHorizontal(c).TrackScroll(&v.navScroll).Height(36).Grow(1).Shrink(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				for i := range v.app.store.Categories {
					v.tabChip(c, i)
				}
				if iconButton(c, iconPlus, 13, p.faint, m.AddTab()).Size(26, 26).Radius(7).Clicked() {
					v.later(v.addTab)
				}
			})
		})
		if iconButton(c, iconSearch, 16, p.muted, m.Search()).Clicked() {
			v.openSearch()
		}
		v.viewModeButton(c)
		if iconButton(c, iconGear, 18, p.muted, m.Settings()).Clicked() {
			v.openSettings()
		}
	})
}

// viewModeButton switches between the list and the grid, just before Settings.
func (v *view) viewModeButton(c *ui.Context) {
	p := v.pal
	m := v.msgs()
	if v.prefs.ViewMode == "grid" {
		if iconButton(c, iconList, 17, p.muted, m.SwitchToList()).Clicked() {
			v.setViewMode("list")
		}
		return
	}
	if iconButton(c, iconGrid, 17, p.muted, m.SwitchToGrid()).Clicked() {
		v.setViewMode("grid")
	}
}
