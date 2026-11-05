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

	// dragSlot is the slot a value is dragged over, -1 for none: the rows and
	// cells that would take it draw a line.
	dragSlot int

	list ui.ListState
	grid ui.GridState

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
		dragSlot:       -1,
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

	t := c.Theme()
	root := c.Root()
	root.Background(t.Background)

	column := ui.Column(c).Fill()
	if column.FileDragOver() {
		column.Background(t.Accent.Alpha(0.06))
	}
	column.Children(func() {
		v.header(c)
		v.body(c)
		v.busyLine(c)
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
	t := c.Theme()
	ui.Row(c).Padding(5, 10).Gap(8).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Spinner(c).Size(11, 11)
		ui.Text(c, v.busy).FontSize(11).TextColor(t.TextMuted)
	})
}

// header is the tab strip and the window's buttons.
func (v *view) header(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(6, 8).Gap(8).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).Gap(2).AlignItems(ui.Center).Wrap().Grow(1).Children(func() {
			for i := range v.app.store.Categories {
				v.tabChip(c, i)
			}
			iconButton(c, "Add tab", iconPlus, func() { v.later(v.addTab) })
		})
		ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
			v.addButton(c)
			v.themeButton(c)
			v.viewModeButton(c)
			iconButton(c, "Search", iconSearch, v.openSearch)
			v.appMenuButton(c)
		})
	})
}

// iconButton is a small borderless button showing an icon.
func iconButton(c *ui.Context, label string, svg *ui.SVG, onClick func()) {
	ui.Button(c, "").Label(label).Tooltip(label).Padding(4).Children(func() {
		ui.Icon(c, svg).FontSize(16).TextColor(c.Theme().TextMuted)
	}).OnClick(onClick)
}

// addButton opens the add menu: files from disk, or an item made by hand.
func (v *view) addButton(c *ui.Context) {
	ui.Button(c, "").Label("Add").Tooltip("Add").Padding(4).Children(func() {
		ui.Icon(c, iconPlus).FontSize(16).TextColor(c.Theme().TextMuted)
	}).PopoverMenuButton(func(m *ui.Menu) {
		if m.Item("Pick files…").Chosen() {
			v.later(func() { v.app.addFiles(v.tab().GUID) })
		}
		if m.Item("Create…").Chosen() {
			v.later(v.openCreate)
		}
	})
}

// themeButton cycles auto → light → dark.
func (v *view) themeButton(c *ui.Context) {
	icon, label := iconSunMoon, "Theme: auto"
	switch v.prefs.Theme {
	case "light":
		icon, label = iconSun, "Theme: light"
	case "dark":
		icon, label = iconMoon, "Theme: dark"
	}
	iconButton(c, label, icon, func() {
		switch v.prefs.Theme {
		case "auto":
			v.prefs.Theme = "light"
		case "light":
			v.prefs.Theme = "dark"
		default:
			v.prefs.Theme = "auto"
		}
		v.savePrefs()
		v.applyTheme()
	})
}

// viewModeButton switches between the list and the grid.
func (v *view) viewModeButton(c *ui.Context) {
	if v.prefs.ViewMode == "grid" {
		iconButton(c, "Switch to list view", iconList, func() { v.setViewMode("list") })
		return
	}
	iconButton(c, "Switch to grid view", iconGrid, func() { v.setViewMode("grid") })
}

func (v *view) setViewMode(mode string) {
	v.prefs.ViewMode = mode
	v.savePrefs()
}

// appMenuButton is the window's menu, as the page's ⋯ was.
func (v *view) appMenuButton(c *ui.Context) {
	ui.Button(c, "").Label("Menu").Tooltip("Menu").Padding(4).Children(func() {
		ui.Icon(c, iconDots).FontSize(16).TextColor(c.Theme().TextMuted)
	}).PopoverMenuButton(func(m *ui.Menu) {
		s := &v.app.store.Settings
		if m.Item("Auto timing").Checked(s.AutoTime).Chosen() {
			s.AutoTime = !s.AutoTime
			v.app.save()
		}
		if m.Item("Auto-hide Launcher").Checked(s.AutoHide).Chosen() {
			s.AutoHide = !s.AutoHide
			v.app.save()
		}
		if m.Item("Always on top").Checked(s.AlwaysOnTop).Chosen() {
			v.app.setAlwaysOnTop(!s.AlwaysOnTop)
		}
		if m.Item("Abs path for new items").Checked(s.AbsolutePaths).Chosen() {
			s.AbsolutePaths = !s.AbsolutePaths
			v.app.save()
		}
		m.Separator()
		if m.Item("Refresh").Chosen() {
			v.later(v.refreshData)
		}
		if m.Item("Open data directory…").Chosen() {
			v.later(v.app.openDataDir)
		}
		m.Separator()
		if m.Item("Convert to absolute path").Chosen() {
			v.later(func() { v.convertPaths(true) })
		}
		if m.Item("Convert to relative path").Chosen() {
			v.later(func() { v.convertPaths(false) })
		}
		if m.Item("Batch update icons").Chosen() {
			v.later(v.batchUpdateIcons)
		}
		m.Separator()
		if m.Item("v" + appVersion + " | GitHub").Chosen() {
			mygo.Shell.OpenExternal("https://github.com/canwdev/go-launcher")
		}
	})
}

// body shows the active tab: a list of rows, or a grid of cells.
func (v *view) body(c *ui.Context) {
	tab := v.tab()
	if tab == nil {
		ui.Text(c, "No tabs").TextColor(c.Theme().TextMuted).Padding(16)
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

// savePrefs writes the view's own settings.
func (v *view) savePrefs() {
	savePrefs(v.prefs)
	v.refresh()
}
