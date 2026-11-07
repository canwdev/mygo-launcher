package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"go-launcher/locales"
)

// view is the window's interface: a function of the app's state, rebuilt for
// every frame. Everything in it is main thread state, as the store is.
type view struct {
	app *App
	win *mygo.Window

	// tray is the notification area icon, while the app keeps running in the
	// background.
	tray *mygo.Tray

	prefs uiPrefs

	// pal is the palette the frame draws with, from the theme's appearance.
	pal palette

	// lang is the language the window shows, which its texts are said in.
	lang string

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

	// customAccentOpen says the settings' custom accent picker shows.
	customAccentOpen bool

	// dragHoverTab is the page an item being dragged rests on, and
	// dragHoverSince when it came to rest there: the page shows itself after
	// dragTabHoverDelay.
	dragHoverTab   string
	dragHoverSince time.Time

	navScroll  ui.ScrollState
	navRect    ui.Rect            // the tab strip's box
	tabRects   map[string]ui.Rect // each page's chip
	centerNav  bool               // center the active chip next frame
	hoverTab   string             // the page chip the pointer rests on
	hoverSince time.Time

	// wheelSum is how far a wheel has turned toward the next page while the
	// wheel switches pages; hoverIgnoredTab is the chip it was used on, which
	// must not switch the page back until another chip is hovered.
	wheelSum        float32
	hoverIgnoredTab string

	// iconTimer redraws the app icons after the accent picker settles;
	// aboutIcon is the tinted icon of the settings' About row, kept until the
	// accent or appearance changes.
	iconTimer    *time.Timer
	aboutIcon    ui.ImageSource
	aboutIconKey string

	// pages is what each page keeps of its own: where it scrolls, and how
	// many cells its grid row holds.
	pages map[string]*pageView

	// menus keeps whether each context menu shows, by the element it belongs
	// to, as MyGo's ContextMenuBase asks for.
	menus map[string]*bool

	// folderGUID is the folder item whose contents the panel lists, "" while
	// it is closed; folderDir is the folder listed and folderRoot the folder of
	// the item, which going up never passes. folderItems is what the last read
	// found, folderGen drops a read a newer one replaced, folderAnchored says
	// the item's cell built this frame, and folderAtX/Y where the pointer was
	// when the panel opened, which it centers on.
	folderGUID       string
	folderDir        string
	folderRoot       string
	folderItems      []folderEntry
	folderErr        error
	folderLoading    bool
	folderGen        int
	folderAnchored   bool
	folderAtX        float32
	folderAtY        float32
	folderList       ui.ListState
	folderIcons      map[string]ui.ImageSource
	folderIconWanted map[string]bool
	folderIconQueue  []string
	folderIconBusy   bool

	// itemDragging says an item was dragged in the last frame, and
	// itemDraggingNow whether one is now: while it is, every page stays
	// built, so that showing another page does not end the drag.
	itemDragging    bool
	itemDraggingNow bool

	// locate is the item a search result brought into view, highlighted for a
	// moment.
	locate     string
	locateTill time.Time

	// launchAt throttles launches per item, so a double click runs once.
	launchAt map[string]time.Time

	// dialogs
	editOpen      bool
	editNew       bool
	editGUID      string
	editName      string
	editPath      string
	editArgs      string
	editWorkDir   string
	editIcon      string
	editIconIndex string
	editAdmin     bool

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

	// windowPicker lists the programs with a window open, to add one as an
	// item; pickerIcons keeps their icons by executable, and pickerLoading
	// says a goroutine still reads them.
	pickerOpen       bool
	pickerMenu       pickerMenuKind // the second-level list the picker shows
	pickerWindows    []runningWindow
	pickerIcons      map[string]ui.ImageSource
	pickerScroll     ui.ScrollState
	pickerLoading    bool
	pickerIconReload bool // a list opened while the icons still load

	// registeredShortcut is the accelerator the app has bound at the system,
	// and the rest is the settings' shortcut record: whether it listens for a
	// chord, the text input that takes it, and why the last change failed.
	registeredShortcut string
	shortcutRecording  bool
	shortcutDraft      string
	shortcutInput      ui.Handle

	// migrateStatus says how the last CLaunch import went, shown in the
	// settings' migration row; empty while none ran.
	migrateStatus string

	searchOpen   bool
	searchQuery  string
	searchChosen int
	searchInput  ui.Handle
	searchList   ui.ListState
}

func newView(app *App) *view {
	v := &view{
		app:            app,
		prefs:          loadPrefs(),
		states:         map[string]ItemState{},
		pages:          map[string]*pageView{},
		tabRects:       map[string]ui.Rect{},
		centerNav:      true,
		launchAt:       map[string]time.Time{},
		runtimeMinutes: "0",
		editIconIndex:  "0",
	}
	// Without a window, as in tests, App.refresh redraws through the view.
	app.view = v
	return v
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
	v.lang = c.Locale()
	deferred := v.drain()
	v.states = v.app.state()
	// The folder panel anchors to the cell of the item it lists, which must be
	// built this frame: a page switch takes it away.
	v.folderAnchored = false
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
	page.HandleInput(v.wheelInput)
	// The whole window takes files dragged from Explorer: those over a tab
	// chip go to that tab (see tabChip), the rest to the active one.
	if files := page.DroppedFiles(); len(files) > 0 {
		v.app.addPaths(files, v.tabGUID())
	}
	// An item dropped where no row, cell or page chip takes it lands at the
	// end of the page shown, rather than nowhere.
	if dragged, ok := ui.Drop[dragItem](page); ok {
		copyItem := c.Modifiers()&ui.Cmd != 0
		guid := v.tabGUID()
		at := 0
		if tab := v.tab(); tab != nil {
			at = len(tab.Slots)
		}
		v.later(func() { v.moveItemToTabAt(dragged.guid, guid, at, copyItem) })
	}
	page.Children(func() {
		v.header(c)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Background(p.bg).Children(func() {
			v.body(c)
			v.folderMenu(c)
			v.busyLine(c)
			v.addFab(c)
		})
	})
	v.dialogs(c)
	v.search(c)

	// What this frame saw of a drag serves the next one.
	v.itemDragging, v.itemDraggingNow = v.itemDraggingNow, false

	// The page shown is where the strip centers, after the wheel or a click
	// chose it.
	if v.centerNav && v.navRect.W > 0 {
		if r, ok := v.tabRects[v.prefs.ActiveTab]; ok {
			x := v.navScroll.X + (r.X + r.W/2) - (v.navRect.X + v.navRect.W/2)
			x = max(0, min(x, v.navScroll.MaxX))
			v.navScroll.X = x
		}
		v.centerNav = false
	}

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
	ui.Row(c).Height(30).Padding(0, 12).Gap(8).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(p.hairline).Children(func() {
		ui.Spinner(c).Size(12, 12)
		ui.Text(c, v.busy).FontSize(12).TextColor(p.muted)
	})
}

// addFab is the round button at the window's bottom right corner: it opens the
// add menu, files from disk or an item made by hand.
func (v *view) addFab(c *ui.Context) {
	p := v.pal
	m := v.msgs()
	b := ui.ButtonBase(c).Absolute().Bottom(20).Right(20).Size(46, 46).Radius(999).Center().
		Background(p.accent).TextColor(p.onAccent).Label(m.AddButton()).
		Tooltip(m.AddButtonHint()).FocusRing(false).
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
	b.Children(func() { ui.Icon(c, iconPlus).Size(20, 20) })
	b.OnClick(func() {
		if v.pickerOpen {
			v.closeWindowPicker()
			return
		}
		v.openWindowPicker()
	})
	v.windowPicker(c, b)
}

// tabGUID is the guid of the active tab, or "" when there is none.
func (v *view) tabGUID() string {
	if tab := v.tab(); tab != nil {
		return tab.GUID
	}
	return ""
}

// dropAfter reports whether the pointer over e aims past its middle: right of
// side-by-side items, as a grid's cells and pages, or below a stacked row. A
// native drop, as files from Explorer, knows where it landed but not that the
// pointer is over e, so the position in e's box decides.
func dropAfter(e ui.Element, vertical bool) bool {
	x, y, _ := e.PointerPosition()
	b := e.Bounds()
	if vertical {
		return x > b.W/2 && x < b.W
	}
	return y > b.H/2 && y < b.H
}

// fileDropHalf makes half of an item a target for files dragged from another
// app, inserting them at at, and shows the insertion line on the item it is
// half of. A native drop says which element it is over, not where in it, so
// the halves say before or after.
func (v *view) fileDropHalf(parent, half ui.Element, tabGUID string, at int, vertical, after bool, p palette) {
	if files := half.DroppedFiles(); len(files) > 0 {
		v.later(func() { v.app.addPathsAt(files, tabGUID, at) })
	}
	if half.FileDragOver() {
		insertLine(parent, vertical, after, p)
	}
}

// insertLine shows where a drop lands: a vertical line at the left or right
// edge of side-by-side items, as between a grid's cells, or a horizontal line
// above or below a row, as between a list's rows. It paints over the element
// instead of drawing a border, so it takes no room from the icon or the text
// and does not bend around the element's rounded corners; the line stops short
// of the corners.
func insertLine(e ui.Element, vertical, after bool, p palette) {
	e.DrawOver(func(pt *ui.Painter, r ui.Rect) {
		const width, end float32 = 2, 6
		var line ui.Rect
		if vertical {
			x := r.X
			if after {
				x = r.X + r.W - width
			}
			line = ui.Rect{X: x, Y: r.Y + end, W: width, H: max(0, r.H-2*end)}
		} else {
			y := r.Y
			if after {
				y = r.Y + r.H - width
			}
			line = ui.Rect{X: r.X + end, Y: y, W: max(0, r.W-2*end), H: width}
		}
		pt.Fill(line, p.accent, width/2)
	})
}

// dropTo turns the side a drop aims at into the index moveSlot takes: at is
// index when left of the slot, index+1 when right, in the slots as they are; a
// moved slot lands one place earlier once it leaves.
func dropTo(from, at int, copyItem bool) int {
	if copyItem || from >= at {
		return at
	}
	return at - 1
}

// body shows the active page: a grid of cells. While an item is dragged every
// page builds, though hidden, so that showing another page does not end the
// drag: the item's cell stays mounted, and the drop can land anywhere in the
// page the pointer shows.
func (v *view) body(c *ui.Context) {
	active := v.tab()
	if active == nil {
		v.emptyState(c, nil)
		return
	}
	tabs := v.app.store.Categories
	ui.Box(c).Grow(1).Children(func() {
		// The pages kept mounted while an item is dragged go under the active
		// one, which paints last over them, so that its background hides them
		// while its drag keeps painting the copy the pointer follows.
		for activePass := 0; activePass < 2; activePass++ {
			for i := range tabs {
				tab := &tabs[i]
				isActive := tab.GUID == active.GUID
				if isActive != (activePass == 1) {
					continue
				}
				if !isActive && !v.itemDragging {
					continue
				}
				page := ui.Box(c).Key(tab.GUID).Absolute().Fill().Background(v.pal.bg)
				page.Children(func() {
					v.gridBody(c, tab)
				})
			}
		}
	})
}

// pageView is what the window keeps for a page of its own.
type pageView struct {
	gridScroll ui.ScrollState
	gridCols   int
}

// pageState returns what a page keeps, making it the first time.
func (v *view) pageState(tab *CategoryNode) *pageView {
	pv := v.pages[tab.GUID]
	if pv == nil {
		pv = &pageView{}
		v.pages[tab.GUID] = pv
	}
	return pv
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
	m := v.msgs()
	ui.Column(c).Grow(1).Padding(28).Gap(6).Center().Children(func() {
		ui.Box(c).Size(44, 44).Radius(12).Background(p.hover).Center().Children(func() {
			ui.Icon(c, iconBox).Size(20, 20).TextColor(p.faint)
		})
		ui.Text(c, m.NoFilesTitle()).FontSize(14).FontWeight(500).TextColor(p.text)
		ui.Text(c, m.NoFilesHint()).
			FontSize(12).TextColor(p.muted)
		if tab != nil {
			ui.Text(c, tab.Name).FontSize(11).TextColor(p.faint)
		}
	})
}

// msgs says the app's texts in the language the window shows.
func (v *view) msgs() locales.Messages { return locales.In(v.lang) }

// savePrefs writes the view's own settings.
func (v *view) savePrefs() {
	savePrefs(v.prefs)
	v.refresh()
}
