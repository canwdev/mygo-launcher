package main

import "github.com/egoist/mygo/ui"

// The accelerators the window itself takes, shown beside the buttons' names.
// They are key names, not words to translate.
const (
	searchAccelerator   = "Ctrl+F"
	settingsAccelerator = "Ctrl+,"
)

// withAccelerator names what a control does and the key that does it.
func withAccelerator(label, accelerator string) string {
	return label + " (" + accelerator + ")"
}

// header is the strip along the top of the window: the tabs and the window's
// controls. It is the window's navigation, drawn on the sidebar's background
// as a toolbar.
func (v *view) header(c *ui.Context) {
	p := v.pal
	m := v.msgs()
	// With a background picture the bar shares the content's base color, so
	// the picture's top runs into the grid without a seam.
	base := p.sidebar
	shown := v.backgroundShown()
	if shown {
		base = p.bg
	}
	bar := ui.Row(c).Height(50).Padding(0, 12).Gap(8).AlignItems(ui.Center).
		Background(base).BorderWidth(0, 0, 1, 0).BorderColor(p.hairline)
	if shown {
		bar.Draw(v.drawBackground)
	}
	bar.DragWindow().Children(func() {
		// The strip scrolls sideways with its native scroll bar when the
		// pages do not fit.
		nav := ui.ScrollHorizontal(c).TrackScroll(&v.navScroll).Height(36).Grow(1).Shrink(1).
			MinWidth(0)
		nav.Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				for i := range v.app.store.Categories {
					v.tabChip(c, i)
				}
				if iconButton(c, iconPlus, 13, p.faint, m.AddTab()).Size(24, 24).Radius(7).
					Tooltip(m.AddTab()).Clicked() {
					v.later(v.addTab)
				}
			})
		})
		v.navRect = nav.Bounds()
		if iconButton(c, iconSearch, 16, p.muted, m.Search()).
			Tooltip(withAccelerator(m.Search(), searchAccelerator)).Clicked() {
			v.openSearch()
		}
		if iconButton(c, iconGear, 18, p.muted, m.Settings()).
			Tooltip(withAccelerator(m.Settings(), settingsAccelerator)).Clicked() {
			v.openSettings()
		}
	})
}
