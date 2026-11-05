package main

import "github.com/egoist/mygo/ui"

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
	if dragged, ok := ui.Drop[dragTab](slot); ok && dragged.index != index {
		v.later(func() { v.moveTab(dragged.index, index) })
	}
	_, overTab := ui.DragOver[dragTab](slot)

	slot.Children(func() {
		b := ui.ButtonBase(c.Key(guid)).Height(30).PaddingX(12).Gap(5).Radius(8).
			AlignItems(ui.Center).FocusRing(false).Label(name)
		switch {
		case selected:
			b.Background(p.press)
		case b.Hovered():
			b.Background(p.hover)
		}
		if overTab {
			b.Border(1, p.accent)
		}

		// An item dragged over the tab: dropping moves it there, Cmd/Ctrl
		// copies.
		if _, over := ui.DragOver[dragItem](b); over {
			b.Border(2, p.accent)
		}
		if dragged, ok := ui.Drop[dragItem](b); ok {
			copyItem := c.Modifiers()&ui.Cmd != 0
			v.later(func() { v.moveItemToTab(dragged.guid, guid, copyItem) })
		}

		// Tabs themselves reorder by dragging.
		b.Drag(dragTab{index: index})
		if b.Dragging() {
			b.Opacity(0.4)
		}
		if b.Clicked() {
			v.setActiveTab(guid)
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
	return func(m *ui.Menu) {
		if m.Item("Rename").Chosen() {
			v.later(func() { v.openTabRename(guid, name) })
		}
		if m.Item("Duplicate").Chosen() {
			v.later(func() { v.duplicateTab(guid) })
		}
		m.Separator()
		if m.Item("Delete").Chosen() {
			v.later(func() { v.requestDeleteTab(guid, name) })
		}
	}
}

// openTabRename asks for a tab's new name.
func (v *view) openTabRename(guid, name string) {
	v.renameOpen = true
	v.renameTitle = "Rename Tab"
	v.renameName = name
	v.renameGUID = ""
	v.renameTabGUID = guid
}

// requestDeleteTab deletes an empty tab at once, and asks about one with items.
func (v *view) requestDeleteTab(guid, name string) {
	tab := v.app.tab(guid)
	if tab == nil {
		return
	}
	if tabItemCount(tab) == 0 {
		v.removeTab(guid)
		return
	}
	v.ask("Delete tab \""+name+"\"?", func() { v.removeTab(guid) })
}

// ask opens the confirmation alert, which runs action when it is accepted.
func (v *view) ask(message string, action func()) {
	v.confirmMsg = message
	v.confirmAction = action
	v.confirmOpen = true
}
