package main

import (
	"testing"

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

// TestViewSwitchesToGrid 验证列表/网格切换，网格里的空槽也画出可放置的空格。
func TestViewSwitchesToGrid(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	app.store.Categories[0].Slots = append(app.store.Categories[0].Slots, nil) // 空槽
	tt.Frame()

	if err := tt.Click("Switch to grid view"); err != nil {
		t.Fatalf("switch to the grid: %v", err)
	}
	if !tt.HasText("Alpha") || !tt.HasText("Beta") {
		t.Fatalf("the grid must show every item, texts %q", tt.Texts())
	}
	if v.prefs.ViewMode != "grid" {
		t.Fatalf("the view mode must be remembered, got %q", v.prefs.ViewMode)
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

// TestViewAppMenuTogglesGameMode 验证窗口菜单里的开关写回 store。
func TestViewAppMenuTogglesGameMode(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	app.store.Settings.GameMode = true

	if err := clickMenu(tt, "Menu", "Game mode"); err != nil {
		t.Fatalf("game mode: %v", err)
	}
	if app.store.Settings.GameMode {
		t.Fatal("the game mode toggle must turn the setting off")
	}
	if err := clickMenu(tt, "Menu", "Game mode"); err != nil {
		t.Fatalf("game mode again: %v", err)
	}
	if !app.store.Settings.GameMode {
		t.Fatal("the game mode toggle must turn the setting back on")
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
