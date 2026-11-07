package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// runtimeField shows an item's accumulated time, with what a running process is
// adding. Clicking it edits the runtime. It returns the field, which the grid
// places in a cell's corner.
func (v *view) runtimeField(c *ui.Context, item *AppItem, st ItemState) ui.Element {
	p := v.pal
	guid := item.GUID
	color := p.muted
	// A running process locks the runtime only while its clock is moving.
	timed := st.Running && v.app.store.Settings.AutoTime

	m := v.msgs()
	hint := m.RuntimeEditHint()
	if timed {
		hint = m.RuntimeRunningHint()
	}
	field := ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(2, 5).Radius(5)
	field.Tooltip(hint)
	if !timed {
		field.Cursor(ui.CursorPointer)
	}
	if field.Hovered() && !timed {
		field.Background(p.hover)
	}
	field.OnClick(func() {
		if !timed {
			v.openRuntime(guid)
		}
	})
	field.Children(func() {
		ui.Icon(c, iconClock).Size(11, 11).TextColor(color)
		ui.Text(c, v.runtimeText(guid)).FontSize(11.5).FontFeatures("tnum").TextColor(color)
	})
	return field
}

// itemIcon shows an item's cached icon, or its initial when it has none.
func (v *view) itemIcon(c *ui.Context, item *AppItem, size float32) {
	p := v.pal
	if src := v.app.iconImage(item); src != nil {
		ui.Image(c, src).Size(size, size).Fit(ui.Contain)
		return
	}
	box := ui.Box(c).Size(size, size).Radius(size / 4).Background(p.hover).Center()
	box.Children(func() {
		ui.Text(c, initial(item.Name)).FontSize(size * 0.42).FontWeight(600).TextColor(p.muted)
	})
}

func initial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}

// isLocated reports whether a search result brought this item into view a
// moment ago.
func (v *view) isLocated(guid string) bool {
	return guid != "" && guid == v.locate && time.Now().Before(v.locateTill)
}
