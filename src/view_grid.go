package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// The grid's cells: at least this wide, and this high, as the page's
// auto-filling grid of 100px squares was.
const (
	gridCellWidth  float32 = 116
	gridCellHeight float32 = 128
)

// gridBody shows the tab as a grid of cells, empty ones included: they are what
// gives the tab its arrangement.
func (v *view) gridBody(c *ui.Context, tab *CategoryNode) {
	slots := tab.Slots
	if len(slots) == 0 {
		v.emptyState(c, tab)
		return
	}
	v.grid.Key = func(i int) any {
		if slots[i] != nil {
			return *slots[i]
		}
		return fmt.Sprintf("slot-%d", i)
	}
	v.grid.Label = func(i int) string {
		if slots[i] != nil {
			if item := v.app.item(*slots[i]); item != nil {
				return item.Name
			}
		}
		return v.msgs().EmptyCell()
	}
	ui.GridView(c, &v.grid, len(slots), gridCellWidth, gridCellHeight, func(i int) {
		v.gridCell(c, tab, i, slots[i])
	}).Grow(1).Padding(16, 16, 88, 16)
}

// gridCell builds one cell: an item card, or the empty cell it may be.
func (v *view) gridCell(c *ui.Context, tab *CategoryNode, index int, guid *string) {
	if guid == nil {
		v.emptyCell(c, index)
		return
	}
	item := v.app.item(*guid)
	if item == nil {
		v.emptyCell(c, index)
		return
	}
	st := v.states[item.GUID]
	p := v.pal

	card := ui.Column(c).Fill().Radius(12).Padding(8).Gap(8).AlignItems(ui.Center).Justify(ui.Center)
	card.Cursor(ui.CursorPointer)
	switch {
	case v.isLocated(item.GUID):
		card.Background(p.accent.Alpha(0.12))
	case st.Running:
		card.Background(p.accent.Alpha(0.07))
	case card.Hovered():
		card.Background(p.press)
	default:
		card.Background(p.hover)
	}

	card.Drag(dragItem{guid: item.GUID, slot: index, tab: tab.GUID})
	if dragged, ok := ui.Drop[dragItem](card); ok {
		copyItem := c.Modifiers()&ui.Cmd != 0
		v.later(func() { v.reorderSlots(dragged.slot, index, copyItem) })
	}
	if _, over := ui.DragOver[dragItem](card); over {
		card.BorderWidth(0, 0, 0, 2).BorderColor(p.accent)
	}
	if card.Dragging() {
		card.Opacity(0.4)
	}

	if card.Clicked() {
		// A click runs the item; a running one asks before it stops.
		if st.Running {
			name := item.Name
			v.ask(v.msgs().StopConfirm(name), func() { v.stopItem(item.GUID) })
		} else {
			v.runItem(item.GUID)
		}
	}
	card.PopoverMenu(v.itemMenu(item, index))

	card.Children(func() {
		v.runtimeField(c, item, st).Absolute().Top(2).Left(4).MaxWidth(gridCellWidth - 30)

		v.itemIcon(c, item, 54)
		ui.Row(c).Gap(4).AlignItems(ui.Center).MaxWidth(gridCellWidth - 12).Children(func() {
			if item.RunAsAdmin {
				ui.Icon(c, iconShield).Size(11, 11).TextColor(c.Theme().Warning)
			}
			ui.Text(c, item.Name).FontSize(12.5).FontWeight(500).SingleLine().TextColor(p.text)
		})
	})
}

// emptyCell is a cell holding nothing: it takes a dropped item, and its menu
// adds or removes empty cells.
func (v *view) emptyCell(c *ui.Context, index int) {
	p := v.pal
	box := ui.Box(c).Fill().Radius(12).Border(1, p.hairline).BorderStyle(ui.BorderDashed)
	if _, over := ui.DragOver[dragItem](box); over {
		box.Border(2, p.accent)
	}
	if dragged, ok := ui.Drop[dragItem](box); ok {
		v.later(func() { v.reorderSlots(dragged.slot, index, false) })
	}
	box.PopoverMenu(func(menu *ui.Menu) {
		m := v.msgs()
		if menu.Item(m.InsertEmpty()).Chosen() {
			v.later(func() { v.insertEmptyAt(index) })
		}
		if menu.Item(m.RemoveEmpty()).Chosen() {
			v.later(func() { v.deleteSlotAt(index) })
		}
	})
	box.Children(func() {
		if box.Hovered() {
			ui.Icon(c, iconPlus).Size(16, 16).TextColor(p.faint)
		}
	})
}
