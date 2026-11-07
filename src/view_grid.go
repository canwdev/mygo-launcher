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
	// the pointer aims before it, at the right edge when after it. The cell
	// dragging draws no line: the copy following the pointer would carry it,
	// and a drop on the cell it came from moves nothing.
	dragging := card.Dragging()
	if _, over := ui.DragOver[dragItem](card); over && !dragging {
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
	if dragging {
		card.Opacity(0.4)
		v.itemDraggingNow = true
	}

	if card.Clicked() {
		// A click runs the item; a running one asks before it stops. A folder
		// lists its contents instead, in a panel centered on the pointer.
		dir := folderOfItem(item)
		switch {
		case st.Running:
			name := item.Name
			v.ask(v.msgs().StopConfirm(name), func() { v.stopItem(item.GUID) })
		case dir != "":
			x, y, _ := card.PointerPosition()
			atX, atY := card.LocalToWindow(x, y)
			v.openFolderMenu(item.GUID, dir, atX, atY)
		default:
			v.runItem(item.GUID)
		}
	}
	// The panel goes with the cell of the item it lists, which this frame built.
	if item.GUID == v.folderGUID {
		v.folderAnchored = true
	}
	v.contextMenu(c, card, itemMenuKey(tab.GUID, item.GUID, index), v.itemMenu(item, tab.GUID, index))

	card.Children(func() {
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

		// The runtime shows in a cell's corner only while the pointer is over
		// it, or while the process behind it runs, and only for an item that is
		// timed at all: a folder or a file has no runtime to show. It is built
		// after the drop halves, so a click on it reaches it, and always, hidden
		// or not, so the halves keep their places from frame to frame: a native
		// drop names the element it lands on by its ID.
		showRuntime := timeableItem(item) && v.app.store.Settings.AutoTime &&
			(st.Running || card.Hovered())
		runtime := v.runtimeField(c, item, st).Absolute().Top(2).Left(4).
			Hidden(!showRuntime)
		v.fileDropHalf(card, runtime, tab.GUID, index, true, false, p)
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
	v.contextMenu(c, box, emptyMenuKey(v.tabGUID(), index), v.emptyMenu(index))
}
