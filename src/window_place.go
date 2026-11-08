package main

import "github.com/egoist/mygo"

// placeWindowAtCursor puts a window of width×height around cursor, kept inside
// work. A window larger than the work area starts at its edge: there is no
// placement that fits it.
func placeWindowAtCursor(width, height int, cursor mygo.Point, work mygo.Rectangle) (x, y int) {
	x, y = cursor.X-width/2, cursor.Y-height/2
	if work.Width <= 0 || work.Height <= 0 {
		return x, y
	}
	return clampAxis(x, width, work.X, work.Width), clampAxis(y, height, work.Y, work.Height)
}

// clampAxis keeps a span of size starting at pos inside the span at start.
func clampAxis(pos, size, start, span int) int {
	if size >= span {
		return start
	}
	return max(start, min(pos, start+span-size))
}

// showMainAtCursor brings the main window up around the pointer, on the
// display it is on. The saved position is replaced: a window called for from
// anywhere should appear where the user is looking.
func (v *view) showMainAtCursor() {
	w := v.app.win
	if w == nil {
		return
	}
	if w.IsMaximized() {
		w.Unmaximize()
	}
	w.Restore()
	cursor := mygo.Screen.CursorScreenPoint()
	display := mygo.Screen.DisplayNearestPoint(cursor)
	b := w.Bounds()
	x, y := placeWindowAtCursor(b.Width, b.Height, cursor, display.WorkArea)
	w.SetPosition(x, y)
	w.Show()
	w.Focus()
}
