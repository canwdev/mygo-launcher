package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/google/uuid"
)

// runningProc is a process the launcher started in game mode and tracks: its
// runtime accumulates until it exits, and it can be stopped.
type runningProc struct {
	start   time.Time
	wait    func() error
	stop    func() error
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

	win  *mygo.Window
	view *view

	watchCancel func()
}

func NewApp() *App {
	_ = os.MkdirAll(filepath.Join(absBase, dataDir), 0o755)
	app := &App{
		store:        loadStore(),
		runtimeStats: map[string]int64{},
		running:      map[string]*runningProc{},
		icons:        map[string]ui.ImageSource{},
	}
	for guid, item := range app.store.Apps {
		if item != nil {
			app.runtimeStats[guid] = item.RuntimeMs
		}
	}
	app.pruneIconFiles()
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
		a.notify("Saving failed: " + err.Error())
		return
	}
	a.fileMu.Lock()
	a.rememberStoreFile(data)
	a.fileMu.Unlock()
	a.refresh()
}

// refresh redraws the window after the store or a process changed; without a
// window, as in tests, the view is asked for the frame instead.
func (a *App) refresh() {
	if a.win != nil {
		a.win.Invalidate()
		return
	}
	if a.view != nil {
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

// minimize and restore act on the window when there is one, following the
// auto-hide setting.
func (a *App) minimize() {
	if a.win != nil {
		a.win.Minimize()
	}
}

func (a *App) restore() {
	if a.win != nil {
		a.win.Restore()
	}
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
// no Stop handle, no runtime.
func openFile(path string, args []string, workDir string, elevated bool) error {
	if isExecutable(path) || elevated {
		return startUntracked(path, args, workDir, elevated)
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
	track    bool // game mode accumulates the runtime of executables
	hide     bool // auto-hide minimizes the window
}

var errAlreadyRunning = errors.New("already running")

// planLaunch reads what running an item takes.
func (a *App) planLaunch(guid string) (launchPlan, error) {
	item := a.item(guid)
	if item == nil {
		return launchPlan{}, fmt.Errorf("invalid item guid %q", guid)
	}
	if _, ok := a.running[guid]; ok {
		return launchPlan{}, errAlreadyRunning
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
		return plan, nil
	}
	plan.path = resolveLaunchPath(item.Path)
	plan.track = a.store.Settings.GameMode && isExecutable(plan.path)
	return plan, nil
}

// planOpen reads what opening an item without tracking takes, as items that
// time themselves by hand are.
func (a *App) planOpen(guid string) (launchPlan, error) {
	item := a.item(guid)
	if item == nil {
		return launchPlan{}, fmt.Errorf("invalid item guid %q", guid)
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
		plan.path, plan.args, plan.elevated = plan.workDir, nil, false
		return plan, nil
	}
	plan.path = resolveLaunchPath(item.Path)
	return plan, nil
}

// runLaunch starts what a plan describes. It may block, as an elevation prompt
// does, so it runs on a goroutine of its own.
func (a *App) runLaunch(plan launchPlan) {
	if plan.path == "" {
		return
	}
	if plan.track {
		p := &runningProc{}
		if err := startTracked(plan.path, plan.args, plan.workDir, plan.elevated, p); err != nil {
			a.notifyErr(err)
			return
		}
		a.dispatch(func() {
			a.running[plan.guid] = p
			if plan.hide {
				a.minimize()
			}
		})
		a.watchExit(plan.guid, p, plan.hide)
		return
	}
	if err := openFile(plan.path, plan.args, plan.workDir, plan.elevated); err != nil {
		a.notifyErr(err)
		return
	}
	if plan.hide {
		a.minimize()
	}
}

// watchExit accumulates a tracked process' runtime once it ends, and brings the
// launcher back when it was the last one and auto-hide asked for it.
func (a *App) watchExit(guid string, p *runningProc, hide bool) {
	go func() {
		_ = p.wait()
		elapsed := time.Since(p.start).Milliseconds()
		a.dispatch(func() {
			a.runtimeStats[guid] += elapsed
			delete(a.running, guid)
			if p.cleanup != nil {
				p.cleanup()
			}
			a.save()
			if hide && len(a.running) == 0 {
				a.restore()
			}
		})
	}()
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

// setRuntimeMs overrides an item's accumulated runtime (the runtime dialog and
// the manual timer write through it). Negative input is clamped to 0.
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
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.win,
			Title:    "Add Files",
			Multiple: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.addPaths(paths, tabGUID)
	}()
}

// addPaths adds files, folders and shortcuts to a tab. Icons are extracted on a
// goroutine, since that reads the disk for every file.
func (a *App) addPaths(paths []string, tabGUID string) {
	if len(paths) == 0 {
		return
	}
	absolute := a.store.Settings.AbsolutePaths
	a.setBusy("Adding files…")
	go func() {
		items := make([]*AppItem, 0, len(paths))
		for _, raw := range paths {
			if item := buildItem(raw, absolute); item != nil {
				items = append(items, item)
			}
		}
		a.dispatch(func() {
			a.setBusy("")
			if len(items) == 0 {
				return
			}
			tab := a.tab(tabGUID)
			if tab == nil {
				return
			}
			for _, item := range items {
				a.store.Apps[item.GUID] = item
				a.runtimeStats[item.GUID] = 0
				guid := item.GUID
				tab.Slots = append(tab.Slots, &guid)
			}
			a.save()
		})
	}()
}

// buildItem makes an item for a path, resolving shortcuts and caching the icon.
// It reads the disk, so it runs off the main thread.
func buildItem(raw string, absolute bool) *AppItem {
	path := normalizePath(raw)
	if path == "" {
		return nil
	}
	name := defaultTitle(path)
	target, args, workDir, iconPath := path, "", filepath.Dir(path), path
	if isShortcutFile(path) {
		if lnk, err := resolveShortcut(path); err == nil && lnk.Target != "" {
			target, args, iconPath = lnk.Target, lnk.Args, lnk.Target
			workDir = lnk.WorkDir
			if workDir == "" {
				workDir = filepath.Dir(target)
			}
		}
	}
	return &AppItem{
		GUID:       uuid.NewString(),
		Name:       name,
		Path:       storedForm(target, absolute),
		WorkingDir: storedForm(workDir, absolute),
		Icon:       writeIcon(iconPath),
		Args:       args,
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
	if item.Path == "" {
		return errors.New("item has no path to extract an icon from")
	}
	icon := writeIcon(resolveLaunchPath(item.Path))
	if icon == "" {
		return errors.New("failed to regenerate icon")
	}
	a.replaceIcon(item, icon)
	return nil
}

func (a *App) replaceIcon(item *AppItem, icon string) {
	if old := item.Icon; old != "" {
		_ = os.Remove(absPath(old))
		delete(a.icons, old)
	}
	item.Icon = icon
	a.save()
	a.refresh()
}

func imageFilter() []mygo.FileFilter {
	return []mygo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "gif", "bmp", "webp", "svg"}}}
}

// pickFile asks for a file; dir is where the dialog starts.
func (a *App) pickFile(dir string) string {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "Select File", DefaultPath: dir})
	if err != nil || len(paths) == 0 {
		return ""
	}
	return a.storedPick(paths[0])
}

func (a *App) pickImage(dir string) string {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "Select Image", DefaultPath: dir, Filters: imageFilter()})
	if err != nil || len(paths) == 0 {
		return ""
	}
	return a.storedPick(paths[0])
}

func (a *App) pickDirectory(dir string) string {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "Select Folder", DefaultPath: dir, Directory: true})
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
	_ = os.MkdirAll(filepath.Join(absBase, dataDir), 0o755)
	_ = openPath(filepath.Join(absBase, dataDir))
}

// reveal shows an item in the file manager.
func (a *App) reveal(guid string) error {
	item := a.item(guid)
	if item == nil {
		return fmt.Errorf("invalid item guid %q", guid)
	}
	target := absPath(item.Path)
	if item.Path == "" && item.WorkingDir != "" {
		target = absPath(item.WorkingDir)
	}
	if target == "" {
		return nil
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
	if item == nil || item.Icon == "" {
		return nil
	}
	if src, ok := a.icons[item.Icon]; ok {
		return src
	}
	src := iconSource(item.Icon)
	a.icons[item.Icon] = src
	return src
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
