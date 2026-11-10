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
	hint := m.AutoTiming()
	if timed {
		hint = m.RuntimeRunningHint()
	}
	// An item with no time yet shows the clock alone: the placeholder would
	// only add dashes to a cell that has nothing to say.
	text := ""
	if v.baseline(guid) > 0 || v.liveMs(guid) > 0 {
		text = v.runtimeText(guid)
	}
	field := ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(2, 5).Radius(5)
	field.Tooltip(hint)
	if !timed {
		field.Cursor(ui.CursorPointer)
	}
	if field.Hovered() && !timed && !v.keyboardNav {
		field.Background(p.hover)
	}
	field.OnClick(func() {
		if !timed {
			v.openRuntime(guid)
		}
	})
	field.Children(func() {
		ui.Icon(c, iconClock).Size(11, 11).TextColor(color)
		if text != "" {
			ui.Text(c, text).FontSize(11.5).FontFeatures("tnum").TextColor(color)
		}
	})
	return field
}

// itemIcon shows an item's cached icon, or its initial when it has none. With
// badge, the administrator shield sits in its bottom-left corner; with gray,
// the icon is drawn without its colors.
func (v *view) itemIcon(c *ui.Context, item *AppItem, size float32, badge, gray bool) {
	p := v.pal
	box := ui.Box(c).Size(size, size)
	box.Children(func() {
		if src := v.app.iconImage(item); src != nil {
			img := ui.Image(c, src).Size(size, size).Fit(ui.Contain)
			if gray {
				img.Grayscale()
			}
		} else {
			fallback := ui.Box(c).Size(size, size).Radius(size / 4).Background(p.hover).Center()
			fallback.Children(func() {
				ui.Text(c, initial(item.Name)).FontSize(size * 0.42).FontWeight(600).TextColor(p.muted)
			})
		}
		if badge {
			v.adminBadge(c, size)
		}
	})
}

// adminBadge marks an elevated item over the bottom-left corner of its icon,
// as Windows marks its shortcuts.
func (v *view) adminBadge(c *ui.Context, iconSize float32) {
	d := max(13, iconSize*0.3)
	badge := ui.Box(c).Absolute().Left(0).Bottom(0).Size(d, d).Radius(999).
		Center().Background(v.pal.bg).Role(ui.RoleImage).
		Label(v.msgs().RunAsAdministrator())
	badge.Children(func() {
		ui.Icon(c, iconShield).Size(d*0.72, d*0.72).TextColor(c.Theme().Warning)
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
