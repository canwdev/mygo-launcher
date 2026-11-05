package main

import (
	"strings"
	"testing"
	"time"

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
		"Settings", "Theme", "Accent color", "Auto timing", "Auto-hide the launcher",
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

// TestViewConfirmsDelete 验证删除条目要先确认，确认后从 store 与 tab 里都去掉。
func TestViewConfirmsDelete(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)

	if err := tt.RightClick("Alpha"); err != nil {
		t.Fatalf("right click on the row: %v", err)
	}
	if err := tt.Click("Delete"); err != nil {
		t.Fatalf("choose Delete: %v", err)
	}
	if !tt.HasText("Confirm") {
		t.Fatalf("deleting must ask first, texts %q", tt.Texts())
	}
	if app.store.Apps["a"] == nil {
		t.Fatal("the item must stay until the dialog is accepted")
	}
	if err := tt.Click("OK"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if app.store.Apps["a"] != nil {
		t.Fatal("accepting must delete the item")
	}
	for _, slot := range app.store.Categories[0].Slots {
		if slot != nil && *slot == "a" {
			t.Fatal("the deleted item must leave the tab's slots")
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
