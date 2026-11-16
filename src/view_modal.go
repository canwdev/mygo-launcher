package main

import "github.com/egoist/mygo/ui"

// modal shows fn in a dialog built the way ui.Modal builds one, except that
// its panel keeps the palette's own background. With a material behind the
// window the theme's background is see-through, and MyGo's dialogs paint that
// one: a dialog you can read the window through is not one.
func (v *view) modal(c *ui.Context, open *bool, fn func()) {
	t := c.Theme()
	p := v.pal
	w, h := c.Size()
	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.4))
		panel.Padding(t.Space(5)).Gap(t.Space(3)).Radius(t.Space(2.5)).
			Background(p.bg).
			MaxWidth(w-t.Space(10)).MaxHeight(h-t.Space(10)).
			Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.3))
		fn()
	})
}
