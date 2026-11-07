package main

import (
	"github.com/google/uuid"
)

// tab returns the tab the window shows, falling back to the first one.
func (v *view) tab() *CategoryNode {
	if len(v.app.store.Categories) == 0 {
		return nil
	}
	tab := v.app.tab(v.prefs.ActiveTab)
	if tab == nil {
		tab = &v.app.store.Categories[0]
	}
	if tab.GUID != v.prefs.ActiveTab {
		v.prefs.ActiveTab = tab.GUID
	}
	return tab
}

func (v *view) setActiveTab(guid string) {
	if v.prefs.ActiveTab == guid {
		return
	}
	v.prefs.ActiveTab = guid
	v.savePrefs()
	v.app.refreshVisibleSpecials()
}

// === Tabs ===

func (v *view) addTab() {
	cat := CategoryNode{GUID: uuid.NewString(), Name: v.msgs().NewTabName(), Slots: []*string{}}
	v.app.store.Categories = append(v.app.store.Categories, cat)
	v.prefs.ActiveTab = cat.GUID
	v.centerNav = true
	v.savePrefs()
	v.app.save()
}

func (v *view) removeTab(guid string) {
	idx := -1
	for i := range v.app.store.Categories {
		if v.app.store.Categories[i].GUID == guid {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	v.app.store.Categories = append(v.app.store.Categories[:idx], v.app.store.Categories[idx+1:]...)
	if v.prefs.ActiveTab == guid {
		next := ""
		if len(v.app.store.Categories) > 0 {
			next = v.app.store.Categories[min(idx, len(v.app.store.Categories)-1)].GUID
		}
		v.prefs.ActiveTab = next
		v.savePrefs()
	}
	v.pruneOrphans()
	v.app.pruneIcons()
	v.app.save()
}

func (v *view) renameTab(guid, name string) {
	for i := range v.app.store.Categories {
		if v.app.store.Categories[i].GUID == guid {
			v.app.store.Categories[i].Name = name
			v.app.save()
			return
		}
	}
}

// duplicateTab copies a tab's arrangement of items; the items themselves are
// shared, as the pool is.
func (v *view) duplicateTab(guid string) {
	for i := range v.app.store.Categories {
		src := v.app.store.Categories[i]
		if src.GUID != guid {
			continue
		}
		copyTab := CategoryNode{
			GUID:  uuid.NewString(),
			Name:  v.msgs().CopyName(src.Name),
			Slots: append([]*string{}, src.Slots...),
		}
		v.app.store.Categories = append(v.app.store.Categories, copyTab)
		v.prefs.ActiveTab = copyTab.GUID
		v.savePrefs()
		v.app.save()
		return
	}
}

func (v *view) moveTab(from, to int) {
	cats := v.app.store.Categories
	if from < 0 || from >= len(cats) || to < 0 || to >= len(cats) || from == to {
		return
	}
	tab := cats[from]
	cats = append(cats[:from], cats[from+1:]...)
	cats = append(cats[:to], append([]CategoryNode{tab}, cats[to:]...)...)
	v.app.store.Categories = cats
	v.app.save()
}

// === Items ===

// itemFields are the values of the item dialog.
type itemFields struct {
	Name       string
	Path       string
	Args       string
	WorkDir    string
	Icon       string
	RunAsAdmin bool
}

func (v *view) createItem(fields itemFields) {
	tab := v.tab()
	if tab == nil {
		v.addTab()
		tab = v.tab()
		if tab == nil {
			return
		}
	}
	name := fields.Name
	if name == "" {
		name = defaultTitle(fields.Path)
	}
	if name == "" {
		return
	}
	guid := uuid.NewString()
	v.app.store.Apps[guid] = &AppItem{
		GUID:       guid,
		Name:       name,
		Path:       fields.Path,
		Args:       fields.Args,
		WorkingDir: fields.WorkDir,
		Icon:       fields.Icon,
		RunAsAdmin: fields.RunAsAdmin,
	}
	v.app.runtimeStats[guid] = 0
	tab.Slots = append(tab.Slots, &guid)
	v.app.save()
	v.app.notify(v.msgs().ItemCreated())
}

func (v *view) updateItem(guid string, fields itemFields) {
	item := v.app.item(guid)
	if item == nil {
		return
	}
	if item.Path != fields.Path {
		item.Link = "" // the shortcut the item came from no longer describes it
	}
	oldIcon := item.Icon
	item.Name = fields.Name
	item.Path = fields.Path
	item.Args = fields.Args
	item.WorkingDir = fields.WorkDir
	item.Icon = fields.Icon
	item.RunAsAdmin = fields.RunAsAdmin
	v.app.save()
	if oldIcon != item.Icon {
		v.app.clearDynamicIcon(guid)
		v.app.pruneIcons() // the icon it replaced, if nothing else uses it
	}
	v.app.notify(v.msgs().ItemUpdated())
}

func (v *view) removeItem(guid string) {
	v.app.clearDynamicIcon(guid)
	for i := range v.app.store.Categories {
		kept := v.app.store.Categories[i].Slots[:0]
		for _, slot := range v.app.store.Categories[i].Slots {
			if slot != nil && *slot == guid {
				continue
			}
			kept = append(kept, slot)
		}
		v.app.store.Categories[i].Slots = kept
	}
	v.pruneOrphans()
	v.app.pruneIcons()
	v.app.save()
}

// duplicateItem copies an item, with its own runtime, after slot.
func (v *view) duplicateItem(guid string, tabGUID string, slot int) {
	item := v.app.item(guid)
	tab := v.app.tab(tabGUID)
	if item == nil || tab == nil {
		return
	}
	copyItem := *item
	copyItem.GUID = uuid.NewString()
	copyItem.Name = v.msgs().CopyName(item.Name)
	copyItem.RuntimeMs = 0
	v.app.store.Apps[copyItem.GUID] = &copyItem
	v.app.runtimeStats[copyItem.GUID] = 0
	newGUID := copyItem.GUID
	at := clamp(slot+1, 0, len(tab.Slots))
	tab.Slots = append(tab.Slots[:at], append([]*string{&newGUID}, tab.Slots[at:]...)...)
	v.app.save()
}

// moveItemToTab moves an item (or copies it) to the end of another tab.
func (v *view) moveItemToTab(guid, tabGUID string, copyItem bool) {
	tab := v.app.tab(tabGUID)
	item := v.app.item(guid)
	if tab == nil || item == nil {
		return
	}
	if copyItem {
		v.duplicateItem(guid, tabGUID, len(tab.Slots)-1)
		return
	}
	for i := range v.app.store.Categories {
		kept := v.app.store.Categories[i].Slots[:0]
		for _, slot := range v.app.store.Categories[i].Slots {
			if slot != nil && *slot == guid {
				continue
			}
			kept = append(kept, slot)
		}
		v.app.store.Categories[i].Slots = kept
	}
	tab.Slots = append(tab.Slots, &guid)
	v.app.save()
}

// moveItemToTabAt moves an item (or copies it) into a tab at slot at, as a
// drop between its items does.
func (v *view) moveItemToTabAt(guid, tabGUID string, at int, copyItem bool) {
	tab := v.app.tab(tabGUID)
	item := v.app.item(guid)
	if tab == nil || item == nil {
		return
	}
	if copyItem {
		v.duplicateItem(guid, tabGUID, at-1)
		return
	}
	for i := range v.app.store.Categories {
		kept := v.app.store.Categories[i].Slots[:0]
		for _, slot := range v.app.store.Categories[i].Slots {
			if slot != nil && *slot == guid {
				continue
			}
			kept = append(kept, slot)
		}
		v.app.store.Categories[i].Slots = kept
	}
	at = clamp(at, 0, len(tab.Slots))
	tab.Slots = append(tab.Slots[:at], append([]*string{&guid}, tab.Slots[at:]...)...)
	v.app.save()
}

// reorderSlots moves the value of slot from to slot to, or copies the item
// there.
func (v *view) reorderSlots(from, to int, copyItem bool) {
	tab := v.tab()
	if tab == nil {
		return
	}
	n := len(tab.Slots)
	if from < 0 || from >= n || to < 0 || to > n || from == to {
		return
	}
	src := tab.Slots[from]
	if copyItem {
		if src == nil {
			return // an empty cell copies nothing
		}
		item := v.app.item(*src)
		if item == nil {
			return
		}
		copyItem := *item
		copyItem.GUID = uuid.NewString()
		copyItem.Name = v.msgs().CopyName(item.Name)
		copyItem.RuntimeMs = 0
		v.app.store.Apps[copyItem.GUID] = &copyItem
		v.app.runtimeStats[copyItem.GUID] = 0
		guid := copyItem.GUID
		at := clamp(to, 0, len(tab.Slots))
		tab.Slots = append(tab.Slots[:at], append([]*string{&guid}, tab.Slots[at:]...)...)
		v.app.save()
		return
	}
	v.app.store.Categories[v.tabIndex(tab)].Slots = moveSlot(tab.Slots, from, to)
	v.app.save()
}

func (v *view) tabIndex(tab *CategoryNode) int {
	for i := range v.app.store.Categories {
		if &v.app.store.Categories[i] == tab {
			return i
		}
	}
	return -1
}

// moveSlot takes the value at from out, and puts it in before to.
func moveSlot(slots []*string, from, to int) []*string {
	moved := slots[from]
	rest := append(append([]*string{}, slots[:from]...), slots[from+1:]...)
	at := clamp(to, 0, len(rest))
	return append(rest[:at], append([]*string{moved}, rest[at:]...)...)
}

// insertEmptyAt puts an empty cell before slot index.
func (v *view) insertEmptyAt(index int) {
	tab := v.tab()
	if tab == nil {
		return
	}
	at := clamp(index, 0, len(tab.Slots))
	tab.Slots = append(tab.Slots[:at], append([]*string{nil}, tab.Slots[at:]...)...)
	v.app.save()
}

// deleteSlotAt removes an empty cell.
func (v *view) deleteSlotAt(index int) {
	tab := v.tab()
	if tab == nil || index < 0 || index >= len(tab.Slots) || tab.Slots[index] != nil {
		return
	}
	tab.Slots = append(tab.Slots[:index], tab.Slots[index+1:]...)
	v.app.save()
}

// pruneOrphans drops items no tab refers to any more.
func (v *view) pruneOrphans() {
	referenced := map[string]bool{}
	for _, cat := range v.app.store.Categories {
		for _, slot := range cat.Slots {
			if slot != nil {
				referenced[*slot] = true
			}
		}
	}
	for guid := range v.app.store.Apps {
		if !referenced[guid] {
			delete(v.app.store.Apps, guid)
			delete(v.app.runtimeStats, guid)
		}
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// dragItem is a value dragged from a row or a cell: the item and where it came
// from.
type dragItem struct {
	guid string
	slot int
	tab  string
}

// dropDragged puts a dropped item at slot at of tab: it reorders within the
// page it came from, or moves to another page, or copies either way.
func (v *view) dropDragged(dragged dragItem, tab *CategoryNode, at int, copyItem bool) {
	if tab == nil {
		return
	}
	if dragged.tab != tab.GUID {
		// Dragged from another page, which a drop between items brings here.
		v.later(func() { v.moveItemToTabAt(dragged.guid, tab.GUID, at, copyItem) })
		return
	}
	v.later(func() { v.reorderSlots(dragged.slot, dropTo(dragged.slot, at, copyItem), copyItem) })
}

// dragTab is a tab dragged to another place.
type dragTab struct {
	index int
}
