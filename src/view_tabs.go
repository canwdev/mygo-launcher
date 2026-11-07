package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// tabHoverDelay is how long the pointer rests on a page before the page shows
// itself. dragTabHoverDelay is the longer rest for something being dragged, so
// that passing over a chip on the way to a cell does not switch the page.
const (
	tabHoverDelay     = 200 * time.Millisecond
	dragTabHoverDelay = 300 * time.Millisecond
)

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
		// The chip being dragged draws no line of its own: the copy following
		// the pointer would carry it, and its own place is no destination.
		dragging := b.Dragging()
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
		if overTab && !dragging {
			insertLine(b, true, dropAfter(b, true), p)
		}

		// An item dragged over the tab: dropping moves it there, Cmd/Ctrl
		// copies. Resting on it a moment shows the page, so the drop can aim
		// at a place in it; files from Explorer do the same.
		_, overItem := ui.DragOver[dragItem](b)
		if (overItem || filesOver) && !overTab {
			// The chip takes an item: outline it without a border, which would
			// take room and move its name.
			b.DrawOver(func(pt *ui.Painter, r ui.Rect) {
				pt.Stroke(r, p.accent, 7, 2)
			})
		}
		if overItem || filesOver {
			if v.dragHoverTab != guid {
				v.dragHoverTab, v.dragHoverSince = guid, c.Now()
			}
			if !selected {
				c.After(dragTabHoverDelay)
				if c.Now().Sub(v.dragHoverSince) >= dragTabHoverDelay {
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
		if dragging {
			b.Opacity(0.4)
		}
		if b.Clicked() {
			v.centerNav = true
			v.setActiveTab(guid)
		}
		// Hovering another chip lets its hover work again after a wheel
		// switched the page; resting on the one the wheel was used on does not.
		if b.Hovered() && v.hoverIgnoredTab != "" && v.hoverIgnoredTab != guid {
			v.hoverIgnoredTab = ""
		}
		// Resting the pointer on a page shows it after a moment.
		if b.Hovered() && !selected && !overItem && !filesOver && guid != v.hoverIgnoredTab {
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
		v.contextMenu(c, b, tabMenuKey(guid), v.tabMenu(guid, name))

		b.Children(func() {
			color, weight := p.muted, 500
			if selected {
				color, weight = p.text, 600
			}
			ui.Text(c, name).FontSize(13).FontWeight(weight).TextColor(color).SingleLine()
		})
	})
}

// wheelInput switches pages with a vertical wheel when the preference asks, and
// takes the wheel so that the grid does not scroll. Shift swaps the two: while
// the wheel switches pages, Shift+wheel scrolls, and while it does not, Shift
// turns it on. The whole window content is the target; the system's title bar
// is outside it.
func (v *view) wheelInput(ev ui.InputEvent) bool {
	if v.folderGUID != "" {
		// The folder panel takes the wheel: its list scrolls, and nothing turns
		// the page behind it.
		return false
	}
	if ev.Kind != ui.InputScroll || ev.DY == 0 {
		return false
	}
	if ev.Mods != 0 && ev.Mods != ui.Shift {
		return false
	}
	if ev.Mods == ui.Shift && v.prefs.WheelPages {
		v.scrollPage(ev.DY)
		return true
	}
	if ev.Mods == 0 && !v.prefs.WheelPages {
		return false
	}
	if ev.Momentum {
		return true // a touchpad's fling would turn page after page
	}
	if guid := v.tabAt(ev.X, ev.Y); guid != "" {
		// The wheel switches the page under the pointer; resting on this chip
		// must not switch it back after its hover delay. The chip may be the
		// one shown already, which has no hover timer of its own.
		v.hoverIgnoredTab = guid
	}
	// A wheel's notch is one step; a touchpad needs more travel, since its
	// moves come in small pieces. One event turns at most one page, however
	// far it scrolls, so a fast wheel or a fling cannot skip pages.
	step := float32(40)
	if ev.Precise {
		step = 120
	}
	if ev.DY > 0 && v.wheelSum < 0 || ev.DY < 0 && v.wheelSum > 0 {
		v.wheelSum = 0 // the other way starts over
	}
	v.wheelSum += ev.DY
	switch {
	case v.wheelSum >= step:
		v.wheelSum = 0
		v.switchPage(1)
	case v.wheelSum <= -step:
		v.wheelSum = 0
		v.switchPage(-1)
	}
	return true
}

// scrollPage scrolls the page's grid by dy, for Shift+wheel while the wheel
// switches pages.
func (v *view) scrollPage(dy float32) {
	tab := v.tab()
	if tab == nil {
		return
	}
	pv := v.pageState(tab)
	pv.gridScroll.Y = max(0, min(pv.gridScroll.Y+dy, pv.gridScroll.MaxY))
	v.refresh()
}

// tabAt returns the page chip at a point of the window, if there is one.
func (v *view) tabAt(x, y float32) string {
	for guid, r := range v.tabRects {
		if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return guid
		}
	}
	return ""
}

// switchPage shows the page delta places after the active one, wrapping
// around.
func (v *view) switchPage(delta int) {
	tabs := v.app.store.Categories
	if len(tabs) < 2 {
		return
	}
	at := 0
	for i := range tabs {
		if tabs[i].GUID == v.prefs.ActiveTab {
			at = i
			break
		}
	}
	at = (at + delta%len(tabs) + len(tabs)) % len(tabs)
	v.centerNav = true
	v.setActiveTab(tabs[at].GUID)
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

// tabMenu is a tab's menu, opened by a right click on its chip.
func (v *view) tabMenu(guid, name string) []menuEntry {
	msgs := v.msgs()
	// The last page stays: it is the window's only place for its items.
	removable := len(v.app.store.Categories) > 1
	entries := []menuEntry{
		{icon: iconRename, label: msgs.Rename(), action: func() {
			v.later(func() { v.openTabRename(guid, name) })
		}},
		{icon: iconCopy, label: msgs.Duplicate(), action: func() {
			v.later(func() { v.duplicateTab(guid) })
		}},
	}
	if removable {
		entries = append(entries,
			menuEntry{},
			menuEntry{icon: iconTrash, label: msgs.Remove(), action: func() {
				v.later(func() { v.requestDeleteTab(guid, name) })
			}},
		)
	}
	return entries
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
