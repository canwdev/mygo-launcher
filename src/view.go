package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// view is the window's interface: a function of the app's state, rebuilt for
// every frame. Everything in it is main thread state, as the store is.
type view struct {
	app *App
	win *mygo.Window

	prefs uiPrefs

	// pal is the palette the frame draws with, from the theme's appearance.
	pal palette

	// ctx is the frame's context, kept to ask for another frame from an
	// action that runs between frames.
	ctx *ui.Context

	// states is what the app reports of each item for this frame.
	states map[string]ItemState

	// pending holds state changes asked for while the view was building, which
	// run before the next build pass, so a collection is never changed in the
	// loop that builds it.
	pending []func()

	// busy is the line shown while a long task runs.
	busy string

	list      ui.ListState
	grid      ui.GridState
	navScroll ui.ScrollState

	// locate is the item a search result brought into view, highlighted for a
	// moment.
	locate     string
	locateTill time.Time

	// launchAt throttles launches per item, so a double click runs once.
	launchAt map[string]time.Time

	// dialogs
	editOpen    bool
	editNew     bool
	editGUID    string
	editName    string
	editPath    string
	editArgs    string
	editWorkDir string
	editIcon    string
	editAdmin   bool

	runtimeOpen    bool
	runtimeGUID    string
	runtimeMinutes string

	renameOpen    bool
	renameTitle   string
	renameName    string
	renameGUID    string // an item, when set
	renameTabGUID string // a tab, when set

	confirmOpen   bool
	confirmMsg    string
	confirmAction func()

	searchOpen   bool
	searchQuery  string
	searchChosen int
	searchInput  ui.Handle
	searchList   ui.ListState
}

func newView(app *App) *view {
	return &view{
		app:            app,
		prefs:          loadPrefs(),
		states:         map[string]ItemState{},
		launchAt:       map[string]time.Time{},
		runtimeMinutes: "0",
	}
}

// later runs fn before the next build pass: actions that change the collection
// being built go through it.
func (v *view) later(fn func()) {
	v.pending = append(v.pending, fn)
	v.refresh()
}

// refresh asks for another frame, which is how an action taken between frames
// (a click, a menu entry) shows what it changed.
func (v *view) refresh() {
	if v.ctx != nil {
		v.ctx.Invalidate()
		return
	}
	if v.app.win != nil {
		v.app.win.Invalidate()
	}
}

// drain runs the actions queued for this build pass, and reports whether any
// ran: the frame they change needs another one.
func (v *view) drain() bool {
	ran := false
	for len(v.pending) > 0 {
		fns := v.pending
		v.pending = nil
		for _, fn := range fns {
			fn()
			ran = true
		}
	}
	return ran
}

func (v *view) view(c *ui.Context) {
	v.ctx = c
	deferred := v.drain()
	v.states = v.app.state()
	v.showMessages(c)

	// A runtime display that counts up needs a frame now and then; the clock
	// goroutine redraws while this says so.
	v.app.tick.Store(v.anyRunning() && v.app.store.Settings.AutoTime)

	if !c.FocusTakesText() && c.Shortcut(ui.Cmd, ui.KeyF) {
		v.openSearch()
	}

	p := applyPalette(c, v.prefs.Accent)
	v.pal = p
	c.Root().Background(p.bg)

	page := ui.Column(c).Fill()
	if page.FileDragOver() {
		page.Background(p.accent.Alpha(0.06))
	}
	page.Children(func() {
		v.header(c)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Background(p.bg).Children(func() {
			v.body(c)
			v.busyLine(c)
			v.addFab(c)
		})
	})
	v.dialogs(c)
	v.search(c)

	if deferred {
		// An action queued for this pass changed the state this frame shows
		// the old value of: build once more.
		c.Invalidate()
	}
}

// applyTheme sets the appearance the preference asks for. It runs when the
// preference changes and once at startup, not on every frame: a view stays a
// function of the state, and the appearance is the window's.
func (v *view) applyTheme() {
	switch v.prefs.Theme {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// showMessages hands the toasts and the busy line that goroutines queued to
// this frame.
func (v *view) showMessages(c *ui.Context) {
	toasts, busy := v.app.takeMessages()
	for _, msg := range toasts {
		c.Toast(msg)
	}
	v.busy = busy
}

// busyLine is the strip at the foot of the window shown while a long task
// runs.
func (v *view) busyLine(c *ui.Context) {
	if v.busy == "" {
		return
	}
	p := v.pal
	ui.Row(c).Height(34).Padding(0, 16).Gap(8).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(p.hairline).Children(func() {
		ui.Spinner(c).Size(12, 12)
		ui.Text(c, v.busy).FontSize(12).TextColor(p.muted)
	})
}

// addFab is the round button at the window's bottom right corner: it opens the
// add menu, files from disk or an item made by hand.
func (v *view) addFab(c *ui.Context) {
	p := v.pal
	b := ui.ButtonBase(c).Absolute().Bottom(24).Right(24).Size(52, 52).Radius(999).Center().
		Background(p.accent).TextColor(p.onAccent).Label("Add").
		Tooltip("Add files or create an item").FocusRing(false).
		Shadow(0, 8, 22, -6, ui.RGBA(0, 0, 0, 0.35))
	fill := p.accent
	if b.Hovered() {
		fill = p.accent.Mix(ui.RGB(255, 255, 255), 0.12)
	}
	b.Background(b.AnimateColor("bg", fill, ui.Animation{Duration: 120 * time.Millisecond}))
	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = 0.92
	}
	b.AnimatePose(pose, ui.Snappy)
	b.Children(func() { ui.Icon(c, iconPlus).Size(22, 22) })
	b.PopoverMenuButton(v.addMenu)
}

func (v *view) addMenu(m *ui.Menu) {
	if m.Item("Pick files…").Chosen() {
		v.later(func() { v.app.addFiles(v.tabGUID()) })
	}
	if m.Item("Create…").Chosen() {
		v.later(v.openCreate)
	}
}

// tabGUID is the guid of the active tab, or "" when there is none.
func (v *view) tabGUID() string {
	if tab := v.tab(); tab != nil {
		return tab.GUID
	}
	return ""
}

// body shows the active tab: a list of rows, or a grid of cells.
func (v *view) body(c *ui.Context) {
	tab := v.tab()
	if tab == nil {
		v.emptyState(c, nil)
		return
	}
	if v.prefs.ViewMode == "grid" {
		v.gridBody(c, tab)
		return
	}
	v.listBody(c, tab)
}

func (v *view) anyRunning() bool {
	for _, st := range v.states {
		if st.Running {
			return true
		}
	}
	return false
}

func (v *view) runningCount() int {
	n := 0
	for _, st := range v.states {
		if st.Running {
			n++
		}
	}
	return n
}

// emptyState says how to fill a tab without items.
func (v *view) emptyState(c *ui.Context, tab *CategoryNode) {
	p := v.pal
	ui.Column(c).Grow(1).Padding(40).Gap(8).Center().Children(func() {
		ui.Box(c).Size(48, 48).Radius(14).Background(p.hover).Center().Children(func() {
			ui.Icon(c, iconBox).Size(22, 22).TextColor(p.faint)
		})
		ui.Text(c, "No files added yet.").FontSize(14).FontWeight(500).TextColor(p.text)
		ui.Text(c, "Click Add to pick files or create an item, or drop files here.").
			FontSize(12).TextColor(p.muted)
		if tab != nil {
			ui.Text(c, tab.Name).FontSize(11).TextColor(p.faint)
		}
	})
}

// savePrefs writes the view's own settings.
func (v *view) savePrefs() {
	savePrefs(v.prefs)
	v.refresh()
}

// setViewMode switches the body between the list and the grid.
func (v *view) setViewMode(mode string) {
	v.prefs.ViewMode = mode
	v.savePrefs()
}
