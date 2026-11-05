package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// listMargin is the page's room around the rows, and the larger trailing room
// it keeps for the add button, so the last row stays clear of it while the
// rest scrolls under it.
var listMargin = [4]float32{12, 10, 68, 10}

// listBody shows the tab as rows: one per item, as MyGo Music lists tracks. As
// in the grid, the page's padding belongs to the scrolling content, so that the
// rows scroll under the add button while its own bottom keeps the last one
// clear.
func (v *view) listBody(c *ui.Context, tab *CategoryNode) {
	rows := v.rows(tab)
	if len(rows) == 0 {
		v.emptyState(c, tab)
		return
	}
	pv := v.pageState(tab)
	ui.Scroll(c).Grow(1).TrackScroll(&pv.listScroll).Children(func() {
		ui.Column(c).Padding(listMargin[:]...).Role(ui.RoleList).Children(func() {
			for _, ref := range rows {
				v.listRow(c, tab, ref)
			}
		})
	})
}

// listRow is one item in the list.
func (v *view) listRow(c *ui.Context, tab *CategoryNode, ref rowRef) {
	item, slot := ref.item, ref.slot
	st := v.states[item.GUID]
	p := v.pal
	m := v.msgs()
	metrics := v.icons()

	row := ui.Row(c).Height(metrics.rowHeight).Padding(0, 6).Gap(10).Radius(8).
		AlignItems(ui.Center).Role(ui.RoleListItem)
	switch {
	case v.isLocated(item.GUID):
		row.Background(p.accent.Alpha(0.12))
	case st.Running:
		row.Background(p.accent.Alpha(0.07))
	case row.Hovered():
		row.Background(p.hover)
	}

	// Dragging, and taking what is dragged: a row reorders, and takes items
	// from another spot (or copies with Cmd/Ctrl held).
	row.Drag(dragItem{guid: item.GUID, slot: slot, tab: tab.GUID})
	// The line shows where the drop lands: above the row when the pointer
	// aims before it, below it when after.
	if _, over := ui.DragOver[dragItem](row); over {
		insertLine(row, false, dropAfter(row, false), p)
	}
	if dragged, ok := ui.Drop[dragItem](row); ok {
		copyItem := c.Modifiers()&ui.Cmd != 0
		at := slot
		// The frame that sees the drop no longer reports the drag over the
		// row, but still knows where the pointer let go.
		if dropAfter(row, false) {
			at++
		}
		v.dropDragged(dragged, tab, at, copyItem)
	}
	if row.Dragging() {
		row.Opacity(0.4)
		v.itemDraggingNow = true
	}

	if row.DoubleClicked() {
		v.runItem(item.GUID)
	}
	row.PopoverMenu(v.itemMenu(item, slot))

	row.Children(func() {
		v.itemIcon(c, item, metrics.listIcon)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, item.Name).FontSize(metrics.listFont).FontWeight(500).TextColor(p.text).
					SingleLine().Shrink(1).MinWidth(0)
				if item.RunAsAdmin {
					ui.Icon(c, iconShield).Size(13, 13).TextColor(c.Theme().Warning).Tooltip(m.RunsAsAdmin())
				}
			})
			if item.Path != "" {
				ui.Text(c, item.Path).FontSize(max(metrics.listFont-3, 10)).TextColor(p.muted).SingleLine().Shrink(1).MinWidth(0)
			}
		})
		v.runtimeField(c, item, st)
		v.runButton(c, item, st)

		// Files land above or below the row, as its two halves take them.
		top := ui.Box(c).Key("file-before").Absolute().Top(0).Left(0).Right(0).HeightPercent(50)
		bottom := ui.Box(c).Key("file-after").Absolute().Bottom(0).Left(0).Right(0).HeightPercent(50)
		v.fileDropHalf(row, top, tab.GUID, slot, false, false, p)
		v.fileDropHalf(row, bottom, tab.GUID, slot+1, false, true, p)
	})
}

// runButton runs an item, or stops the process it started.
func (v *view) runButton(c *ui.Context, item *AppItem, st ItemState) {
	p := v.pal
	m := v.msgs()
	label, icon := m.Run(), iconPlay
	if st.Running {
		label, icon = m.Stop(), iconStop
	}
	b := ui.ButtonBase(c).Height(26).PaddingX(10).Gap(5).Radius(999).AlignItems(ui.Center).
		Background(p.hover).TextColor(p.text).Label(label).FocusRing(false)
	if b.Hovered() {
		b.Background(p.press)
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(12, 12)
		ui.Text(c, label).FontSize(12.5).FontWeight(600).SingleLine()
	})
	if b.Clicked() {
		if st.Running {
			v.stopItem(item.GUID)
		} else {
			v.runItem(item.GUID)
		}
	}
}

// runtimeField shows an item's accumulated time, with what a running process is
// adding. Clicking it edits the runtime. It returns the field, which the grid
// places in a cell's corner.
func (v *view) runtimeField(c *ui.Context, item *AppItem, st ItemState) ui.Element {
	p := v.pal
	guid := item.GUID
	color := p.muted
	// A running process locks the runtime only while its clock is moving.
	timed := st.Running && v.app.store.Settings.AutoTime

	m := v.msgs()
	hint := m.RuntimeEditHint()
	switch {
	case timed:
		hint = m.RuntimeRunningHint()
	case st.Running:
		hint = m.RuntimeRunningNoAutoHint()
	}
	field := ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(2, 5).Radius(5)
	field.Tooltip(hint)
	if !timed {
		field.Cursor(ui.CursorPointer)
	}
	if field.Hovered() && !timed {
		field.Background(p.hover)
	}
	field.OnClick(func() {
		if !timed {
			v.openRuntime(guid)
		}
	})
	field.Children(func() {
		ui.Icon(c, iconClock).Size(11, 11).TextColor(color)
		ui.Text(c, v.runtimeText(guid)).FontSize(11.5).FontFeatures("tnum").TextColor(color)
	})
	return field
}

// itemIcon shows an item's cached icon, or its initial when it has none.
func (v *view) itemIcon(c *ui.Context, item *AppItem, size float32) {
	p := v.pal
	if src := v.app.iconImage(item); src != nil {
		ui.Image(c, src).Size(size, size).Fit(ui.Contain)
		return
	}
	box := ui.Box(c).Size(size, size).Radius(size / 4).Background(p.hover).Center()
	box.Children(func() {
		ui.Text(c, initial(item.Name)).FontSize(size * 0.42).FontWeight(600).TextColor(p.muted)
	})
}

func initial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}

// isLocated reports whether a search result brought this item into view a
// moment ago.
func (v *view) isLocated(guid string) bool {
	return guid != "" && guid == v.locate && time.Now().Before(v.locateTill)
}
