package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// listBody shows the tab as rows: one per item, as the page's list view did.
func (v *view) listBody(c *ui.Context, tab *CategoryNode) {
	rows := v.rows(tab)
	v.list.Key = func(i int) any { return rows[i].item.GUID }
	v.list.Label = func(i int) string { return rows[i].item.Name }
	ui.List(c, &v.list, len(rows), func(i int) {
		v.listRow(c, rows[i])
	}).Grow(1).Padding(6).Children(func() {
		if len(rows) == 0 {
			v.emptyState(c, tab)
		}
	})
}

// emptyState says how to fill an empty tab.
func (v *view) emptyState(c *ui.Context, tab *CategoryNode) {
	t := c.Theme()
	ui.Column(c).Padding(24).Gap(4).Center().Children(func() {
		ui.Text(c, "No files added yet.").TextColor(t.TextMuted)
		ui.Text(c, "Click + to pick files or create an item, or drop files here.").
			FontSize(11).TextColor(t.TextMuted)
		if tab != nil {
			ui.Text(c, "tab: "+tab.Name).FontSize(11).TextColor(t.TextMuted)
		}
	})
}

// listRow is one item in the list.
func (v *view) listRow(c *ui.Context, ref rowRef) {
	item, slot := ref.item, ref.slot
	st := v.states[item.GUID]
	t := c.Theme()

	row := ui.Row(c).Padding(5, 8).Gap(10).Radius(6).AlignItems(ui.Center)
	row.Cursor(ui.CursorPointer)
	switch {
	case v.isLocated(item.GUID):
		row.Background(t.Accent.Alpha(0.18))
	case st.Running:
		row.Background(t.Danger.Alpha(0.10))
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}

	// Dragging, and taking what is dragged: a row reorders, and takes items
	// from another spot (or copies with Cmd/Ctrl held).
	row.Drag(dragItem{guid: item.GUID, slot: slot, tab: v.prefs.ActiveTab})
	if dragged, ok := ui.Drop[dragItem](row); ok {
		copyItem := c.Modifiers()&ui.Cmd != 0
		v.later(func() { v.reorderSlots(dragged.slot, slot, copyItem) })
	}
	if _, over := ui.DragOver[dragItem](row); over {
		row.BorderWidth(2, 0, 0, 0).BorderColor(t.Accent)
	}
	if row.Dragging() {
		row.Opacity(0.4)
	}

	if row.DoubleClicked() {
		v.runItem(item.GUID)
	}
	row.PopoverMenu(v.itemMenu(item, slot))

	row.Children(func() {
		v.itemIcon(c, item, 26)
		ui.Text(c, item.Name).SingleLine().Grow(1)
		if item.RunAsAdmin {
			ui.Icon(c, iconShield).FontSize(13).TextColor(t.Warning).Tooltip("Runs as administrator")
		}
		v.runtimeField(c, item, st)
		if st.Running {
			ui.Button(c, "Stop").OnClick(func() { v.stopItem(item.GUID) })
		} else {
			ui.Button(c, "Run").OnClick(func() { v.runItem(item.GUID) })
		}
		more := ui.Button(c, "").Label("Item menu").Tooltip("Item menu").Padding(3).
			PopoverMenuButton(v.itemMenu(item, slot))
		more.Children(func() { ui.Icon(c, iconDots).FontSize(14).TextColor(t.TextMuted) })
	})
}

// runtimeField shows an item's accumulated time, and the time a manual or auto
// timer is adding. Clicking it edits the runtime, or stops a running timer. It
// returns the field, which the grid places in a cell's corner.
func (v *view) runtimeField(c *ui.Context, item *AppItem, st ItemState) ui.Element {
	t := c.Theme()
	guid := item.GUID
	active := v.timerActive(guid)
	color := t.TextMuted
	icon := iconClock
	if v.prefs.AutoTimers[guid] {
		icon = iconTimer
	}
	if active {
		color = t.Danger
	}

	hint := "Click to edit runtime"
	switch {
	case active:
		hint = "Click to stop the timer and save"
	case st.Running:
		hint = "Running — stop the program to edit its runtime"
	}
	field := ui.Row(c).Gap(3).AlignItems(ui.Center).Padding(1, 3).Radius(4)
	field.Tooltip(hint)
	if active || !st.Running {
		field.Cursor(ui.CursorPointer)
	}
	if field.Hovered() && !active && !st.Running {
		field.Background(t.SurfaceHover)
	}
	field.OnClick(func() {
		switch {
		case active:
			v.stopTimer(guid)
		case st.Running:
			// Nothing to do: the process reports its own time.
		default:
			v.openRuntime(guid)
		}
	})
	field.Children(func() {
		ui.Icon(c, icon).FontSize(11).TextColor(color)
		ui.Text(c, v.runtimeText(guid)).FontSize(11).TextColor(color)
	})
	return field
}

// itemIcon shows an item's cached icon, or its initial when it has none.
func (v *view) itemIcon(c *ui.Context, item *AppItem, size float32) {
	t := c.Theme()
	if src := v.app.iconImage(item); src != nil {
		ui.Image(c, src).Size(size, size).Fit(ui.Contain)
		return
	}
	box := ui.Box(c).Size(size, size).Radius(4).Background(t.SurfaceHover).Center()
	box.Children(func() {
		ui.Text(c, initial(item.Name)).FontSize(size * 0.5).Bold().TextColor(t.TextMuted)
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
