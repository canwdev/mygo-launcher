package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// The grid's cells: at least this wide, and this high, as the page's
// auto-filling grid of 100px squares was.
const (
	gridCellWidth  float32 = 104
	gridCellHeight float32 = 116
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
		return "Empty"
	}
	ui.GridView(c, &v.grid, len(slots), gridCellWidth, gridCellHeight, func(i int) {
		v.gridCell(c, tab, i, slots[i])
	}).Grow(1).Padding(8)
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
	t := c.Theme()

	card := ui.Column(c).Fill().Radius(8).Padding(6).Gap(2).Center()
	card.Cursor(ui.CursorPointer)
	switch {
	case v.isLocated(item.GUID):
		card.Background(t.Accent.Alpha(0.18))
	case st.Running:
		card.Background(t.Danger.Alpha(0.12))
	case card.Hovered():
		card.Background(t.SurfaceHover)
	}

	card.Drag(dragItem{guid: item.GUID, slot: index, tab: tab.GUID})
	if dragged, ok := ui.Drop[dragItem](card); ok {
		copyItem := c.Modifiers()&ui.Cmd != 0
		v.later(func() { v.reorderSlots(dragged.slot, index, copyItem) })
	}
	if _, over := ui.DragOver[dragItem](card); over {
		card.BorderWidth(2, 0, 0, 0).BorderColor(t.Accent)
	}
	if card.Dragging() {
		card.Opacity(0.4)
	}

	if card.Clicked() {
		// A click runs the item; a running one asks before it stops.
		if st.Running {
			name := item.Name
			v.ask("Stop \""+name+"\"?", func() { v.stopItem(item.GUID) })
		} else {
			v.runItem(item.GUID)
		}
	}
	card.PopoverMenu(v.itemMenu(item, index))

	card.Children(func() {
		v.runtimeField(c, item, st).Absolute().Top(2).Left(4).MaxWidth(gridCellWidth - 30)
		more := ui.Button(c, "").Label("Item menu").Tooltip("Item menu").Padding(2).
			PopoverMenuButton(v.itemMenu(item, index))
		more.Absolute().Top(2).Right(2)
		more.Children(func() { ui.Icon(c, iconDots).FontSize(13).TextColor(t.TextMuted) })
		more.Hidden(!card.Hovered())
		more.Opacity(0.8)

		v.itemIcon(c, item, 42)
		ui.Row(c).Gap(3).AlignItems(ui.Center).MaxWidth(gridCellWidth - 12).Children(func() {
			if item.RunAsAdmin {
				ui.Icon(c, iconShield).FontSize(11).TextColor(t.Warning)
			}
			ui.Text(c, item.Name).FontSize(11).SingleLine().TextColor(t.Text)
		})
	})
}

// emptyCell is a cell holding nothing: it takes a dropped item, and its menu
// adds or removes empty cells.
func (v *view) emptyCell(c *ui.Context, index int) {
	t := c.Theme()
	box := ui.Box(c).Fill().Radius(8).Border(1, t.Border).BorderStyle(ui.BorderDashed)
	if _, over := ui.DragOver[dragItem](box); over {
		box.Border(2, t.Accent)
	}
	if dragged, ok := ui.Drop[dragItem](box); ok {
		v.later(func() { v.reorderSlots(dragged.slot, index, false) })
	}
	box.PopoverMenu(func(m *ui.Menu) {
		if m.Item("Insert empty").Chosen() {
			v.later(func() { v.insertEmptyAt(index) })
		}
		if m.Item("Delete empty").Chosen() {
			v.later(func() { v.deleteSlotAt(index) })
		}
	})
	box.Children(func() {
		if box.Hovered() {
			ui.Icon(c, iconPlus).FontSize(16).TextColor(t.TextMuted.Alpha(0.6))
		}
	})
}
