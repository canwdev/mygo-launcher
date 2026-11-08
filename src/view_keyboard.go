package main

import "github.com/egoist/mygo/ui"

// itemKeyKeys are the keys that choose a slot of the page shown, by the
// slot's index in QWERTY order: Q is the first, W the second, and so on.
// Blanks keep their place in the sequence.
var itemKeyKeys = []ui.Key{
	ui.KeyQ, ui.KeyW, ui.KeyE, ui.KeyR, ui.KeyT, ui.KeyY, ui.KeyU, ui.KeyI, ui.KeyO, ui.KeyP,
	ui.KeyA, ui.KeyS, ui.KeyD, ui.KeyF, ui.KeyG, ui.KeyH, ui.KeyJ, ui.KeyK, ui.KeyL,
	ui.KeyZ, ui.KeyX, ui.KeyC, ui.KeyV, ui.KeyB, ui.KeyN, ui.KeyM,
}

// overlayOpen reports whether something floats over the page: a menu, a panel
// or a dialog takes the keys itself, and the page's keyboard must wait.
func (v *view) overlayOpen() bool {
	if v.editOpen || v.runtimeOpen || v.renameOpen || v.confirmOpen ||
		v.searchOpen || v.pickerOpen || v.folderGUID != "" {
		return true
	}
	for _, open := range v.menus {
		if open != nil && *open {
			return true
		}
	}
	return false
}

// keyboard binds the window's keys while the page shows alone: the arrows and
// Enter choose and run items, the page keys turn pages, and Escape puts the
// window away. A floating layer leaves them unbound, so its own keys win.
func (v *view) keyboard(c *ui.Context) {
	// The window's own commands work over a menu too; a dialog keeps them
	// out, as OnWindowShortcut does by itself.
	c.OnWindowShortcut(ui.Cmd, ui.KeyF, v.openSearch)
	c.OnWindowShortcut(ui.Cmd, ui.KeyComma, v.openSettings)
	if v.overlayOpen() {
		return
	}
	c.OnWindowShortcut(0, ui.KeyEscape, v.escapeMain)
	c.OnWindowShortcut(0, ui.KeyEnter, v.activateSelected)
	c.OnWindowShortcut(0, ui.KeyLeft, func() { v.moveSelection(0, -1) })
	c.OnWindowShortcut(0, ui.KeyRight, func() { v.moveSelection(0, 1) })
	c.OnWindowShortcut(0, ui.KeyUp, func() { v.moveSelection(1, -1) })
	c.OnWindowShortcut(0, ui.KeyDown, func() { v.moveSelection(1, 1) })
	c.OnWindowShortcut(0, ui.KeyPageUp, func() { v.switchPageKey(-1) })
	c.OnWindowShortcut(0, ui.KeyPageDown, func() { v.switchPageKey(1) })
	c.OnWindowShortcut(0, ui.KeyTab, func() { v.switchPageKey(1) })
	c.OnWindowShortcut(0, ui.KeyHome, func() { v.gotoPage(0) })
	c.OnWindowShortcut(0, ui.KeyEnd, func() { v.gotoPage(-1) })

	for i, key := range []ui.Key{
		ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5,
		ui.Key6, ui.Key7, ui.Key8, ui.Key9,
	} {
		page := i
		c.OnWindowShortcut(0, key, func() { v.gotoPage(page) })
	}

	tab := v.tab()
	if tab == nil {
		return
	}
	for i := 0; i < len(tab.Slots) && i < len(itemKeyKeys); i++ {
		index := i
		c.OnWindowShortcut(0, itemKeyKeys[i], func() { v.selectSlot(index) })
	}
}

// escapeMain puts the window away as closing it would: in the tray with the
// background setting on, minimized otherwise.
func (v *view) escapeMain() {
	if v.overlayOpen() {
		return
	}
	if v.app.keepInBackground() {
		v.app.hideWindow()
	} else {
		v.app.minimize()
	}
}

// gotoPage shows the page at index; -1 is the last. The page's first item is
// chosen, as a page turned by the keyboard should have somewhere to start.
func (v *view) gotoPage(index int) {
	v.keyboardNav = true
	tabs := v.app.store.Categories
	if index < 0 {
		index = len(tabs) - 1
	}
	if index < 0 || index >= len(tabs) {
		return
	}
	// The pointer resting on a chip must not turn the page back after its
	// hover delay; it does so again only after moving to another chip.
	if v.hoveredTab != "" {
		v.hoverIgnoredTab = v.hoveredTab
	}
	v.centerNav = true
	v.setActiveTab(tabs[index].GUID)
	v.selectFirstItem()
	v.refresh()
}

// switchPageKey turns the page by delta, wrapping around.
func (v *view) switchPageKey(delta int) {
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
	v.gotoPage((at + delta%len(tabs) + len(tabs)) % len(tabs))
}

// selectedSlot is the slot the keyboard chose on tab, or -1 when it chose none
// or the slot no longer holds an item.
func (v *view) selectedSlot(tab *CategoryNode) int {
	if tab == nil || v.keySelTab != tab.GUID || v.keySel < 0 || v.keySel >= len(tab.Slots) {
		return -1
	}
	if tab.Slots[v.keySel] == nil {
		return -1
	}
	return v.keySel
}

// selectSlot chooses a slot by its index, blanks included: a blank keeps the
// index but has nothing to choose.
func (v *view) selectSlot(index int) {
	v.keyboardNav = true
	tab := v.tab()
	if tab == nil || index < 0 || index >= len(tab.Slots) || tab.Slots[index] == nil {
		return
	}
	v.setSelection(tab, index)
}

// selectFirstItem chooses the first item of the page shown, if it has one.
func (v *view) selectFirstItem() {
	tab := v.tab()
	if tab == nil {
		v.setSelection(nil, -1)
		return
	}
	v.setSelection(tab, firstItemSlot(tab))
}

func (v *view) setSelection(tab *CategoryNode, index int) {
	if tab == nil || index < 0 || index >= len(tab.Slots) || tab.Slots[index] == nil {
		v.keySelTab, v.keySel = "", -1
	} else {
		v.keySelTab, v.keySel = tab.GUID, index
		v.keySelReveal = true
	}
	v.refresh()
}

// moveSelection moves the chosen item: axis 0 turns left or right, axis 1 up
// or down. Leaving the first or last row, or the first or last item, turns the
// page instead.
func (v *view) moveSelection(axis, dir int) {
	v.keyboardNav = true
	tab := v.tab()
	if tab == nil || len(tab.Slots) == 0 {
		return
	}
	cur := v.selectedSlot(tab)
	if cur < 0 {
		if index := firstItemSlot(tab); index >= 0 {
			v.setSelection(tab, index)
		}
		return
	}
	cols := max(1, v.pageState(tab).gridCols)

	if axis == 0 {
		if dir < 0 {
			if cur == firstItemSlot(tab) {
				v.switchPageKey(-1)
				return
			}
			if next := prevItemSlot(tab, cur); next >= 0 {
				v.setSelection(tab, next)
			}
			return
		}
		if cur == lastItemSlot(tab) {
			v.switchPageKey(1)
			return
		}
		if next := nextItemSlot(tab, cur); next >= 0 {
			v.setSelection(tab, next)
		}
		return
	}

	lastRow := (len(tab.Slots) - 1) / cols
	row := cur / cols
	if dir < 0 && row == 0 || dir > 0 && row == lastRow {
		v.switchPageKey(dir)
		return
	}
	for r := row + dir; r >= 0 && r <= lastRow; r += dir {
		if index := nearestItemInRow(tab, r, cur%cols, cols); index >= 0 {
			v.setSelection(tab, index)
			return
		}
	}
	v.switchPageKey(dir)
}

// firstItemSlot and lastItemSlot are the first and last slots holding an item.
func firstItemSlot(tab *CategoryNode) int {
	for i, slot := range tab.Slots {
		if slot != nil {
			return i
		}
	}
	return -1
}

func lastItemSlot(tab *CategoryNode) int {
	for i := len(tab.Slots) - 1; i >= 0; i-- {
		if tab.Slots[i] != nil {
			return i
		}
	}
	return -1
}

// nextItemSlot and prevItemSlot are the neighboring items in row order.
func nextItemSlot(tab *CategoryNode, from int) int {
	for i := from + 1; i < len(tab.Slots); i++ {
		if tab.Slots[i] != nil {
			return i
		}
	}
	return -1
}

func prevItemSlot(tab *CategoryNode, from int) int {
	for i := from - 1; i >= 0; i-- {
		if tab.Slots[i] != nil {
			return i
		}
	}
	return -1
}

// nearestItemInRow is the item of row closest to column col, or -1 when the
// row holds none. Rows past the last slot have no items.
func nearestItemInRow(tab *CategoryNode, row, col, cols int) int {
	start := row * cols
	end := min(start+cols, len(tab.Slots))
	if start >= end {
		return -1
	}
	for d := 0; d < cols; d++ {
		for _, c := range [2]int{col - d, col + d} {
			if c < 0 || c >= cols || start+c >= end {
				continue
			}
			if tab.Slots[start+c] != nil {
				return start + c
			}
		}
	}
	return -1
}

// activateSelected runs the chosen item as a click would.
func (v *view) activateSelected() {
	tab := v.tab()
	index := v.selectedSlot(tab)
	if index < 0 {
		return
	}
	item := v.app.item(*tab.Slots[index])
	if item == nil {
		return
	}
	if st := v.states[item.GUID]; st.Running {
		name := item.Name
		v.ask(v.msgs().StopConfirm(name), func() { v.stopItem(item.GUID) })
		return
	}
	if dir := folderOfItem(item); dir != "" {
		v.openFolderMenu(item.GUID, dir, v.keySelX, v.keySelY)
		return
	}
	v.runItem(item.GUID)
}
