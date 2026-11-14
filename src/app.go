package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/google/uuid"
	"go-launcher/locales"
)

// runningProc is a process the launcher started and tracks: its runtime
// accumulates until it exits, and it can be stopped.
type runningProc struct {
	start time.Time
	wait  func() error
	stop  func() error
	// timed is false once the timer was turned off while the process ran:
	// its time is no longer added when it exits.
	timed   bool
	cleanup func()
}

// ItemState is what the view shows of an item besides its stored fields.
type ItemState struct {
	Running   bool
	RuntimeMs int64
	StartAt   int64 // unix ms of the process start, 0 when it is not running
}

// App owns the store and the processes. Everything in it is used on the main
// thread, by the view and the actions it runs; goroutines hand their results
// over through Window.Update, except for what the mutexes below name.
type App struct {
	store        AppStore
	runtimeStats map[string]int64
	running      map[string]*runningProc

	// icons caches decoded icons by the path the store keeps.
	icons map[string]ui.ImageSource

	// dynamicIcons are state icons, as the Recycle Bin's: kept in memory for
	// the item, never written to the icon cache.
	dynamicIcons map[string]ui.ImageSource

	// missing holds the items whose file or folder was found missing by the
	// availability check or a launch; the grid dims them.
	missing map[string]bool

	// updates reports whether the updater plugin is used.
	updates bool

	// fileMu guards the store file fingerprint, which the watch goroutine reads
	// and writes.
	fileMu    sync.Mutex
	storeFile storeFileState

	// msgMu guards what goroutines hand to the next frame: the toasts to show
	// and the busy line.
	msgMu  sync.Mutex
	toasts []string
	busy   string

	// tick tells the clock goroutine that a runtime display keeps changing.
	tick atomic.Bool

	// quitting is set when a quit starts, so the main window closes rather
	// than hiding in the tray.
	quitting bool

	win         *mygo.Window
	settingsWin *mygo.Window
	view        *view

	watchCancel func()
}

func NewApp() *App {
	_ = os.MkdirAll(filepath.Join(dataBase(), dataDir), 0o755)
	app := &App{
		store:        loadStore(),
		runtimeStats: map[string]int64{},
		running:      map[string]*runningProc{},
		icons:        map[string]ui.ImageSource{},
		dynamicIcons: map[string]ui.ImageSource{},
	}
	if app.rebaseIcons() {
		_, _ = writeStoreAtomic(app.store)
	}
	for guid, item := range app.store.Apps {
		if item != nil {
			app.runtimeStats[guid] = item.RuntimeMs
		}
	}
	app.pruneIcons()
	app.syncStoreFileState()
	return app
}

// state reports what the view shows of every item.
func (a *App) state() map[string]ItemState {
	out := make(map[string]ItemState, len(a.store.Apps))
	for guid, item := range a.store.Apps {
		if item == nil {
			continue
		}
		st := ItemState{RuntimeMs: a.runtimeStats[guid]}
		if p, ok := a.running[guid]; ok {
			st.Running = true
			st.StartAt = p.start.UnixMilli()
		}
		out[guid] = st
	}
	return out
}

// setMissing records whether an item's file or folder was found; the grid dims
// the items that were not.
func (a *App) setMissing(guid string, missing bool) {
	if missing {
		if a.missing == nil {
			a.missing = map[string]bool{}
		}
		a.missing[guid] = true
		return
	}
	delete(a.missing, guid)
}

// isMissing reports whether the last check found the item's path missing.
func (a *App) isMissing(guid string) bool { return a.missing[guid] }

// itemPresent reports whether item's file or folder exists now. A shell target
// has no file to check and counts as present; an item without a path opens its
// working directory, as a folder item does.
func itemPresent(item *AppItem) bool {
	if item == nil || isShellTarget(item.Path) {
		return true
	}
	if item.Path == "" {
		return item.WorkingDir != "" && fileExists(absPath(item.WorkingDir))
	}
	return fileExists(resolveLaunchPath(item.Path))
}

func (a *App) item(guid string) *AppItem {
	return a.store.Apps[guid]
}

// tab returns the tab with guid, or the first one.
func (a *App) tab(guid string) *CategoryNode {
	for i := range a.store.Categories {
		if a.store.Categories[i].GUID == guid {
			return &a.store.Categories[i]
		}
	}
	if len(a.store.Categories) > 0 {
		return &a.store.Categories[0]
	}
	return nil
}

// save writes the store, runtime stats included, and records the fingerprint
// that keeps the watch from reading the file back as an edit made by hand.
func (a *App) save() {
	for guid, ms := range a.runtimeStats {
		if item := a.store.Apps[guid]; item != nil {
			item.RuntimeMs = ms
		}
	}
	data, err := writeStoreAtomic(a.store)
	if err != nil {
		a.notify(locales.SavingFailed(err.Error()))
		return
	}
	a.fileMu.Lock()
	a.rememberStoreFile(data)
	a.fileMu.Unlock()
	a.refresh()
}

// refresh redraws the windows after the store or a process changed; without a
// window, as in tests, the view is asked for the frame instead.
func (a *App) refresh() {
	if a.win != nil {
		a.win.Invalidate()
	}
	if a.settingsWin != nil {
		a.settingsWin.Invalidate()
	}
	if a.win == nil && a.view != nil {
		a.view.refresh()
	}
}

// dispatch runs fn on the main thread, where the store and the view live.
// Goroutines hand everything they change over through it. Without a window,
// as in tests, it runs at once.
func (a *App) dispatch(fn func()) {
	if a.win == nil {
		fn()
		return
	}
	a.win.Update(fn)
}

// minimize hides the window when there is one, as auto-hide asks after a
// launch. Nothing brings it back on its own, but the tray does. A tray-only
// window has no minimize to fall back to, so it goes to the tray instead. It
// runs on the main thread, where the view's preferences are read.
func (a *App) minimize() {
	if a.win == nil {
		return
	}
	if a.view != nil && a.view.prefs.TrayOnly {
		a.hideWindow()
		return
	}
	a.win.Minimize()
	releaseMemory()
}

// releaseMemory returns to the system what the last frames left behind: a
// window that left the screen keeps less while the app waits in the tray.
func releaseMemory() {
	debug.FreeOSMemory()
}

// notify queues a toast for the next frame; safe from any goroutine.
func (a *App) notify(msg string) {
	a.msgMu.Lock()
	a.toasts = append(a.toasts, msg)
	a.msgMu.Unlock()
	a.refresh()
}

func (a *App) notifyErr(err error) {
	if err != nil {
		a.notify(err.Error())
	}
}

// setBusy shows (or clears, with "") the busy line; safe from any goroutine.
func (a *App) setBusy(msg string) {
	a.msgMu.Lock()
	a.busy = msg
	a.msgMu.Unlock()
	a.refresh()
}

func (a *App) takeMessages() (toasts []string, busy string) {
	a.msgMu.Lock()
	toasts, busy = a.toasts, a.busy
	a.toasts = nil
	a.msgMu.Unlock()
	return toasts, busy
}

// openPath opens a file, a folder or a URL with the system's default handler.
func openPath(path string) error {
	return mygo.Shell.OpenPath(path)
}

// openFile opens a path: executables (and items that ask for elevation) start as
// a real process, so args and working directory apply, but without tracking:
// no Stop handle, no runtime. A URL or shell target is handed to the shell.
func openFile(path string, args []string, workDir string, elevated bool) error {
	if isShellTarget(path) {
		return openShellTarget(path)
	}
	if isExecutable(path) || elevated {
		return startUntracked(path, args, workDir, elevated)
	}
	return openPath(path)
}

// openShellTarget opens a URL, a shell URI or a Start menu app. A packaged app
// is started by its Applications folder item with the default verb, which the
// "open" verb OpenPath uses does not reach.
func openShellTarget(path string) error {
	if strings.HasPrefix(strings.ToLower(path), "shell:appsfolder\\") {
		return mygo.Shell.LaunchApp(mygo.InstalledApp{Path: path})
	}
	return openPath(path)
}

// launchPlan is what a launch needs, read on the main thread: the process may
// take a while to start (an elevation prompt waits for the user), so it runs on
// a goroutine of its own.
type launchPlan struct {
	guid     string
	path     string
	args     []string
	workDir  string
	elevated bool
	track    bool // an executable is tracked, so it can be stopped; its runtime is kept while auto timing is on
	hide     bool // auto-hide minimizes the window
}

var errAlreadyRunning = errors.New("already running")

// planLaunch reads what running an item takes.
func (a *App) planLaunch(guid string) (launchPlan, error) {
	item := a.item(guid)
	if item == nil {
		return launchPlan{}, fmt.Errorf("invalid item guid %q", guid)
	}
	// With the timer off nothing is tracked, so the same program can run
	// more than once and there is no process to stop.
	timing := a.store.Settings.AutoTime
	if timing {
		if _, ok := a.running[guid]; ok {
			return launchPlan{}, errAlreadyRunning
		}
	}
	plan := launchPlan{
		guid:     guid,
		hide:     a.store.Settings.AutoHide,
		elevated: item.RunAsAdmin,
		args:     splitArgs(item.Args),
	}
	if item.WorkingDir != "" {
		plan.workDir = absPath(item.WorkingDir)
	}
	if item.Path == "" {
		// A folder item opens its working directory; there is no process to
		// track or time.
		plan.path, plan.args, plan.elevated = plan.workDir, nil, false
		if !fileExists(plan.path) {
			a.setMissing(guid, true)
			return launchPlan{}, errors.New(locales.PathMissing())
		}
		a.setMissing(guid, false)
		return plan, nil
	}
	if isShellTarget(item.Path) {
		// A URL or shell namespace goes to the shell as written: it is not a
		// file to resolve and not a process to track.
		plan.path, plan.elevated = item.Path, false
		a.setMissing(guid, false)
		return plan, nil
	}
	plan.path = resolveLaunchPath(item.Path)
	if !fileExists(plan.path) {
		a.setMissing(guid, true)
		return launchPlan{}, errors.New(locales.PathMissing())
	}
	a.setMissing(guid, false)
	plan.track = isExecutable(plan.path) && timing
	return plan, nil
}

// runLaunch starts what a plan describes. It may block, as an elevation prompt
// does, so it runs on a goroutine of its own.
func (a *App) runLaunch(plan launchPlan) {
	if plan.path == "" {
		return
	}
	if plan.track {
		p := &runningProc{timed: true}
		if err := startTracked(plan.path, plan.args, plan.workDir, plan.elevated, p); err != nil {
			a.notifyErr(err)
			return
		}
		a.dispatch(func() {
			// The timer may have been turned off while the process started:
			// it runs, but the window no longer tracks it.
			if a.store.Settings.AutoTime {
				a.running[plan.guid] = p
			} else {
				p.timed = false
			}
			if plan.hide {
				a.minimize()
			}
		})
		a.watchExit(plan.guid, p)
		return
	}
	if err := openFile(plan.path, plan.args, plan.workDir, plan.elevated); err != nil {
		a.notifyErr(err)
		return
	}
	if plan.hide {
		a.dispatch(a.minimize)
	}
}

// watchExit accumulates a tracked process' runtime once it ends, when auto
// timing is on. The window stays where the launch left it: nothing is restored
// automatically.
func (a *App) watchExit(guid string, p *runningProc) {
	go func() {
		_ = p.wait()
		elapsed := time.Since(p.start).Milliseconds()
		a.dispatch(func() { a.finishProcess(guid, p, elapsed) })
	}()
}

// finishProcess drops a process that ended and adds its time to the runtime,
// unless auto timing is off: then its clock never moved and there is nothing to
// add.
func (a *App) finishProcess(guid string, p *runningProc, elapsed int64) {
	// A process the timer stopped following may exit after the same item was
	// started again: only drop the entry that is still this process.
	if a.running[guid] == p {
		delete(a.running, guid)
	}
	if p.cleanup != nil {
		p.cleanup()
	}
	if !a.store.Settings.AutoTime || !p.timed {
		a.refresh()
		return
	}
	a.runtimeStats[guid] += elapsed
	a.save()
}

// stopTiming ends everything the timer was doing, without ending the programs
// it tracked: no process is running as far as the window is concerned, a
// launch no longer waits for one to stop, and a process still running when the
// timer comes back is not timed from this point on either.
func (a *App) stopTiming() {
	for _, p := range a.running {
		p.timed = false
	}
	a.running = map[string]*runningProc{}
	a.tick.Store(false)
	a.refresh()
}

func (a *App) stop(guid string) error {
	p, ok := a.running[guid]
	if ok && p.stop != nil {
		if err := p.stop(); err != nil {
			return err
		}
		a.refresh()
	}
	return nil
}

// setRuntimeMs overrides an item's accumulated runtime (the runtime dialog
// writes through it). Negative input is clamped to 0.
func (a *App) setRuntimeMs(guid string, ms int64) error {
	item := a.item(guid)
	if item == nil {
		return fmt.Errorf("invalid item guid %q", guid)
	}
	if ms < 0 {
		ms = 0
	}
	a.runtimeStats[guid] = ms
	item.RuntimeMs = ms
	a.save()
	a.refresh()
	return nil
}

// addFiles asks for files and adds them to the tab with tabGUID.
func (a *App) addFiles(tabGUID string) {
	go func() {
		paths, err := openFilesDialog(a.win, locales.AddFilesTitle())
		if err != nil || len(paths) == 0 {
			return
		}
		a.addPaths(paths, tabGUID)
	}()
}

// addPaths adds files, folders and shortcuts to a tab. Icons are extracted on a
// goroutine, since that reads the disk for every file; without a window, as in
// tests, it adds them at once.
func (a *App) addPaths(paths []string, tabGUID string) {
	if len(paths) == 0 {
		return
	}
	absolute := a.store.Settings.AbsolutePaths
	// Without a window, as in tests, the shell calls behind the icons would
	// wait for a main thread that never runs: skip them.
	withIcon := a.win != nil
	if !withIcon {
		a.addItems(buildItems(paths, absolute, false), tabGUID)
		return
	}
	a.setBusy(locales.AddingFiles())
	go func() {
		items := buildItems(paths, absolute, true)
		a.dispatch(func() {
			a.setBusy("")
			a.addItems(items, tabGUID)
		})
	}()
}

// addPathsAt adds files, folders and shortcuts at slot at of a tab, or at its
// end for -1, as dropping them between two items does.
func (a *App) addPathsAt(paths []string, tabGUID string, at int) {
	if len(paths) == 0 {
		return
	}
	absolute := a.store.Settings.AbsolutePaths
	withIcon := a.win != nil
	if !withIcon {
		a.addItemsAt(buildItems(paths, absolute, false), tabGUID, at)
		return
	}
	a.setBusy(locales.AddingFiles())
	go func() {
		items := buildItems(paths, absolute, true)
		a.dispatch(func() {
			a.setBusy("")
			a.addItemsAt(items, tabGUID, at)
		})
	}()
}

// addRunningWindow adds the program behind a window to the tab with tabGUID.
func (a *App) addRunningWindow(w runningWindow, tabGUID string) {
	if w.Path == "" {
		return
	}
	absolute := a.store.Settings.AbsolutePaths
	withIcon := a.win != nil
	if !withIcon {
		if item := buildRunningItem(w, absolute, false); item != nil {
			a.addItems([]*AppItem{item}, tabGUID)
		}
		return
	}
	a.setBusy(locales.AddingProgram())
	go func() {
		item := buildRunningItem(w, absolute, true)
		a.dispatch(func() {
			a.setBusy("")
			if item != nil {
				a.addItems([]*AppItem{item}, tabGUID)
			}
		})
	}()
}

// addSystemLocation adds a shell folder, as the Recycle Bin, to a tab. Its
// path is a parsing name the shell opens, not a file: it is kept as written,
// has no working directory, and the shell draws its icon.
func (a *App) addSystemLocation(loc systemLocation, name, tabGUID string) {
	makeItem := func(withIcon bool) *AppItem {
		icon := ""
		if withIcon {
			icon = writeIcon(loc.Path)
		}
		return &AppItem{GUID: uuid.NewString(), Name: name, Path: loc.Path, Icon: icon}
	}
	if a.win == nil {
		a.addItems([]*AppItem{makeItem(false)}, tabGUID)
		return
	}
	a.setBusy(locales.AddingLocation())
	go func() {
		item := makeItem(true)
		a.dispatch(func() {
			a.setBusy("")
			a.addItems([]*AppItem{item}, tabGUID)
		})
	}()
}

// buildRunningItem makes an entry for the program of a window, named as the
// program names itself rather than after the window's current title.
func buildRunningItem(w runningWindow, absolute, withIcon bool) *AppItem {
	path := normalizePath(w.Path)
	if path == "" {
		return nil
	}
	name := strings.TrimSpace(w.Name)
	if name == "" {
		name = defaultTitle(path)
	}
	icon := ""
	if withIcon {
		icon = writeIcon(path)
	}
	workDir := ""
	if !isShellTarget(path) {
		workDir = storedForm(filepath.Dir(path), absolute)
	}
	return &AppItem{
		GUID:       uuid.NewString(),
		Name:       name,
		Path:       storedForm(path, absolute),
		WorkingDir: workDir,
		Icon:       icon,
	}
}

// buildItems makes entries for paths, resolving shortcuts and caching icons. It
// reads the disk, so it runs off the main thread outside tests.
func buildItems(paths []string, absolute, withIcon bool) []*AppItem {
	items := make([]*AppItem, 0, len(paths))
	for _, raw := range paths {
		if item := buildItem(raw, absolute, withIcon); item != nil {
			items = append(items, item)
		}
	}
	return items
}

// addItems puts entries at the end of a tab and saves the store.
func (a *App) addItems(items []*AppItem, tabGUID string) {
	a.addItemsAt(items, tabGUID, -1)
}

// addItemsAt puts entries at slot at of a tab, or at its end for -1, and saves
// the store.
func (a *App) addItemsAt(items []*AppItem, tabGUID string, at int) {
	if len(items) == 0 {
		return
	}
	tab := a.tab(tabGUID)
	if tab == nil {
		return
	}
	if at < 0 || at > len(tab.Slots) {
		at = len(tab.Slots)
	}
	for _, item := range items {
		a.store.Apps[item.GUID] = item
		a.runtimeStats[item.GUID] = 0
		guid := item.GUID
		tab.Slots = append(tab.Slots[:at], append([]*string{&guid}, tab.Slots[at:]...)...)
		at++
	}
	a.save()
}

// buildItem makes an item for a path, resolving shortcuts and caching the icon.
// It reads the disk, so it runs off the main thread.
func buildItem(raw string, absolute, withIcon bool) *AppItem {
	path := normalizePath(raw)
	if path == "" {
		return nil
	}
	name := defaultTitle(path)
	target, args, workDir, link := path, "", filepath.Dir(path), ""
	switch {
	case isShortcutFile(path):
		if lnk, err := resolveShortcut(path); err == nil && lnk.Target != "" {
			target, args, workDir = lnk.Target, lnk.Args, lnk.WorkDir
			link = storedForm(path, absolute)
			if workDir == "" {
				workDir = filepath.Dir(target)
			}
		}
	case isURLFile(path):
		// An Internet Shortcut points at a page: the item opens the page it
		// names, and keeps the file for its icon and "Update icon".
		if us, err := resolveInternetShortcut(path); err == nil && us.URL != "" {
			target, workDir = us.URL, ""
			link = storedForm(path, absolute)
		}
	}
	icon := ""
	if withIcon {
		// An image shows itself, rather than the icon of its file type.
		icon = contentIcon(path)
		if icon == "" {
			// A shortcut's own icon is the one Explorer draws, with the app's
			// badge; the target's may be a blank stub, as chrome_proxy.exe's is.
			icon = writeIcon(path)
			if icon == "" && target != path && !isShellTarget(target) {
				icon = writeIcon(target)
			}
		}
	}
	return &AppItem{
		GUID:       uuid.NewString(),
		Name:       name,
		Path:       storedForm(target, absolute),
		WorkingDir: storedForm(workDir, absolute),
		Icon:       icon,
		Args:       args,
		Link:       link,
	}
}

func storedForm(path string, absolute bool) string {
	if path == "" || absolute {
		return path
	}
	return toStoredPath(path)
}

// updateIcon extracts an item's icon again from its path.
func (a *App) updateIcon(guid string) error {
	item := a.item(guid)
	if item == nil {
		return fmt.Errorf("invalid item guid %q", guid)
	}
	// An icon outside the app's folders is taken in, drawn down like one the
	// dialog chose; one already there needs nothing.
	if item.Icon != "" && !allowedIcon(item.Icon) {
		icon, err := a.customIcon(item.Icon, 0)
		if err != nil {
			return err
		}
		a.replaceIcon(item, icon)
		return nil
	}
	if isCustomIcon(item.Icon) {
		return nil // already kept in the app's folder
	}
	// A shell folder, as the Recycle Bin, has no file to check: the shell draws
	// its icon from the parsing name.
	if isShellNamespace(item.Path) {
		icon := writeIcon(item.Path)
		if icon == "" {
			return errors.New(locales.IconFailed())
		}
		a.replaceIcon(item, icon)
		return nil
	}
	if item.Path == "" && item.Link == "" {
		return errors.New(locales.NoIconPath())
	}
	linkPath, launchPath := "", ""
	if item.Link != "" {
		linkPath = absPath(item.Link)
	}
	if item.Path != "" {
		launchPath = resolveLaunchPath(item.Path)
	}
	if !fileExists(linkPath) && !fileExists(launchPath) {
		return errors.New(locales.PathMissing())
	}
	// A shortcut's own icon is the one Explorer draws; its target's may be a
	// blank stub. An image or an .ico shows its own content.
	icon := ""
	if linkPath != "" {
		icon = contentIcon(linkPath)
		if icon == "" {
			icon = writeIcon(linkPath)
		}
	}
	if icon == "" && launchPath != "" {
		icon = contentIcon(launchPath)
		if icon == "" {
			icon = writeIcon(launchPath)
		}
	}
	if icon == "" {
		return errors.New(locales.IconFailed())
	}
	a.replaceIcon(item, icon)
	return nil
}

// refreshSpecialIcon redraws an item's icon when its special source reports a
// new one, as the Recycle Bin does when its contents change. The image stays
// in memory: a state icon changes often and must not fill the icon cache.
func (a *App) refreshSpecialIcon(item *AppItem) bool {
	if item == nil || isCustomIcon(item.Icon) {
		return false
	}
	sp := specialItemFor(item.Path)
	if sp == nil || sp.RefreshIcon == nil {
		return false
	}
	data, ok := sp.RefreshIcon(item)
	if !ok || len(data) == 0 {
		return false
	}
	src, err := ui.DecodeImage(data)
	if err != nil {
		return false
	}
	if a.dynamicIcons == nil {
		a.dynamicIcons = map[string]ui.ImageSource{}
	}
	a.dynamicIcons[item.GUID] = src
	a.refresh()
	return true
}

// clearDynamicIcon drops an item's state icon, so a chosen icon shows instead.
func (a *App) clearDynamicIcon(guid string) {
	delete(a.dynamicIcons, guid)
}

func (a *App) replaceIcon(item *AppItem, icon string) {
	a.clearDynamicIcon(item.GUID)
	if old := item.Icon; old != "" {
		// Never remove a file outside the app's icon folders: an icon added
		// before the folders were enforced may point at the user's own file.
		if isAutoIcon(old) || isCustomIcon(old) {
			_ = os.Remove(absPath(old))
		}
		delete(a.icons, old)
	}
	item.Icon = icon
	a.save()
	a.refresh()
}

func iconFilter() []mygo.FileFilter {
	return []mygo.FileFilter{{Name: locales.IconFilesFilter(), Extensions: []string{
		"png", "jpg", "jpeg", "gif", "bmp", "webp", "svg", "ico", "exe", "dll",
	}}}
}

// pickFile asks for a file; dir is where the dialog starts.
func (a *App) pickFile(dir string) string {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: locales.SelectFileTitle(), DefaultPath: dir})
	if err != nil || len(paths) == 0 {
		return ""
	}
	return a.storedPick(paths[0])
}

func (a *App) pickIcon(dir string) string {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: locales.SelectIconTitle(), DefaultPath: dir, Filters: iconFilter()})
	if err != nil || len(paths) == 0 {
		return ""
	}
	return a.storedPick(paths[0])
}

// showIconPicker is the shell's own icon picker; tests replace it.
var showIconPicker = pickIconFromShell

// appIconFilter is iconFilter without SVG: the window and tray icons take
// PNG only, which a vector would have to be drawn at their size first.
func appIconFilter() []mygo.FileFilter {
	return []mygo.FileFilter{{Name: locales.IconFilesFilter(), Extensions: []string{
		"png", "jpg", "jpeg", "gif", "bmp", "webp", "ico", "exe", "dll",
	}}}
}

func (a *App) pickAppIcon(dir string) string {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: locales.SelectIconTitle(), DefaultPath: dir, Filters: appIconFilter()})
	if err != nil || len(paths) == 0 {
		return ""
	}
	return a.storedPick(paths[0])
}

func (a *App) pickBackground(dir string) string {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: locales.SelectFileTitle(), DefaultPath: dir, Filters: []mygo.FileFilter{{
		Name:       locales.ImageFilesFilter(),
		Extensions: []string{"png", "jpg", "jpeg", "gif", "bmp", "webp"},
	}}})
	if err != nil || len(paths) == 0 {
		return ""
	}
	return a.storedPick(paths[0])
}

func (a *App) pickDirectory(dir string) string {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: locales.SelectFolderTitle(), DefaultPath: dir, Directory: true})
	if err != nil || len(paths) == 0 {
		return ""
	}
	return a.storedPick(paths[0])
}

// storedPick keeps a picked path as the item fields should show it: absolute
// with "Abs path for new items" on, relative to the launcher otherwise.
func (a *App) storedPick(picked string) string {
	if picked == "" {
		return ""
	}
	if a.store.Settings.AbsolutePaths {
		return picked
	}
	return toStoredPath(picked)
}

// openDataDir shows the data folder.
func (a *App) openDataDir() {
	_ = os.MkdirAll(filepath.Join(dataBase(), dataDir), 0o755)
	_ = openPath(filepath.Join(dataBase(), dataDir))
}

// reveal shows an item in the file manager. An item without a file, or one
// whose file is gone, says so instead.
func (a *App) reveal(guid string) error {
	item := a.item(guid)
	if item == nil {
		return fmt.Errorf("invalid item guid %q", guid)
	}
	target := ""
	switch {
	case item.Path != "":
		target = resolveLaunchPath(item.Path)
	case item.WorkingDir != "":
		target = absPath(item.WorkingDir)
	}
	if !fileExists(target) {
		return errors.New(locales.PathMissing())
	}
	mygo.Shell.ShowItemInFolder(target)
	return nil
}

// convertPaths rewrites every item's paths, absolute or relative.
func (a *App) convertPaths(absolute bool) {
	for _, item := range a.store.Apps {
		convertItemPaths(item, absolute)
	}
	a.save()
	a.refresh()
}

func (a *App) convertItemPath(guid string, absolute bool) error {
	item := a.item(guid)
	if item == nil {
		return fmt.Errorf("invalid item guid %q", guid)
	}
	convertItemPaths(item, absolute)
	a.save()
	a.refresh()
	return nil
}

// setAlwaysOnTop applies the window setting and keeps it in the store.
func (a *App) setAlwaysOnTop(on bool) {
	a.store.Settings.AlwaysOnTop = on
	if a.win != nil {
		a.win.SetAlwaysOnTop(on)
	}
	a.save()
}

// iconImage returns an item's icon, decoded once and kept.
func (a *App) iconImage(item *AppItem) ui.ImageSource {
	if item == nil {
		return nil
	}
	if src := a.dynamicIcons[item.GUID]; src != nil {
		return src
	}
	if item.Icon == "" || !allowedIcon(item.Icon) {
		return nil
	}
	if src, ok := a.icons[item.Icon]; ok {
		return src
	}
	src := iconSource(item.Icon)
	a.icons[item.Icon] = src
	return src
}

// startSpecialPoll keeps the icons of special items up to date while they
// show: once when the window appears or the page comes forward, then every 5
// seconds while the window is visible.
func (a *App) startSpecialPoll() {
	if a.win == nil {
		return
	}
	a.refreshVisibleSpecials()
	a.win.OnShow(func() { a.refreshVisibleSpecials() })
	a.win.OnFocus(func() { a.refreshVisibleSpecials() })
	go func() {
		for range time.Tick(5 * time.Second) {
			a.dispatch(a.refreshVisibleSpecials)
		}
	}()
}

// refreshVisibleSpecials redraws the special icons of the page shown. It does
// nothing while the window is hidden or minimized, and never reads the shell
// for a page that is not shown.
func (a *App) refreshVisibleSpecials() {
	if a.win == nil || !a.win.IsVisible() || a.win.IsMinimized() || a.view == nil {
		return
	}
	tab := a.tab(a.view.prefs.ActiveTab)
	if tab == nil {
		return
	}
	for _, slot := range tab.Slots {
		if slot == nil {
			continue
		}
		if item := a.store.Apps[*slot]; item != nil {
			a.refreshSpecialIcon(item)
		}
	}
}

// startClock redraws every ten seconds while a runtime display changes, which
// is how a running process' time counts up, as the page's clock did.
func (a *App) startClock() {
	go func() {
		for range time.Tick(10 * time.Second) {
			if a.tick.Load() {
				a.refresh()
			}
		}
	}()
}
