package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// gridMargin is the small room the page keeps around the grid, and the larger
// trailing room it keeps for the add button, so the last row stays clear of it
// while the rest scrolls under it.
var gridMargin = [4]float32{8, 8, 68, 8}

// gridBody shows the tab as a grid of cells, empty ones included: they are what
// gives the tab its arrangement. The cells pack together with no gap, and the
// page's padding belongs to the grid itself, so that the content scrolls under
// the add button instead of stopping above it.
func (v *view) gridBody(c *ui.Context, tab *CategoryNode) {
	slots := tab.Slots
	if len(slots) == 0 {
		v.emptyState(c, tab)
		return
	}
	m := v.icons()
	pv := v.pageState(tab)
	width, _ := c.Size()
	room := width - gridMargin[1] - gridMargin[3]
	pv.gridCols = max(1, int(math.Floor(float64(room/m.cellWidth))))
	ui.Scroll(c).Grow(1).TrackScroll(&pv.gridScroll).Children(func() {
		ui.Grid(c).Columns(pv.gridCols).Gap(0).Padding(gridMargin[:]...).Role(ui.RoleList).Children(func() {
			for i := range slots {
				v.gridCell(c, tab, i, slots[i], m)
			}
		})
	})
}

// gridCell builds one cell: an item card, or the empty cell it may be. The card
// draws no background of its own; the pointer shows one, as the states do.
func (v *view) gridCell(c *ui.Context, tab *CategoryNode, index int, guid *string, m iconMetrics) {
	if guid == nil {
		v.emptyCell(c, index, m)
		return
	}
	item := v.app.item(*guid)
	if item == nil {
		v.emptyCell(c, index, m)
		return
	}
	st := v.states[item.GUID]
	p := v.pal

	card := ui.Column(c).Height(m.cellHeight).Radius(8).Gap(6).
		AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleListItem)
	card.Cursor(ui.CursorPointer)
	switch {
	case v.isLocated(item.GUID):
		card.Background(p.accent.Alpha(0.12))
	case st.Running:
		card.Background(p.accent.Alpha(0.07))
	case card.Hovered():
		card.Background(p.hover)
	}

	card.Drag(dragItem{guid: item.GUID, slot: index, tab: tab.GUID})
	// The line shows where the drop lands: at the left edge of the cell when
	// the pointer aims before it, at the right edge when after it.
	if _, over := ui.DragOver[dragItem](card); over {
		insertLine(card, true, dropAfter(card, true), p)
	}
	if dragged, ok := ui.Drop[dragItem](card); ok {
		copyItem := c.Modifiers()&ui.Cmd != 0
		at := index
		// The frame that sees the drop no longer reports the drag over the
		// cell, but still knows where the pointer let go.
		if dropAfter(card, true) {
			at++
		}
		v.dropDragged(dragged, tab, at, copyItem)
	}
	if card.Dragging() {
		card.Opacity(0.4)
		v.itemDraggingNow = true
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
		// The runtime shows in a cell's corner only while the pointer is over
		// it, or while the process behind it runs. It is built anyway, hidden:
		// a child that comes and goes would move the drop halves' IDs with it,
		// and a native drop names the element it lands on by that ID.
		v.runtimeField(c, item, st).Absolute().Top(2).Left(4).
			Hidden(!st.Running && !card.Hovered())

		v.itemIcon(c, item, m.gridIcon)
		ui.Row(c).Gap(4).AlignItems(ui.Center).MaxWidth(m.cellWidth - 8).Children(func() {
			if item.RunAsAdmin {
				ui.Icon(c, iconShield).Size(11, 11).TextColor(c.Theme().Warning)
			}
			ui.Text(c, item.Name).FontSize(m.gridFont).FontWeight(500).SingleLine().TextColor(p.text)
		})

		// Files land before or after the cell, as its two halves take them.
		left := ui.Box(c).Key("file-before").Absolute().Top(0).Bottom(0).Left(0).WidthPercent(50)
		right := ui.Box(c).Key("file-after").Absolute().Top(0).Bottom(0).Right(0).WidthPercent(50)
		v.fileDropHalf(card, left, tab.GUID, index, true, false, p)
		v.fileDropHalf(card, right, tab.GUID, index+1, true, true, p)
	})
}

// emptyCell is a cell holding nothing: it takes a dropped item, and its menu
// adds or removes empty cells.
func (v *view) emptyCell(c *ui.Context, index int, m iconMetrics) {
	p := v.pal
	box := ui.Box(c).Height(m.cellHeight).Radius(8)
	if box.Hovered() {
		box.Background(p.hover)
	}
	if _, over := ui.DragOver[dragItem](box); over {
		insertLine(box, true, dropAfter(box, true), p)
	}
	if dragged, ok := ui.Drop[dragItem](box); ok {
		at := index
		if dropAfter(box, true) {
			at++
		}
		v.dropDragged(dragged, v.tab(), at, false)
	}
	box.Children(func() {
		left := ui.Box(c).Key("file-before").Absolute().Top(0).Bottom(0).Left(0).WidthPercent(50)
		right := ui.Box(c).Key("file-after").Absolute().Top(0).Bottom(0).Right(0).WidthPercent(50)
		v.fileDropHalf(box, left, v.tabGUID(), index, true, false, p)
		v.fileDropHalf(box, right, v.tabGUID(), index+1, true, true, p)
	})
	box.PopoverMenu(func(menu *ui.Menu) {
		msgs := v.msgs()
		if menu.Item(msgs.InsertEmpty()).Chosen() {
			v.later(func() { v.insertEmptyAt(index) })
		}
		if menu.Item(msgs.RemoveEmpty()).Chosen() {
			v.later(func() { v.deleteSlotAt(index) })
		}
	})
}
