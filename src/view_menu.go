package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"
)

// menuEntry is one row of a context menu: its icon, its label and what it
// runs. A separator has no label.
type menuEntry struct {
	icon   *ui.SVG
	label  string
	action func()
}

// Menu keys name a menu by the element it belongs to, so that two cells of the
// same item keep their own open state.
func itemMenuKey(tabGUID, guid string, slot int) string {
	return "item:" + tabGUID + ":" + guid + ":" + strconv.Itoa(slot)
}

func emptyMenuKey(tabGUID string, slot int) string {
	return "empty:" + tabGUID + ":" + strconv.Itoa(slot)
}

func tabMenuKey(guid string) string { return "tab:" + guid }

// menuOpen returns the flag that keeps a context menu open: MyGo's
// ContextMenuBase shows the menu while it is true, and the menu state lives as
// long as the element it belongs to.
func (v *view) menuOpen(key string) *bool {
	if v.menus == nil {
		v.menus = map[string]*bool{}
	}
	open := v.menus[key]
	if open == nil {
		open = new(bool)
		v.menus[key] = open
	}
	return open
}

// contextMenu gives target a right-click menu of entries, drawn in the window
// as MyGo draws its popover menus: the items take the keyboard, Escape and a
// press outside close it.
func (v *view) contextMenu(c *ui.Context, target ui.Element, key string, entries []menuEntry) {
	if len(entries) == 0 {
		return
	}
	parts := ui.ContextMenuBase(c, target, v.menuOpen(key))
	parts.Popup(func(panel ui.Element) {
		v.menuPanel(c, parts, panel, entries)
	})
}

// menuPanel styles a menu and builds its rows. The item highlighted by the
// pointer or the keys takes the accent color, as the menus MyGo draws do.
func (v *view) menuPanel(c *ui.Context, parts ui.MenuParts, panel ui.Element, entries []menuEntry) {
	p := v.pal
	panel.MinWidth(176).Padding(5).Radius(10).Background(p.bg).Border(1, p.hairline).
		Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
	for _, entry := range entries {
		if entry.label == "" {
			ui.Box(c).Height(1).Margin(5, 4).Background(p.hairline)
			continue
		}
		item := parts.Item().Height(32).Padding(0, 8).Gap(10).Radius(6).
			AlignItems(ui.Center).Label(entry.label)
		color := p.text
		if item.Highlighted() {
			color = p.onAccent
			item.Background(p.accent).TextColor(color)
		}
		item.Children(func() {
			if entry.icon != nil {
				ui.Icon(c, entry.icon).Size(16, 16).TextColor(color)
			} else {
				// A row without an icon keeps the icons' column, so its label
				// lines up with the others.
				ui.Box(c).Size(16, 16).Shrink(0)
			}
			ui.Text(c, entry.label).FontSize(13).TextColor(color).SingleLine().Grow(1)
		})
		if item.Clicked() {
			entry.action()
		}
	}
}

// emptyMenu is an empty cell's menu: it adds another empty cell there, or
// takes this one away.
func (v *view) emptyMenu(slot int) []menuEntry {
	msgs := v.msgs()
	return []menuEntry{
		{icon: iconSquarePlus, label: msgs.InsertEmpty(), action: func() {
			v.later(func() { v.insertEmptyAt(slot) })
		}},
		{icon: iconSquareMinus, label: msgs.RemoveEmpty(), action: func() {
			v.later(func() { v.deleteSlotAt(slot) })
		}},
	}
}
