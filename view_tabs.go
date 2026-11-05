package main

import "github.com/egoist/mygo/ui"

// tabChip builds one tab of the strip: it takes the click that shows it, drags
// onto another place in the strip, takes items dragged onto it (moving them, or
// copying them with Cmd/Ctrl held), and opens its menu on a right click.
func (v *view) tabChip(c *ui.Context, index int) {
	tab := v.app.store.Categories[index]
	guid, name := tab.GUID, tab.Name
	selected := guid == v.prefs.ActiveTab
	t := c.Theme()

	chip := ui.Row(c).Key(guid).Padding(3, 10).Radius(6).Gap(4).AlignItems(ui.Center)
	chip.Cursor(ui.CursorPointer)
	if selected {
		chip.Background(t.Accent)
	} else if chip.Hovered() {
		chip.Background(t.SurfaceHover)
	}

	// An item dragged over the tab: dropping moves it there, Cmd/Ctrl copies.
	if _, over := ui.DragOver[dragItem](chip); over {
		chip.Border(2, t.Accent)
	}
	if dragged, ok := ui.Drop[dragItem](chip); ok {
		copyItem := c.Modifiers()&ui.Cmd != 0
		v.later(func() { v.moveItemToTab(dragged.guid, guid, copyItem) })
	}

	// Tabs themselves reorder by dragging.
	chip.Drag(dragTab{index: index})
	if dragged, ok := ui.Drop[dragTab](chip); ok && dragged.index != index {
		v.later(func() { v.moveTab(dragged.index, index) })
	}
	if chip.Dragging() {
		chip.Opacity(0.4)
	}

	if chip.Clicked() {
		v.setActiveTab(guid)
	}

	menu := func(m *ui.Menu) {
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
	chip.PopoverMenu(menu)

	label := t.Text
	if selected {
		label = t.AccentText
	}
	chip.Children(func() {
		ui.Text(c, name).SingleLine().TextColor(label).FontSize(t.FontSize)
		more := ui.Button(c, "").Label("Tab menu").Tooltip("Tab menu").Padding(1).
			PopoverMenuButton(menu)
		more.Children(func() {
			ui.Icon(c, iconDots).FontSize(12).TextColor(label)
		})
		more.Hidden(!selected && !chip.Hovered())
		more.Opacity(0.7)
	})
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
	hasContent := false
	for _, slot := range tab.Slots {
		if slot != nil {
			hasContent = true
			break
		}
	}
	if !hasContent {
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
