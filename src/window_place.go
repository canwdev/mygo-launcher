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

// placeWindowInCenter puts a window of width×height in the middle of work, the
// work area of the display the window belongs on. A window larger than it
// starts at its edge, as one placed at the pointer does.
func placeWindowInCenter(width, height int, work mygo.Rectangle) (x, y int) {
	if work.Width <= 0 || work.Height <= 0 {
		return -width / 2, -height / 2
	}
	x, y = work.X+(work.Width-width)/2, work.Y+(work.Height-height)/2
	return clampAxis(x, width, work.X, work.Width), clampAxis(y, height, work.Y, work.Height)
}

// clampAxis keeps a span of size starting at pos inside the span at start.
func clampAxis(pos, size, start, span int) int {
	if size >= span {
		return start
	}
	return max(start, min(pos, start+span-size))
}

// windowPlace is where the main window shows whenever it is brought up.
type windowPlace string

const (
	// placeLast shows it where it was left: the geometry kept in
	// window-state.json.
	placeLast windowPlace = "last"
	// placeCursor shows it around the pointer.
	placeCursor windowPlace = "cursor"
	// placeCenter shows it in the middle of the display the pointer is on.
	placeCenter windowPlace = "center"
)

// startupPlace is where the window opens: --at-cursor asks for the pointer
// whatever the preference says, which otherwise picks between the saved
// position, the pointer and the middle of the display.
func (v *view) startupPlace(opts launchOptions) windowPlace {
	if opts.atCursor {
		return placeCursor
	}
	switch place := windowPlace(v.prefs.WindowPosition); place {
	case placeLast, placeCenter:
		return place
	}
	return placeCursor
}

// showMainAtPreference brings the main window up where the preference says:
// from the tray, from the global shortcut, or on the first frame of a window
// opened away from its saved position.
func (v *view) showMainAtPreference() {
	v.showMainAt(v.startupPlace(launchOptions{}))
}

// showMainAt brings the main window up at place, restoring it first: a window
// called for from anywhere should appear where the user is looking.
func (v *view) showMainAt(place windowPlace) {
	w := v.app.win
	if w == nil {
		return
	}
	if w.IsMaximized() {
		w.Unmaximize()
	}
	w.Restore()
	if place != placeLast {
		// The pointer's display is the one the window belongs on, both
		// centered on the pointer and in its middle.
		cursor := mygo.Screen.CursorScreenPoint()
		work := mygo.Screen.DisplayNearestPoint(cursor).WorkArea
		b := w.Bounds()
		var x, y int
		switch place {
		case placeCenter:
			x, y = placeWindowInCenter(b.Width, b.Height, work)
		default:
			x, y = placeWindowAtCursor(b.Width, b.Height, cursor, work)
		}
		w.SetPosition(x, y)
	}
	w.Show()
	w.Focus()
}
