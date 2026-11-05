package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// newTestView builds the view of a test app and its window's tester, focused as
// a window the user just clicked is: the first click on an unfocused window
// only focuses it, and would otherwise be lost.
func newTestView(t *testing.T, width, height int) (*App, *view, *ui.Tester) {
	t.Helper()
	app := newTestApp(t)
	seedItems(t, app)
	v := newView(app)
	tt := ui.NewTester(v.view, width, height)
	tt.SetFocused(true)
	return app, v, tt
}

// seedItems puts two items in the store's first tab.
func seedItems(t *testing.T, app *App) {
	t.Helper()
	app.store.Apps["a"] = &AppItem{GUID: "a", Name: "Alpha", Path: "C:/tools/alpha.exe", RuntimeMs: 3_600_000}
	app.store.Apps["b"] = &AppItem{GUID: "b", Name: "Beta", Path: "C:/tools/beta.exe"}
	app.store.Categories[0].Name = "Default" // the store names it in the system's language
	app.store.Categories[0].Slots = []*string{ptrString("a"), ptrString("b")}
	app.runtimeStats["a"] = 3_600_000
	app.runtimeStats["b"] = 0
}

// clickMenu opens a popover menu by clicking its button and chooses the entry
// labeled item: MyGo draws the menu in the window, so its entries are elements
// the tester clicks.
func clickMenu(tt *ui.Tester, button, item string) error {
	if err := tt.Click(button); err != nil {
		return err
	}
	return tt.Click(item)
}

// newTestSettings drives the settings window's content headlessly: openSettings
// needs a real window, so the content is tested on its own.
func newTestSettings(t *testing.T, v *view) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(v.settingsView, 560, 720)
	tt.SetFocused(true)
	return tt
}

// TestViewShowsItems 验证列表视图把 tab 里的条目连同运行时一起画出来。
func TestViewShowsItems(t *testing.T) {
	_, _, tt := newTestView(t, 720, 460)

	if !tt.HasText("Alpha") || !tt.HasText("Beta") {
		t.Fatalf("the list must show every item, texts %q", tt.Texts())
	}
	if !tt.HasText("1h") {
		t.Fatalf("the runtime must show beside an item, texts %q", tt.Texts())
	}
	if !tt.HasText("Default") {
		t.Fatalf("the tab strip must show the tab's name, texts %q", tt.Texts())
	}
	if !tt.HasText("Run") {
		t.Fatalf("each item needs a Run button, texts %q", tt.Texts())
	}
}

// TestViewOpensItemEditor 走一遍条目的右键菜单：「Edit」打开编辑对话框，改名
// 保存后写回 store。
func TestViewOpensItemEditor(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)

	if err := tt.RightClick("Alpha"); err != nil {
		t.Fatalf("right click on the row: %v", err)
	}
	if err := tt.Click("Edit"); err != nil {
		t.Fatalf("choose Edit: %v", err)
	}
	if !tt.HasText("Edit item") {
		t.Fatalf("the item dialog must open, texts %q", tt.Texts())
	}
	if !tt.HasText("Run as administrator") || !tt.HasText("Working directory") {
		t.Fatalf("the dialog must show every field, texts %q", tt.Texts())
	}

	if err := tt.Click("Name"); err != nil {
		t.Fatalf("focus the name field: %v", err)
	}
	tt.Type("Renamed") // the field holds the item's name, so this appends
	if err := tt.Click("Save"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(tt.Texts()) == 0 {
		t.Fatal("the window must still draw after the dialog closed")
	}
	if got := app.store.Apps["a"].Name; got != "AlphaRenamed" {
		t.Fatalf("the dialog must write the item, name %q", got)
	}
	if tt.HasText("Edit item") {
		t.Fatalf("the dialog must close after saving, texts %q", tt.Texts())
	}
}

// TestViewSwitchesToGrid 验证顶栏的列表/网格切换，网格里的空槽也画出可放置的空格。
func TestViewSwitchesToGrid(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	app.store.Categories[0].Slots = append(app.store.Categories[0].Slots, nil) // 空槽
	tt.Frame()

	if err := tt.Click("Switch to grid view"); err != nil {
		t.Fatalf("switch to the grid: %v", err)
	}
	if v.prefs.ViewMode != "grid" {
		t.Fatalf("the view mode must be remembered, got %q", v.prefs.ViewMode)
	}
	if !tt.HasText("Alpha") || !tt.HasText("Beta") {
		t.Fatalf("the grid must show every item, texts %q", tt.Texts())
	}
	if err := tt.Click("Switch to list view"); err != nil {
		t.Fatalf("switch back to the list: %v", err)
	}
	if v.prefs.ViewMode != "list" {
		t.Fatalf("the view mode must go back to the list, got %q", v.prefs.ViewMode)
	}
}

// TestViewSearch 验证搜索浮层：打开、输入过滤、点击结果定位。
func TestViewSearch(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)

	if err := tt.Click("Search"); err != nil {
		t.Fatalf("open the search: %v", err)
	}
	if !tt.HasText("Type to search…") {
		t.Fatalf("the search must show its hint, texts %q", tt.Texts())
	}
	tt.Type("bet")
	if !tt.HasText("Beta") {
		t.Fatalf("the search must filter the items, texts %q", tt.Texts())
	}
	if hits := v.searchHits(); len(hits) != 1 || hits[0].guid != "b" {
		t.Fatalf("search hits = %+v", hits)
	}
	if app.store.Apps["b"] == nil {
		t.Fatal("the store must not change while searching")
	}
}

// TestViewTabsAddRename 验证标签的增、改名与切换。
func TestViewTabsAddRename(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)

	if err := tt.Click("Add tab"); err != nil {
		t.Fatalf("add a tab: %v", err)
	}
	if len(app.store.Categories) != 2 {
		t.Fatalf("adding a tab must append one, got %+v", app.store.Categories)
	}
	if !tt.HasText("New Tab") {
		t.Fatalf("the new tab must show in the strip, texts %q", tt.Texts())
	}

	if err := tt.RightClick("New Tab"); err != nil {
		t.Fatalf("open the tab menu: %v", err)
	}
	if err := tt.Click("Rename"); err != nil {
		t.Fatalf("choose Rename: %v", err)
	}
	if !tt.HasText("Rename Tab") {
		t.Fatalf("the rename dialog must open, texts %q", tt.Texts())
	}
	tt.Type(" 2")
	if err := tt.Click("OK"); err != nil {
		t.Fatalf("confirm the rename: %v", err)
	}
	if !tt.HasText("New Tab 2") {
		t.Fatalf("the renamed tab must show, texts %q", tt.Texts())
	}
	if app.store.Categories[1].Name != "New Tab 2" {
		t.Fatalf("the rename must be saved, got %q", app.store.Categories[1].Name)
	}
}

// TestViewSettingsChoosesLanguage 验证语言下拉框把选择写进 ui-state.json；
// 无头环境没有应用语言可改，所以只检查偏好。
func TestViewSettingsChoosesLanguage(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	settings := newTestSettings(t, v)

	if err := settings.Click("English"); err != nil {
		t.Fatalf("open the language dropdown: %v", err)
	}
	if err := settings.Click("简体中文"); err != nil {
		t.Fatalf("choose 简体中文: %v", err)
	}
	if v.prefs.Language != "zh-Hans" {
		t.Fatalf("the language must be remembered, got %q", v.prefs.Language)
	}
	if got := loadPrefs().Language; got != "zh-Hans" {
		t.Fatalf("the language must be saved, got %q", got)
	}

	settings.Frame()
	if err := settings.Click("简体中文"); err != nil {
		t.Fatalf("open the language dropdown again: %v", err)
	}
	if err := settings.Click("English"); err != nil {
		t.Fatalf("choose English: %v", err)
	}
	if v.prefs.Language != "en" {
		t.Fatalf("the language must change, got %q", v.prefs.Language)
	}
}

// TestViewSettingsTogglesBackgroundOnClose 验证「关闭后在后台运行」默认开启，开关
// 写回 store。
func TestViewSettingsTogglesBackgroundOnClose(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)
	if !app.store.Settings.BackgroundOnClose {
		t.Fatal("running in the background must be on by default")
	}
	settings := newTestSettings(t, v)

	if err := settings.Click("Toggle run in background"); err != nil {
		t.Fatalf("run in background: %v", err)
	}
	if app.store.Settings.BackgroundOnClose {
		t.Fatal("the toggle must turn running in the background off")
	}
	if loadStore().Settings.BackgroundOnClose {
		t.Fatal("the setting must be saved")
	}
	if err := settings.Click("Toggle run in background"); err != nil {
		t.Fatalf("run in background again: %v", err)
	}
	if !app.store.Settings.BackgroundOnClose {
		t.Fatal("the toggle must turn running in the background back on")
	}
}

// TestViewCloseHidesWhenRunningInBackground 验证关闭窗口时按设置隐藏到托盘，
// 退出应用时照常关闭。
func TestViewCloseHidesWhenRunningInBackground(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)

	e := &mygo.CloseEvent{}
	v.handleClose(e)
	if !e.DefaultPrevented() {
		t.Fatal("closing the window must hide it while the app runs in the background")
	}

	app.store.Settings.BackgroundOnClose = false
	e = &mygo.CloseEvent{}
	v.handleClose(e)
	if e.DefaultPrevented() {
		t.Fatal("with running in the background off the window must close")
	}

	app.store.Settings.BackgroundOnClose = true
	app.quitting = true
	e = &mygo.CloseEvent{}
	v.handleClose(e)
	if e.DefaultPrevented() {
		t.Fatal("a quit must close the window, not hide it")
	}
}

// TestViewSettingsTogglesAutoHide 验证设置窗口里的开关写回 store。
func TestViewSettingsTogglesAutoHide(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)
	app.store.Settings.AutoHide = true
	settings := newTestSettings(t, v)

	if err := settings.Click("Toggle auto-hide"); err != nil {
		t.Fatalf("auto-hide: %v", err)
	}
	if app.store.Settings.AutoHide {
		t.Fatal("the auto-hide toggle must turn the setting off")
	}
	if err := settings.Click("Toggle auto-hide"); err != nil {
		t.Fatalf("auto-hide again: %v", err)
	}
	if !app.store.Settings.AutoHide {
		t.Fatal("the auto-hide toggle must turn the setting back on")
	}
}

// TestViewRuntimeDialogAdjustsByHour 验证 Edit runtime 的 -1h / +1h 按钮直接改
// 写已保存的时长，且不会减到负数。
func TestViewRuntimeDialogAdjustsByHour(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)

	v.runtimeGUID = "a"
	v.runtimeOpen = true
	tt.Frame()
	if !tt.HasText("-1h") || !tt.HasText("+1h") {
		t.Fatalf("the dialog must show both buttons, texts %q", tt.Texts())
	}

	if err := tt.Click("+1h"); err != nil {
		t.Fatalf("add an hour: %v", err)
	}
	if got := app.store.Apps["a"].RuntimeMs; got != 7_200_000 {
		t.Fatalf("+1h must add an hour to the stored runtime, got %dms", got)
	}
	if v.runtimeMinutes != "120" {
		t.Fatalf("the minutes field must follow the buttons, got %q", v.runtimeMinutes)
	}
	if err := tt.Click("-1h"); err != nil {
		t.Fatalf("subtract an hour: %v", err)
	}
	if got := app.store.Apps["a"].RuntimeMs; got != 3_600_000 {
		t.Fatalf("-1h must take an hour off, got %dms", got)
	}

	app.runtimeStats["a"] = 60_000
	app.store.Apps["a"].RuntimeMs = 60_000
	v.states = app.state()
	v.adjustRuntime(-time.Hour)
	if got := app.store.Apps["a"].RuntimeMs; got != 0 {
		t.Fatalf("the runtime must not go below zero, got %dms", got)
	}
}

// TestViewSettingsTogglesAutoTiming 验证设置窗口里的自动计时开关写回 store。
func TestViewSettingsTogglesAutoTiming(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)
	app.store.Settings.AutoTime = true
	settings := newTestSettings(t, v)

	if err := settings.Click("Toggle auto timing"); err != nil {
		t.Fatalf("auto timing: %v", err)
	}
	if app.store.Settings.AutoTime {
		t.Fatal("the auto timing toggle must turn the setting off")
	}
	if err := settings.Click("Toggle auto timing"); err != nil {
		t.Fatalf("auto timing again: %v", err)
	}
	if !app.store.Settings.AutoTime {
		t.Fatal("the auto timing toggle must turn the setting back on")
	}
}

// TestViewSettingsShowsItsChoices 验证设置窗口画出了分组与版本，并且主窗口的
// Settings 入口存在。
func TestViewSettingsShowsItsChoices(t *testing.T) {
	_, v, tt := newTestView(t, 720, 460)
	if !tt.HasText("Settings") {
		t.Fatalf("the window needs a Settings entry, texts %q", tt.Texts())
	}
	if err := tt.Click("Settings"); err != nil {
		t.Fatalf("click the settings entry: %v", err)
	}

	settings := newTestSettings(t, v)
	for _, text := range []string{
		"Settings", "Language", "Theme", "Accent color", "Run in background",
		"Auto timing", "Auto-hide the launcher",
		"Always on top", "Absolute paths for new items", "Data directory",
		"Version " + appVersion,
	} {
		if !settings.HasText(text) {
			t.Fatalf("the settings must show %q, texts %q", text, settings.Texts())
		}
	}
}

// TestViewSettingsChoosesAccent 验证默认是 --bang-1，明暗各有其色，并且色板能换成
// --bang-2。
func TestViewSettingsChoosesAccent(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	def, ok := presetColor(defaultAccent, false)
	if !ok {
		t.Fatalf("no %s preset", defaultAccent)
	}
	if got := accentColor("", false); got != def {
		t.Fatalf("the default accent must be %s, got %v", defaultAccent, got)
	}
	if accentColor(defaultAccent, false) == accentColor(defaultAccent, true) {
		t.Fatal("the presets must have a light and a dark color")
	}

	settings := newTestSettings(t, v)
	if err := settings.Click("Anon"); err != nil {
		t.Fatalf("choose the second accent: %v", err)
	}
	if v.prefs.Accent != "bang-2" {
		t.Fatalf("the accent must be remembered, got %q", v.prefs.Accent)
	}
	if got, want := accentColor(v.prefs.Accent, false), accentColor("bang-2", false); got != want {
		t.Fatalf("the chosen accent must apply, got %v want %v", got, want)
	}
}

// fakeUpdater swaps the updater plugin's preferences for ones in memory, until
// the returned function puts them back: the plugin panics without useUpdater.
func fakeUpdater(checks, downloads *bool, checked *int, last *time.Time, available *string) func() {
	check, getChecks, setChecks := checkForUpdates, updateChecks, setUpdateChecks
	getDownloads, setDownloads := updateDownloads, setUpdateDownloads
	lastFn, availableFn := lastUpdateCheck, availableUpdate
	checkForUpdates = func() { *checked++ }
	updateChecks = func() bool { return *checks }
	setUpdateChecks = func(on bool) { *checks = on }
	updateDownloads = func() bool { return *downloads }
	setUpdateDownloads = func(on bool) { *downloads = on }
	lastUpdateCheck = func() time.Time { return *last }
	availableUpdate = func() string { return *available }
	return func() {
		checkForUpdates, updateChecks, setUpdateChecks = check, getChecks, setChecks
		updateDownloads, setUpdateDownloads = getDownloads, setDownloads
		lastUpdateCheck, availableUpdate = lastFn, availableFn
	}
}

// TestSettingsUpdates 验证关于分组里的更新开关、手动检查和状态文字。
func TestSettingsUpdates(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)
	app.updates = true
	checks, downloads, checked := true, false, 0
	var last time.Time
	available := ""
	defer fakeUpdater(&checks, &downloads, &checked, &last, &available)()

	tt := ui.NewTester(v.settingsView, 560, 1600)
	tt.SetFocused(true)
	tt.Frame()
	if !tt.HasText("Not checked yet") {
		t.Fatalf("the about section must say no check ran, texts %q", tt.Texts())
	}
	if err := tt.Click("Check for updates"); err != nil {
		t.Fatalf("check for updates: %v", err)
	}
	if checked != 1 {
		t.Fatalf("the button must check once, got %d", checked)
	}

	if err := tt.Click("Toggle automatic update checks"); err != nil {
		t.Fatalf("turn checks off: %v", err)
	}
	if checks {
		t.Fatal("automatic checks must be off")
	}
	if err := tt.Click("Toggle automatic update checks"); err != nil {
		t.Fatalf("turn checks on: %v", err)
	}
	if !checks {
		t.Fatal("automatic checks must be on")
	}
	if err := tt.Click("Toggle automatic update downloads"); err != nil {
		t.Fatalf("turn downloads on: %v", err)
	}
	if !downloads {
		t.Fatal("automatic downloads must be on")
	}

	available = "0.3.0"
	tt.Frame()
	if !tt.HasText("Version 0.3.0 is available") {
		t.Fatalf("the about section must show the new version, texts %q", tt.Texts())
	}
	available = ""
	last = time.Now()
	tt.Frame()
	if !tt.HasText("Last checked") {
		t.Fatalf("the about section must show the last check, texts %q", tt.Texts())
	}
}

// TestAutoTimingGatesRuntime 验证关掉自动计时后，运行中的进程不再显示增量。
func TestAutoTimingGatesRuntime(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)
	app.running["a"] = &runningProc{start: time.Now().Add(-time.Hour)}
	v.states = app.state()

	if got := v.runtimeText("a"); !strings.Contains(got, "(+ ") {
		t.Fatalf("the running time must show while auto timing is on, got %q", got)
	}
	app.store.Settings.AutoTime = false
	if got := v.runtimeText("a"); got != "1h" {
		t.Fatalf("auto timing off must leave the runtime alone, got %q", got)
	}
}

// TestFinishProcessRespectsAutoTiming 验证进程结束时是否把这段时长记进去。
func TestFinishProcessRespectsAutoTiming(t *testing.T) {
	app := newTestApp(t)
	app.store.Apps["a"] = &AppItem{GUID: "a", Name: "A"}
	app.runtimeStats["a"] = 5_000
	app.running["a"] = &runningProc{}
	app.store.Settings.AutoTime = false

	app.finishProcess("a", &runningProc{}, time.Hour.Milliseconds())
	if _, ok := app.running["a"]; ok {
		t.Fatal("the finished process must no longer be tracked")
	}
	if got := app.runtimeStats["a"]; got != 5_000 {
		t.Fatalf("with auto timing off the runtime must not move, got %dms", got)
	}

	app.running["a"] = &runningProc{}
	app.store.Settings.AutoTime = true
	app.finishProcess("a", &runningProc{}, 60_000)
	if got := app.runtimeStats["a"]; got != 65_000 {
		t.Fatalf("with auto timing on the time must be added, got %dms", got)
	}
}

// TestViewConfirmsRemove 验证移除条目要先确认，确认后从 store 与 tab 里都去掉。
func TestViewConfirmsRemove(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)

	if err := tt.RightClick("Alpha"); err != nil {
		t.Fatalf("right click on the row: %v", err)
	}
	if err := tt.Click("Remove"); err != nil {
		t.Fatalf("choose Remove: %v", err)
	}
	if !tt.HasText("Confirm") {
		t.Fatalf("removing must ask first, texts %q", tt.Texts())
	}
	if app.store.Apps["a"] == nil {
		t.Fatal("the item must stay until the dialog is accepted")
	}
	if err := tt.Click("OK"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if app.store.Apps["a"] != nil {
		t.Fatal("accepting must remove the item")
	}
	for _, slot := range app.store.Categories[0].Slots {
		if slot != nil && *slot == "a" {
			t.Fatal("the removed item must leave the tab's slots")
		}
	}
}

// TestViewShowsChineseTexts 验证窗口按语言画出中文文案：MyGo 的原生测试器把语言
// 设成 zh-Hans 后，按钮和标题都跟着换。
func TestViewShowsChineseTexts(t *testing.T) {
	_, v, tt := newTestView(t, 720, 460)
	tt.SetLocale("zh-Hans")

	if !tt.HasText("运行") || !tt.HasText("设置") || !tt.HasText("搜索") {
		t.Fatalf("the window must say its texts in Chinese, texts %q", tt.Texts())
	}
	if tt.HasText("Run") {
		t.Fatalf("the English texts must be gone, texts %q", tt.Texts())
	}

	settings := newTestSettings(t, v)
	settings.SetLocale("zh-Hans")
	for _, text := range []string{"设置", "外观", "语言", "行为", "路径", "数据", "关于"} {
		if !settings.HasText(text) {
			t.Fatalf("the settings must say %q in Chinese, texts %q", text, settings.Texts())
		}
	}
}

// drag presses at from, moves in steps to to, and releases there: the gesture
// that drags a row, a cell or a tab.
func drag(tt *ui.Tester, from, to ui.Rect) {
	x1, y1 := from.X+from.W/2, from.Y+from.H/2
	x2, y2 := to.X+to.W/2, to.Y+to.H/2
	tt.Press(x1, y1)
	for i := 1; i <= 8; i++ {
		tt.Move(x1+(x2-x1)*float32(i)/8, y1+(y2-y1)*float32(i)/8)
	}
	tt.Release(x2, y2)
}

// tabSlots lists a tab's slots, "empty" for a nil one.
func tabSlots(app *App, index int) []string {
	out := make([]string, 0, len(app.store.Categories[index].Slots))
	for _, slot := range app.store.Categories[index].Slots {
		if slot == nil {
			out = append(out, "empty")
			continue
		}
		out = append(out, *slot)
	}
	return out
}

// TestViewListDragReorders 验证列表里拖动一行把条目换位。
func TestViewListDragReorders(t *testing.T) {
	app, _, tt := newTestView(t, 720, 520)

	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha row")
	}
	beta, ok := tt.Find("Beta")
	if !ok {
		t.Fatal("no Beta row")
	}
	drag(tt, alpha, beta)

	if got := tabSlots(app, 0); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("slots = %v, want the dragged item first", got)
	}
}

// TestViewGridDragReorders 验证网格里拖动一格把条目换位，空槽留在原处。
func TestViewGridDragReorders(t *testing.T) {
	app, v, tt := newTestView(t, 720, 520)
	app.store.Categories[0].Slots = append(app.store.Categories[0].Slots, nil)
	v.prefs.ViewMode = "grid"
	tt.Frame()

	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha cell")
	}
	beta, ok := tt.Find("Beta")
	if !ok {
		t.Fatal("no Beta cell")
	}
	drag(tt, alpha, beta)

	if got := tabSlots(app, 0); len(got) != 3 || got[0] != "b" || got[1] != "a" || got[2] != "empty" {
		t.Fatalf("slots = %v, want the dragged item first and the empty cell last", got)
	}
}

// TestViewDragsItemToTab 验证把列表里的一行拖到另一个标签上，条目就搬过去。
func TestViewDragsItemToTab(t *testing.T) {
	app, _, tt := newTestView(t, 720, 520)
	if err := tt.Click("Add tab"); err != nil {
		t.Fatalf("add a tab: %v", err)
	}
	app.store.Categories[1].Name = "Second"
	if err := tt.Click("Default"); err != nil {
		t.Fatalf("show the first tab: %v", err)
	}
	tt.Frame()

	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha row")
	}
	chip, ok := tt.Find("Second")
	if !ok {
		t.Fatal("no Second tab")
	}
	drag(tt, alpha, chip)

	if got := tabSlots(app, 0); len(got) != 1 || got[0] != "b" {
		t.Fatalf("the first tab = %v, want only the other item", got)
	}
	if got := tabSlots(app, 1); len(got) != 1 || got[0] != "a" {
		t.Fatalf("the second tab = %v, want the dragged item", got)
	}
}

// TestViewDropsFiles 验证把文件从资源管理器拖到窗口，条目加入当前标签。
func TestViewDropsFiles(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	path := filepath.Join(t.TempDir(), "gamma.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !tt.DropFiles(360, 300, path) {
		t.Fatal("the window must take dropped files")
	}
	if got := tabSlots(app, 0); len(got) != 3 {
		t.Fatalf("slots = %v, want the dropped file appended", got)
	}
	item := app.store.Apps[*app.store.Categories[0].Slots[2]]
	if item == nil || item.Name != "gamma" {
		t.Fatalf("dropped item = %+v", item)
	}
}

// TestViewDropsFilesOnTab 验证拖到某个标签上的文件加入该标签。
func TestViewDropsFilesOnTab(t *testing.T) {
	app, _, tt := newTestView(t, 900, 520)
	if err := tt.Click("Add tab"); err != nil {
		t.Fatalf("add a tab: %v", err)
	}
	app.store.Categories[1].Name = "Second"
	tt.Frame()
	chip, ok := tt.Find("Second")
	if !ok {
		t.Fatal("no Second tab")
	}

	path := filepath.Join(t.TempDir(), "delta.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !tt.DropFiles(chip.X+chip.W/2, chip.Y+chip.H/2, path) {
		t.Fatal("the tab must take dropped files")
	}
	if got := tabSlots(app, 1); len(got) != 1 {
		t.Fatalf("second tab = %v, want the dropped file", got)
	}
	if got := tabSlots(app, 0); len(got) != 2 {
		t.Fatalf("first tab = %v, want it unchanged", got)
	}
	item := app.store.Apps[*app.store.Categories[1].Slots[0]]
	if item == nil || item.Name != "delta" {
		t.Fatalf("dropped item = %+v", item)
	}
}

// TestViewTabDragReorders 验证拖动标签改它们的顺序。
func TestViewTabDragReorders(t *testing.T) {
	app, _, tt := newTestView(t, 720, 520)
	if err := tt.Click("Add tab"); err != nil {
		t.Fatalf("add a tab: %v", err)
	}
	app.store.Categories[1].Name = "Second"
	tt.Frame()

	first, ok := tt.Find("Default")
	if !ok {
		t.Fatal("no Default tab")
	}
	second, ok := tt.Find("Second")
	if !ok {
		t.Fatal("no Second tab")
	}
	drag(tt, second, first)

	if app.store.Categories[0].Name != "Second" || app.store.Categories[1].Name != "Default" {
		t.Fatalf("tabs = %q, %q, want the dragged tab first",
			app.store.Categories[0].Name, app.store.Categories[1].Name)
	}
}
