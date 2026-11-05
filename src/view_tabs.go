package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// tabHoverDelay is how long something being dragged rests on a page before the
// page shows itself, so that it can be dropped where the pointer aims.
const tabHoverDelay = 200 * time.Millisecond

// tabChip is one tab of the top navigation: it takes the click that shows it,
// drags onto another place in the strip, takes items dragged onto it (moving
// them, or copying them with Cmd/Ctrl held), and opens its menu on a right
// click.
func (v *view) tabChip(c *ui.Context, index int) {
	tab := &v.app.store.Categories[index]
	guid, name := tab.GUID, tab.Name
	selected := guid == v.prefs.ActiveTab
	p := v.pal

	// An element takes one type of dragged value, so the chip inside takes
	// the items and this wrapper, its place in the strip, takes the tabs
	// dragged over it.
	slot := ui.Row(c).AlignItems(ui.Center)
	_, overTab := ui.DragOver[dragTab](slot)

	slot.Children(func() {
		b := ui.ButtonBase(c.Key(guid)).Height(32).PaddingX(12).Gap(4).Radius(7).
			AlignItems(ui.Center).FocusRing(false).Label(name)
		v.tabRects[guid] = b.Bounds()
		if dragged, ok := ui.Drop[dragTab](slot); ok && dragged.index != index {
			at := index
			if dropAfter(b, true) {
				at++
			}
			to := dropTo(dragged.index, at, false)
			v.later(func() { v.moveTab(dragged.index, to) })
		}

		// Files from Explorer dropped on a tab are added to it, rather than
		// to the active tab.
		if files := b.DroppedFiles(); len(files) > 0 {
			v.app.addPaths(files, guid)
		}
		filesOver := b.FileDragOver()

		switch {
		case selected:
			b.Background(p.press)
		case b.Hovered():
			b.Background(p.hover)
		}
		// The line shows where a dragged page lands: at the left edge of the
		// chip when the pointer aims before it, at the right edge when after.
		if overTab {
			insertLine(b, true, dropAfter(b, true), p)
		}

		// An item dragged over the tab: dropping moves it there, Cmd/Ctrl
		// copies. Resting on it a moment shows the page, so the drop can aim
		// at a place in it; files from Explorer do the same.
		_, overItem := ui.DragOver[dragItem](b)
		if (overItem || filesOver) && !overTab {
			b.Border(2, p.accent)
		}
		if overItem || filesOver {
			if v.dragHoverTab != guid {
				v.dragHoverTab, v.dragHoverSince = guid, c.Now()
			}
			if !selected {
				c.After(tabHoverDelay)
				if c.Now().Sub(v.dragHoverSince) >= tabHoverDelay {
					v.later(func() { v.setActiveTab(guid) })
				}
			}
		} else if v.dragHoverTab == guid {
			v.dragHoverTab = ""
		}
		if dragged, ok := ui.Drop[dragItem](b); ok {
			copyItem := c.Modifiers()&ui.Cmd != 0
			// Dropping an item on the page it already belongs to changes
			// nothing; copying it there still makes another.
			if copyItem || dragged.tab != guid {
				v.later(func() { v.moveItemToTab(dragged.guid, guid, copyItem) })
			}
		}

		// Tabs themselves reorder by dragging.
		b.Drag(dragTab{index: index})
		if b.Dragging() {
			b.Opacity(0.4)
		}
		if b.Clicked() {
			v.centerNav = true
			v.setActiveTab(guid)
		}
		// Resting the pointer on a page shows it after a moment.
		if b.Hovered() && !selected && !overItem && !filesOver {
			if v.hoverTab != guid {
				v.hoverTab, v.hoverSince = guid, c.Now()
			}
			c.After(tabHoverDelay)
			if c.Now().Sub(v.hoverSince) >= tabHoverDelay {
				v.later(func() { v.setActiveTab(guid) })
			}
		} else if v.hoverTab == guid {
			v.hoverTab = ""
		}
		menu := v.tabMenu(guid, name)
		b.PopoverMenu(menu)

		b.Children(func() {
			color, weight := p.muted, 500
			if selected {
				color, weight = p.text, 600
			}
			ui.Text(c, name).FontSize(13).FontWeight(weight).TextColor(color).SingleLine()
		})
	})
}

// tabItemCount is how many items a tab holds, empty cells aside.
func tabItemCount(tab *CategoryNode) int {
	n := 0
	for _, slot := range tab.Slots {
		if slot != nil {
			n++
		}
	}
	return n
}

// tabMenu is a tab's menu, shared by a right click and the ⋯ of the chip.
func (v *view) tabMenu(guid, name string) func(m *ui.Menu) {
	msgs := v.msgs()
	// The last page stays: it is the window's only place for its items.
	removable := len(v.app.store.Categories) > 1
	return func(m *ui.Menu) {
		if m.Item(msgs.Rename()).Chosen() {
			v.later(func() { v.openTabRename(guid, name) })
		}
		if m.Item(msgs.Duplicate()).Chosen() {
			v.later(func() { v.duplicateTab(guid) })
		}
		if removable {
			m.Separator()
			if m.Item(msgs.Remove()).Chosen() {
				v.later(func() { v.requestDeleteTab(guid, name) })
			}
		}
	}
}

// openTabRename asks for a tab's new name.
func (v *view) openTabRename(guid, name string) {
	v.renameOpen = true
	v.renameTitle = v.msgs().RenameTab()
	v.renameName = name
	v.renameGUID = ""
	v.renameTabGUID = guid
}

// requestDeleteTab deletes an empty tab at once, and asks about one with items.
func (v *view) requestDeleteTab(guid, name string) {
	if len(v.app.store.Categories) <= 1 {
		return
	}
	tab := v.app.tab(guid)
	if tab == nil {
		return
	}
	if tabItemCount(tab) == 0 {
		v.removeTab(guid)
		return
	}
	v.ask(v.msgs().RemoveTabConfirm(name), func() { v.removeTab(guid) })
}

// ask opens the confirmation alert, which runs action when it is accepted.
func (v *view) ask(message string, action func()) {
	v.confirmMsg = message
	v.confirmAction = action
	v.confirmOpen = true
}
