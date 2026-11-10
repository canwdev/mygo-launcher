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

// scrollSettingsToBehavior brings the lower part of the settings, where the
// behavior switches are, into the window.
func scrollSettingsToBehavior(settings *ui.Tester) {
	settings.Scroll(280, 500, 0, 400)
}

// TestViewShowsItems 验证网格把 tab 里的条目画出来。
func TestViewShowsItems(t *testing.T) {
	_, _, tt := newTestView(t, 720, 460)

	if !tt.HasText("Alpha") || !tt.HasText("Beta") {
		t.Fatalf("the list must show every item, texts %q", tt.Texts())
	}
	if !tt.HasText("Default") {
		t.Fatalf("the tab strip must show the tab's name, texts %q", tt.Texts())
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

// TestViewMarksElevatedItems 验证以管理员身份运行的条目在图标左下角显示盾牌，
// 不再放在名称前面。
func TestViewMarksElevatedItems(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	app.store.Apps["a"].RunAsAdmin = true
	tt.Frame()

	name, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha cell")
	}
	badge, ok := tt.Access("Run as administrator")
	if !ok {
		t.Fatalf("an elevated item must show the shield, texts %q", tt.Texts())
	}
	if badge.Bounds.Y+badge.Bounds.H > name.Y {
		t.Fatalf("the shield must sit above the name, badge %+v name %+v", badge.Bounds, name)
	}
	if badge.Bounds.X+badge.Bounds.W/2 >= name.X+name.W/2 {
		t.Fatalf("the shield must sit at the icon's left, badge %+v name %+v", badge.Bounds, name)
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

// TestViewTabsAddRename 验证页面的增、改名与切换。
func TestViewTabsAddRename(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)

	if err := tt.Click("Add page"); err != nil {
		t.Fatalf("add a page: %v", err)
	}
	if len(app.store.Categories) != 2 {
		t.Fatalf("adding a page must append one, got %+v", app.store.Categories)
	}
	if !tt.HasText("New Page") {
		t.Fatalf("the new page must show in the strip, texts %q", tt.Texts())
	}

	if err := tt.RightClick("New Page"); err != nil {
		t.Fatalf("open the page menu: %v", err)
	}
	if err := tt.Click("Rename"); err != nil {
		t.Fatalf("choose Rename: %v", err)
	}
	if !tt.HasText("Rename Page") {
		t.Fatalf("the rename dialog must open, texts %q", tt.Texts())
	}
	tt.Type(" 2")
	if err := tt.Click("OK"); err != nil {
		t.Fatalf("confirm the rename: %v", err)
	}
	if !tt.HasText("New Page 2") {
		t.Fatalf("the renamed page must show, texts %q", tt.Texts())
	}
	if app.store.Categories[1].Name != "New Page 2" {
		t.Fatalf("the rename must be saved, got %q", app.store.Categories[1].Name)
	}
}

// TestViewLastPageCannotBeRemoved 验证只剩一个页面时菜单里没有「Remove」入口，
// 即使直接请求也不会删掉它。
func TestViewLastPageCannotBeRemoved(t *testing.T) {
	t.Run("only page", func(t *testing.T) {
		app, v, tt := newTestView(t, 720, 460)

		v.requestDeleteTab(app.store.Categories[0].GUID, "Default")
		if len(app.store.Categories) != 1 {
			t.Fatalf("the only page must stay, got %d pages", len(app.store.Categories))
		}
		if err := tt.RightClick("Default"); err != nil {
			t.Fatalf("open the page menu: %v", err)
		}
		if tt.HasText("Remove") {
			t.Fatalf("the only page must not offer Remove, texts %q", tt.Texts())
		}
	})
	t.Run("another page", func(t *testing.T) {
		_, v, tt := newTestView(t, 720, 460)
		v.addTab()
		tt.Frame()

		if err := tt.RightClick("Default"); err != nil {
			t.Fatalf("open the page menu: %v", err)
		}
		if !tt.HasText("Remove") {
			t.Fatalf("a page beside another must offer Remove, texts %q", tt.Texts())
		}
	})
}

// TestItemMenuInsertsBlank 验证条目菜单提供「插入空白」。
func TestItemMenuInsertsBlank(t *testing.T) {
	_, _, tt := newTestView(t, 720, 460)

	if err := tt.RightClick("Alpha"); err != nil {
		t.Fatalf("open the item menu: %v", err)
	}
	if !tt.HasText("Insert blank") {
		t.Fatalf("a cell must offer Insert blank, texts %q", tt.Texts())
	}
}

// TestItemMenuReportsMissingFileActions 验证路径为空或不存在时，点「打开所在文件夹」
// 或「更新图标」会提示不可用，而不是没有反应。
func TestItemMenuReportsMissingFileActions(t *testing.T) {
	const missing = "This item's file or folder does not exist"
	_, v, tt := newTestView(t, 720, 460)

	if err := tt.RightClick("Alpha"); err != nil {
		t.Fatalf("open the item menu: %v", err)
	}
	if err := tt.Click("Open folder…"); err != nil {
		t.Fatalf("choose Open folder: %v", err)
	}
	if !tt.HasText(missing) {
		t.Fatalf("a missing file must be reported, texts %q", tt.Texts())
	}

	tt = ui.NewTester(v.view, 720, 460)
	tt.SetFocused(true)
	if err := tt.RightClick("Alpha"); err != nil {
		t.Fatalf("open the item menu again: %v", err)
	}
	if err := tt.Click("Edit"); err != nil {
		t.Fatalf("open the editor: %v", err)
	}
	if err := clickMenu(tt, "More actions", "Update icon"); err != nil {
		t.Fatalf("choose Update icon: %v", err)
	}
	if !tt.HasText(missing) {
		t.Fatalf("updating a missing file must be reported, texts %q", tt.Texts())
	}
}

// TestDialogsFitNarrowWindow 验证窗口很窄时，对话框的输入框和按钮仍留在窗口内。
func TestDialogsFitNarrowWindow(t *testing.T) {
	const width float32 = 420
	fit := func(t *testing.T, tt *ui.Tester, labels ...string) {
		t.Helper()
		for _, label := range labels {
			info, ok := tt.Access(label)
			if !ok {
				t.Fatalf("%q must show in the dialog, texts %q", label, tt.Texts())
			}
			if info.Bounds.X < 0 || info.Bounds.X+info.Bounds.W > width {
				t.Fatalf("%q must stay inside the window, bounds %+v", label, info.Bounds)
			}
		}
	}

	t.Run("item", func(t *testing.T) {
		_, v, tt := newTestView(t, int(width), 460)
		v.openEdit("a")
		tt.Frame()
		fit(t, tt, "Browse…", "Clear", "Cancel", "Save")
	})
	t.Run("rename", func(t *testing.T) {
		_, v, tt := newTestView(t, int(width), 460)
		v.openItemRename("a", "Alpha")
		tt.Frame()
		fit(t, tt, "Cancel", "OK")
	})
	t.Run("runtime", func(t *testing.T) {
		_, v, tt := newTestView(t, int(width), 460)
		v.openRuntime("a")
		tt.Frame()
		fit(t, tt, "Cancel", "Save")
	})
	t.Run("search", func(t *testing.T) {
		_, v, tt := newTestView(t, int(width), 460)
		v.openSearch()
		tt.Frame()
		field, ok := tt.AccessByRole(ui.RoleTextField, "")
		if !ok {
			t.Fatalf("the search must show its field, texts %q", tt.Texts())
		}
		if field.Bounds.X < 0 || field.Bounds.X+field.Bounds.W > width {
			t.Fatalf("the search field must stay inside the window, bounds %+v", field.Bounds)
		}
	})
}

// TestViewRunReportsMissingPath 验证启动条目时先检查路径，失效时提示。
func TestViewRunReportsMissingPath(t *testing.T) {
	_, v, tt := newTestView(t, 720, 460)

	v.runItem("a")
	tt.Frame()
	if !tt.HasText("This item's file or folder does not exist") {
		t.Fatalf("a missing path must be reported before the launch, texts %q", tt.Texts())
	}
}

// TestViewRuntimeFieldIsClickable 验证网格单元左上角的时长可以点开编辑。
func TestViewRuntimeFieldIsClickable(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	app.store.Settings.AutoTime = true
	tt.Frame()

	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha cell")
	}
	tt.Move(alpha.X+alpha.W/2, alpha.Y+alpha.H/2)
	tt.Frame()
	runtime, ok := tt.Access("1h")
	if !ok {
		t.Fatalf("the hovered cell must show its runtime, texts %q", tt.Texts())
	}
	tt.ClickAt(runtime.Bounds.X+runtime.Bounds.W/2, runtime.Bounds.Y+runtime.Bounds.H/2)
	tt.Frame()
	if !tt.HasText("Edit runtime") {
		t.Fatalf("a click on the runtime must edit it, texts %q", tt.Texts())
	}
}

// TestViewHidesRuntimeForNonPrograms 验证文件夹、文件和 shell 目标不显示计时器：只
// 有会被跟踪启动的程序才有运行时长，它们的旧时长也不画。
func TestViewHidesRuntimeForNonPrograms(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	dir := filepath.Join(absBase, "Folder")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	app.store.Apps = map[string]*AppItem{
		"dir":  {GUID: "dir", Name: "Folder", Path: dir, RuntimeMs: 7_200_000},
		"file": {GUID: "file", Name: "Note", Path: filepath.Join(absBase, "note.txt")},
		"url":  {GUID: "url", Name: "Site", Path: "https://example.com"},
		"prog": {GUID: "prog", Name: "Tool", Path: "C:/tools/tool.exe", RuntimeMs: 3_600_000},
	}
	app.store.Categories[0].Slots = []*string{
		ptrString("dir"), ptrString("file"), ptrString("url"), ptrString("prog"),
	}
	app.store.Settings.AutoTime = true
	app.runtimeStats["dir"], app.runtimeStats["prog"] = 7_200_000, 3_600_000
	tt.Frame()

	hover := func(name string) {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("no cell %q, texts %q", name, tt.Texts())
		}
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Frame()
	}

	for _, name := range []string{"Folder", "Note", "Site"} {
		hover(name)
		if tt.HasText("2h") || tt.HasText("1h") || tt.HasText("--") {
			t.Fatalf("cell %q must show no timer, texts %q", name, tt.Texts())
		}
	}

	hover("Tool")
	if !tt.HasText("1h") {
		t.Fatalf("a program's cell must keep its timer, texts %q", tt.Texts())
	}
}

// TestViewOffersUpdateIconOutsideFolders 验证路径外的图标可以从菜单「更新图标」
// 收进来，已经自定义的图标则没有这一项。
func TestViewOffersUpdateIconOutsideFolders(t *testing.T) {
	t.Run("outside", func(t *testing.T) {
		app, _, tt := newTestView(t, 720, 460)
		app.store.Apps["a"].Icon = `C:\pictures\a.png`
		tt.Frame()
		if err := tt.RightClick("Alpha"); err != nil {
			t.Fatalf("open the item menu: %v", err)
		}
		if err := tt.Click("Edit"); err != nil {
			t.Fatalf("open the editor: %v", err)
		}
		if err := tt.Click("More actions"); err != nil {
			t.Fatalf("open the actions menu: %v", err)
		}
		if !tt.HasText("Update icon") {
			t.Fatalf("an icon outside the folders must offer Update icon, texts %q", tt.Texts())
		}
	})
	t.Run("custom", func(t *testing.T) {
		app, _, tt := newTestView(t, 720, 460)
		app.store.Apps["a"].Icon = "./" + customIconsDir + "/a.png"
		tt.Frame()
		if err := tt.RightClick("Alpha"); err != nil {
			t.Fatalf("open the item menu: %v", err)
		}
		if err := tt.Click("Edit"); err != nil {
			t.Fatalf("open the editor: %v", err)
		}
		if err := tt.Click("More actions"); err != nil {
			t.Fatalf("open the actions menu: %v", err)
		}
		if tt.HasText("Update icon") {
			t.Fatalf("a custom icon needs no update entry, texts %q", tt.Texts())
		}
	})
}

// TestViewShowsIconIndexForPrograms 验证图标是程序或 DLL 时才显示序号输入框。
func TestViewShowsIconIndexForPrograms(t *testing.T) {
	_, v, tt := newTestView(t, 720, 460)
	v.openEdit("a")
	tt.Frame()
	if tt.HasText("Index") {
		t.Fatalf("an image icon needs no index, texts %q", tt.Texts())
	}
	v.editIcon = `C:\Windows\System32\shell32.dll`
	tt.Frame()
	if !tt.HasText("Index") {
		t.Fatalf("a DLL icon needs an index field, texts %q", tt.Texts())
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
	scrollSettingsToBehavior(settings)

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

// TestViewSettingsTogglesGPU 验证设置里的 GPU 绘制开关写回 ui-state.json，默认关闭。
func TestViewSettingsTogglesGPU(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	if v.prefs.GPU {
		t.Fatal("drawing in memory must be the default")
	}
	settings := newTestSettings(t, v)
	scrollSettingsToBehavior(settings)
	if err := settings.Click("Toggle GPU drawing"); err != nil {
		t.Fatalf("toggle GPU drawing: %v", err)
	}
	if !v.prefs.GPU || !loadPrefs().GPU {
		t.Fatal("the GPU preference must be kept")
	}
}

// TestViewSettingsTogglesWheelPages 验证滚轮切换页面默认开启，开关写回
// ui-state.json。
func TestViewSettingsTogglesWheelPages(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	if !v.prefs.WheelPages {
		t.Fatal("the wheel must switch pages by default")
	}
	settings := newTestSettings(t, v)
	if err := settings.Click("Toggle wheel page switching"); err != nil {
		t.Fatalf("toggle wheel page switching: %v", err)
	}
	if v.prefs.WheelPages || loadPrefs().WheelPages {
		t.Fatal("turning it off must be kept")
	}
	if err := settings.Click("Toggle wheel page switching"); err != nil {
		t.Fatalf("toggle wheel page switching again: %v", err)
	}
	if !v.prefs.WheelPages || !loadPrefs().WheelPages {
		t.Fatal("turning it back on must be kept")
	}
}

// TestWheelSwitchesPages 验证开启后滚轮上下切换页面，并绕过网格滚动。
func TestWheelSwitchesPages(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	first := app.store.Categories[0].GUID
	app.store.Categories = append(app.store.Categories, CategoryNode{GUID: "second", Name: "Second"})
	v.prefs.WheelPages = true
	tt.Frame()

	tt.Wheel(360, 300, 0, 40)
	tt.Frame()
	if v.prefs.ActiveTab != "second" {
		t.Fatalf("a wheel down must show the next page, got %q", v.prefs.ActiveTab)
	}
	tt.Wheel(360, 300, 0, -40)
	tt.Frame()
	if v.prefs.ActiveTab != first {
		t.Fatalf("a wheel up must show the previous page, got %q", v.prefs.ActiveTab)
	}
}

// TestWheelOverActiveTabKeepsPage 验证光标停在当前页的胶囊上滚轮时，切过去的页面
// 不会被这个胶囊的悬停切换回来。
func TestWheelOverActiveTabKeepsPage(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	first := app.store.Categories[0].GUID
	app.store.Categories = append(app.store.Categories,
		CategoryNode{GUID: "second", Name: "Second"},
		CategoryNode{GUID: "third", Name: "Third"},
	)
	v.prefs.WheelPages = true
	tt.Frame()

	chip := v.tabRects[first]
	x, y := chip.X+chip.W/2, chip.Y+chip.H/2
	tt.Move(x, y)
	tt.Frame()
	tt.Wheel(x, y, 0, 40)
	tt.Frame()
	if v.prefs.ActiveTab != "second" {
		t.Fatalf("the wheel must show the next page, got %q", v.prefs.ActiveTab)
	}
	tt.Advance(2 * tabHoverDelay)
	if v.prefs.ActiveTab != "second" {
		t.Fatalf("the chip under the pointer must not switch the page back, got %q", v.prefs.ActiveTab)
	}

	tt.Wheel(x, y, 0, 40) // another turn while still on the same chip
	tt.Frame()
	if v.prefs.ActiveTab != "third" {
		t.Fatalf("another wheel must show the next page, got %q", v.prefs.ActiveTab)
	}
}

// TestWheelShiftSwapsPagesAndScroll 验证 Shift+滚轮与普通滚轮的用途互换：
// 开启切页时 Shift 滚动，关闭切页时 Shift 切页。
func TestWheelShiftSwapsPagesAndScroll(t *testing.T) {
	t.Run("enabled scrolls", func(t *testing.T) {
		app, v, tt := newTestView(t, 720, 460)
		for i := 0; i < 60; i++ {
			app.store.Categories[0].Slots = append(app.store.Categories[0].Slots, ptrString("a"))
		}
		app.store.Categories = append(app.store.Categories, CategoryNode{GUID: "second", Name: "Second"})
		v.prefs.WheelPages = true
		tt.Frame()
		first := v.prefs.ActiveTab

		if !v.wheelInput(ui.InputEvent{Kind: ui.InputScroll, DY: 40, Mods: ui.Shift, X: 360, Y: 300}) {
			t.Fatal("Shift+wheel must be taken")
		}
		tt.Frame()
		if v.prefs.ActiveTab != first {
			t.Fatalf("Shift+wheel must not change pages, got %q", v.prefs.ActiveTab)
		}
		if pv := v.pageState(v.tab()); pv.gridScroll.Y == 0 {
			t.Fatalf("Shift+wheel must scroll the grid, Y=%v MaxY=%v", pv.gridScroll.Y, pv.gridScroll.MaxY)
		}
	})

	t.Run("off switches", func(t *testing.T) {
		app, v, tt := newTestView(t, 720, 460)
		app.store.Categories = append(app.store.Categories, CategoryNode{GUID: "second", Name: "Second"})
		v.prefs.WheelPages = false
		tt.Frame()

		if !v.wheelInput(ui.InputEvent{Kind: ui.InputScroll, DY: 40, Mods: ui.Shift, X: 360, Y: 300}) {
			t.Fatal("Shift+wheel must be taken")
		}
		if v.prefs.ActiveTab != "second" {
			t.Fatalf("Shift+wheel must change pages when the wheel does not, got %q", v.prefs.ActiveTab)
		}
	})
}

// TestWheelTurnsOnePagePerEvent 验证一次滚轮事件即使滚动很远也只切一页，不会跳页。
func TestWheelTurnsOnePagePerEvent(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	app.store.Categories = append(app.store.Categories,
		CategoryNode{GUID: "second", Name: "Second"},
		CategoryNode{GUID: "third", Name: "Third"},
	)
	v.prefs.WheelPages = true
	tt.Frame()

	tt.Wheel(360, 300, 0, 400) // a wheel's notches, or a whole fling, in one event
	tt.Frame()
	if v.prefs.ActiveTab != "second" {
		t.Fatalf("one wheel event must turn one page, got %q", v.prefs.ActiveTab)
	}

	tt.Scroll(360, 300, 0, 400) // a touchpad's precise scroll, far in one event
	tt.Frame()
	if v.prefs.ActiveTab != "third" {
		t.Fatalf("one precise scroll must turn one page, got %q", v.prefs.ActiveTab)
	}
}

// TestWheelOverTabsKeepsPage 验证在页面胶囊上滚轮切页后，胶囊的悬停自动切换被
// 忽略，直到指针移到另一个胶囊。
func TestWheelOverTabsKeepsPage(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	first := app.store.Categories[0].GUID
	app.store.Categories = append(app.store.Categories,
		CategoryNode{GUID: "second", Name: "Second"},
		CategoryNode{GUID: "third", Name: "Third"},
	)
	v.prefs.WheelPages = true
	tt.Frame()

	third := v.tabRects["third"]
	x, y := third.X+third.W/2, third.Y+third.H/2
	tt.Move(x, y)
	tt.Frame()
	tt.Wheel(x, y, 0, 40) // switches from the first page to the second
	tt.Frame()
	if v.prefs.ActiveTab != "second" {
		t.Fatalf("the wheel must show the next page, got %q", v.prefs.ActiveTab)
	}
	tt.Advance(2 * tabHoverDelay)
	if v.prefs.ActiveTab != "second" {
		t.Fatalf("resting on the wheel's chip must not switch back, got %q", v.prefs.ActiveTab)
	}

	// Another chip's hover works again, and shows its page.
	firstChip := v.tabRects[first]
	tt.Move(firstChip.X+firstChip.W/2, firstChip.Y+firstChip.H/2)
	tt.Advance(2 * tabHoverDelay)
	if v.prefs.ActiveTab != first {
		t.Fatalf("hovering another chip must show its page again, got %q", v.prefs.ActiveTab)
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

// TestViewRuntimeDialogAdjustsByHour 验证 -1h / +1h 只改待保存的分钟数，点保存
// 才写入已保存的时长，且不会减到负数。
func TestViewRuntimeDialogAdjustsByHour(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	v.openRuntime("a")
	tt.Frame()
	for _, label := range []string{"-1h", "-30m", "+30m", "+1h"} {
		if !tt.HasText(label) {
			t.Fatalf("the dialog must show %q, texts %q", label, tt.Texts())
		}
	}
	stored := app.store.Apps["a"].RuntimeMs

	if err := tt.Click("+1h"); err != nil {
		t.Fatalf("add an hour: %v", err)
	}
	if got := app.store.Apps["a"].RuntimeMs; got != stored {
		t.Fatalf("+1h must wait for Save, stored %dms", got)
	}
	if v.runtimeMinutes != "120" {
		t.Fatalf("the minutes field must follow the buttons, got %q", v.runtimeMinutes)
	}
	if err := tt.Click("-1h"); err != nil {
		t.Fatalf("subtract an hour: %v", err)
	}
	if v.runtimeMinutes != "60" {
		t.Fatalf("-1h must take an hour off the field, got %q", v.runtimeMinutes)
	}
	if err := tt.Click("+30m"); err != nil {
		t.Fatalf("add half an hour: %v", err)
	}
	if v.runtimeMinutes != "90" {
		t.Fatalf("+30m must add half an hour, got %q", v.runtimeMinutes)
	}
	if err := tt.Click("-30m"); err != nil {
		t.Fatalf("subtract half an hour: %v", err)
	}
	if v.runtimeMinutes != "60" {
		t.Fatalf("-30m must take half an hour off, got %q", v.runtimeMinutes)
	}

	v.runtimeMinutes = "0"
	v.adjustRuntime(-60)
	if v.runtimeMinutes != "0" {
		t.Fatalf("the minutes must not go below zero, got %q", v.runtimeMinutes)
	}

	v.runtimeMinutes = "120"
	if err := tt.Click("Save"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := app.store.Apps["a"].RuntimeMs; got != 7_200_000 {
		t.Fatalf("Save must write the minutes, got %dms", got)
	}
}

// TestViewSettingsTogglesAutoTiming 验证设置窗口里的计时器开关写回 store。
func TestViewSettingsTogglesAutoTiming(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)
	app.store.Settings.AutoTime = true
	v.runtimeOpen = true
	settings := newTestSettings(t, v)

	if err := settings.Click("Toggle timer"); err != nil {
		t.Fatalf("timer: %v", err)
	}
	if app.store.Settings.AutoTime {
		t.Fatal("the timer toggle must turn the setting off")
	}
	if v.runtimeOpen {
		t.Fatal("turning the timer off must close its dialog")
	}
	if err := settings.Click("Toggle timer"); err != nil {
		t.Fatalf("timer again: %v", err)
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
		"Timer", "Auto-hide",
		"Always on top", "Absolute paths for new items", "Data directory",
		"Version " + appVersion,
		filepath.Join(absBase, dataDir),
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

// TestViewSettingsCustomAccent 验证强调色最右的自定义按钮：点开取色器，选中的颜色
// 以 #rrggbb 写进偏好。
func TestViewSettingsCustomAccent(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	settings := newTestSettings(t, v)
	if err := settings.Click("Custom accent color"); err != nil {
		t.Fatalf("open the custom accent picker: %v", err)
	}
	if !v.customAccentOpen {
		t.Fatal("the custom accent picker must open")
	}
	if err := settings.Click("Red"); err != nil {
		t.Fatalf("choose a color: %v", err)
	}
	if !strings.HasPrefix(v.prefs.Accent, "#") {
		t.Fatalf("the custom accent must be a hex color, got %q", v.prefs.Accent)
	}
	if got := loadPrefs().Accent; got != v.prefs.Accent {
		t.Fatalf("the custom accent must be saved, got %q", got)
	}
}

// TestViewSettingsChoosesIconSize 验证图标大小的三个挡位：默认中，选择小或大后记住
// 并写回 ui-state.json。
func TestViewSettingsChoosesIconSize(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	if v.prefs.IconSize != "medium" {
		t.Fatalf("the icon size must be medium by default, got %q", v.prefs.IconSize)
	}
	settings := newTestSettings(t, v)
	if err := settings.Click("Small"); err != nil {
		t.Fatalf("choose the small icons: %v", err)
	}
	if v.prefs.IconSize != "small" {
		t.Fatalf("the icon size must be remembered, got %q", v.prefs.IconSize)
	}
	if got := loadPrefs().IconSize; got != "small" {
		t.Fatalf("the icon size must be saved, got %q", got)
	}
	if err := settings.Click("Large"); err != nil {
		t.Fatalf("choose the large icons: %v", err)
	}
	if v.prefs.IconSize != "large" {
		t.Fatalf("the icon size must change, got %q", v.prefs.IconSize)
	}
}

// TestViewGridShowsRuntimeOnHover 验证网格单元左上角的时长默认不画，指针移到格子上
// 才出现；没有时长的条目只画计时图标，不画占位的「--」。
func TestViewGridShowsRuntimeOnHover(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	app.store.Settings.AutoTime = true
	tt.Frame()
	if tt.HasText("1h") {
		t.Fatalf("an unhovered cell must not show its runtime, texts %q", tt.Texts())
	}
	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha cell")
	}
	tt.Move(alpha.X+alpha.W/2, alpha.Y+alpha.H/2)
	tt.Frame()
	if !tt.HasText("1h") {
		t.Fatalf("a hovered cell must show its runtime, texts %q", tt.Texts())
	}
	beta, ok := tt.Find("Beta")
	if !ok {
		t.Fatal("no Beta cell")
	}
	tt.Move(beta.X+beta.W/2, beta.Y+beta.H/2)
	tt.Frame()
	if tt.HasText("--") {
		t.Fatalf("a cell without runtime must not show a placeholder, texts %q", tt.Texts())
	}
}

// TestViewHidesTimerWhenOff 验证计时器关闭后，格子里不再显示时长，运行中也不显示。
func TestViewHidesTimerWhenOff(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	app.store.Settings.AutoTime = false
	tt.Frame()

	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha cell")
	}
	tt.Move(alpha.X+alpha.W/2, alpha.Y+alpha.H/2)
	tt.Frame()
	if tt.HasText("1h") || tt.HasText("--") {
		t.Fatalf("the timer off must hide the runtime, texts %q", tt.Texts())
	}

	app.running["a"] = &runningProc{start: time.Now()}
	tt.Frame()
	if tt.HasText("1h") {
		t.Fatalf("a running item must not show a runtime with the timer off, texts %q", tt.Texts())
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
	app.store.Settings.AutoTime = true
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
	first := &runningProc{}
	app.running["a"] = first
	app.store.Settings.AutoTime = false

	app.finishProcess("a", first, time.Hour.Milliseconds())
	if _, ok := app.running["a"]; ok {
		t.Fatal("the finished process must no longer be tracked")
	}
	if got := app.runtimeStats["a"]; got != 5_000 {
		t.Fatalf("with auto timing off the runtime must not move, got %dms", got)
	}

	second := &runningProc{timed: true}
	app.running["a"] = second
	app.store.Settings.AutoTime = true
	app.finishProcess("a", second, 60_000)
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

	if !tt.HasText("设置") || !tt.HasText("搜索") {
		t.Fatalf("the window must say its texts in Chinese, texts %q", tt.Texts())
	}
	if tt.HasText("Settings") {
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
	dragTo(tt, from, to.X+to.W/2, to.Y+to.H/2)
}

// dragTo drags from the middle of from and lets go at x2, y2, which picks the
// side of a drop target the item lands on.
func dragTo(tt *ui.Tester, from ui.Rect, x2, y2 float32) {
	x1, y1 := from.X+from.W/2, from.Y+from.H/2
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

// TestViewDropsItemOnItsPageNothing 验证把条目拖到它当前所在的页面胶囊上放下，
// 什么都不改变。
func TestViewDropsItemOnItsPageNothing(t *testing.T) {
	app, _, tt := newTestView(t, 720, 520)
	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha row")
	}
	chip, ok := tt.Find("Default")
	if !ok {
		t.Fatal("no Default page")
	}
	drag(tt, alpha, chip)
	if got := tabSlots(app, 0); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("slots = %v, want the drop on its own page to change nothing", got)
	}
}

// TestViewDropsItemAtEndOfPage 验证条目拖到没有落点的地方时落到页面末尾，
// 而不是取消拖动。
func TestViewDropsItemAtEndOfPage(t *testing.T) {
	app, _, tt := newTestView(t, 720, 520)
	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha row")
	}
	dragTo(tt, alpha, 360, 350) // the empty part of the page
	if got := tabSlots(app, 0); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("slots = %v, want the dropped item at the end", got)
	}
}

// TestViewDropsBesideTarget 验证拖动的指示线决定落点：右半边插在目标之后，
// 左半边插在它之前。
func TestViewDropsBesideTarget(t *testing.T) {
	app, v, tt := newTestView(t, 720, 520)
	app.store.Apps["c"] = &AppItem{GUID: "c", Name: "Gamma", Path: "C:/tools/gamma.exe"}
	app.store.Categories[0].Slots = []*string{ptrString("a"), ptrString("b"), ptrString("c")}
	tt.Frame()

	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha cell")
	}
	beta, ok := tt.Find("Beta")
	if !ok {
		t.Fatal("no Beta cell")
	}
	dragTo(tt, alpha, beta.X+beta.W/2+20, beta.Y+beta.H/2) // the right half: after it
	if got := tabSlots(app, 0); len(got) != 3 || got[0] != "b" || got[1] != "a" || got[2] != "c" {
		t.Fatalf("slots = %v, want the item after Beta", got)
	}

	// Beta and Gamma now swap places around Alpha: drag Gamma before Alpha.
	tt = ui.NewTester(v.view, 720, 520)
	tt.SetFocused(true)
	tt.Frame()
	alpha, ok = tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha cell")
	}
	gamma, ok := tt.Find("Gamma")
	if !ok {
		t.Fatal("no Gamma cell")
	}
	dragTo(tt, gamma, alpha.X-20, alpha.Y+alpha.H/2) // the left half: before it
	if got := tabSlots(app, 0); len(got) != 3 || got[0] != "b" || got[1] != "c" || got[2] != "a" {
		t.Fatalf("slots = %v, want the item before Alpha", got)
	}
}

// TestViewGridDragReorders 验证网格里拖动一格把条目换位，空槽留在原处。
func TestViewGridDragReorders(t *testing.T) {
	app, _, tt := newTestView(t, 720, 520)
	app.store.Categories[0].Slots = append(app.store.Categories[0].Slots, nil)
	tt.Frame()

	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha cell")
	}
	beta, ok := tt.Find("Beta")
	if !ok {
		t.Fatal("no Beta cell")
	}
	dragTo(tt, alpha, beta.X+beta.W/2+20, beta.Y+beta.H/2) // the cell's right half

	if got := tabSlots(app, 0); len(got) != 3 || got[0] != "b" || got[1] != "a" || got[2] != "empty" {
		t.Fatalf("slots = %v, want the dragged item first and the empty cell last", got)
	}
}

// TestViewDragsItemToTab 验证把网格里的一格拖到另一个标签上，条目就搬过去。
func TestViewDragsItemToTab(t *testing.T) {
	app, _, tt := newTestView(t, 720, 520)
	if err := tt.Click("Add page"); err != nil {
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

// TestViewDragOverTabShowsIt 验证拖着条目停在页面胶囊上一会儿，就自动切换到这个
// 页面，方便直接把条目放到它里面的某个位置。
func TestViewDragOverTabShowsIt(t *testing.T) {
	app, v, tt := newTestView(t, 720, 520)
	if err := tt.Click("Add page"); err != nil {
		t.Fatalf("add a page: %v", err)
	}
	app.store.Categories[1].Name = "Second"
	if err := tt.Click("Default"); err != nil {
		t.Fatalf("show the first page: %v", err)
	}
	tt.Frame()
	second := app.store.Categories[1].GUID

	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha row")
	}
	chip, ok := tt.Find("Second")
	if !ok {
		t.Fatal("no Second page")
	}
	x1, y1 := alpha.X+alpha.W/2, alpha.Y+alpha.H/2
	x2, y2 := chip.X+chip.W/2, chip.Y+chip.H/2
	tt.Press(x1, y1)
	for i := 1; i <= 8; i++ {
		tt.Move(x1+(x2-x1)*float32(i)/8, y1+(y2-y1)*float32(i)/8)
	}
	if v.prefs.ActiveTab == second {
		t.Fatal("the page must not show at once on a passing drag")
	}
	tt.Advance(2 * time.Second)
	tt.Frame()
	if v.prefs.ActiveTab != second {
		t.Fatalf("resting on a page must show it, active %q want %q", v.prefs.ActiveTab, second)
	}
	tt.Release(x2, y2)
	tt.Frame()
	if got := tabSlots(app, 1); len(got) != 1 || got[0] != "a" {
		t.Fatalf("the second page = %v, want the dragged item", got)
	}
	// The drag ended with the drop: a later click must not drop it again.
	if err := tt.Click("Default"); err != nil {
		t.Fatalf("show the first page again: %v", err)
	}
	tt.Frame()
	beta, ok := tt.Find("Beta")
	if !ok {
		t.Fatal("no Beta row")
	}
	tt.ClickAt(beta.X+beta.W/2, beta.Y+beta.H/2)
	tt.Frame()
	if got := tabSlots(app, 0); len(got) != 1 || got[0] != "b" {
		t.Fatalf("the first page = %v, want the drop not to repeat", got)
	}
}

// TestViewDropsItemInAnotherPage 验证拖着条目停在另一个页面上切过去后，能直接落到
// 那个页面的条目旁，而不是卡在原页面。
func TestViewDropsItemInAnotherPage(t *testing.T) {
	app, v, tt := newTestView(t, 720, 520)
	if err := tt.Click("Add page"); err != nil {
		t.Fatalf("add a page: %v", err)
	}
	app.store.Categories[1].Name = "Second"
	app.store.Apps["c"] = &AppItem{GUID: "c", Name: "Gamma", Path: "C:/tools/gamma.exe"}
	app.store.Categories[1].Slots = []*string{ptrString("c")}
	if err := tt.Click("Default"); err != nil {
		t.Fatalf("show the first page: %v", err)
	}
	tt.Frame()
	second := app.store.Categories[1].GUID

	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha row")
	}
	chip, ok := tt.Find("Second")
	if !ok {
		t.Fatal("no Second page")
	}
	x1, y1 := alpha.X+alpha.W/2, alpha.Y+alpha.H/2
	x2, y2 := chip.X+chip.W/2, chip.Y+chip.H/2
	tt.Press(x1, y1)
	for i := 1; i <= 8; i++ {
		tt.Move(x1+(x2-x1)*float32(i)/8, y1+(y2-y1)*float32(i)/8)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if v.prefs.ActiveTab != second {
		t.Fatalf("resting on a page must show it, active %q want %q", v.prefs.ActiveTab, second)
	}

	gamma, ok := tt.Find("Gamma")
	if !ok {
		t.Fatalf("no Gamma row after switching, texts %q", tt.Texts())
	}
	x3, y3 := gamma.X, gamma.Y+gamma.H/2
	tt.Move(x3, y3)
	tt.Frame()
	tt.Release(x3, y3)
	tt.Frame()

	if got := tabSlots(app, 0); len(got) != 1 || got[0] != "b" {
		t.Fatalf("the first page = %v, want the dragged item gone", got)
	}
	if got := tabSlots(app, 1); len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("the second page = %v, want the item before Gamma", got)
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

// TestViewDropsFilesAtPosition 验证文件拖到某个单元格旁时插在那个位置，而不是
// 加到末尾。
func TestViewDropsFilesAtPosition(t *testing.T) {
	app, _, tt := newTestView(t, 720, 520)
	path := filepath.Join(t.TempDir(), "gamma.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatal("no Alpha cell")
	}
	if !tt.DropFiles(alpha.X+alpha.W/2+20, alpha.Y+alpha.H/2, path) {
		t.Fatalf("the cell must take dropped files, slots %v", tabSlots(app, 0))
	}
	if got := tabSlots(app, 0); len(got) != 3 || got[0] != "a" || got[2] != "b" || got[1] == "a" || got[1] == "b" {
		t.Fatalf("slots = %v, want the file after Alpha", got)
	}

	tt.Frame()
	beta, ok := tt.Find("Beta")
	if !ok {
		t.Fatal("no Beta cell")
	}
	if !tt.DropFiles(beta.X+beta.W/2-15, beta.Y+beta.H/2, path) {
		t.Fatal("the cell must take dropped files")
	}
	got := tabSlots(app, 0)
	if len(got) != 4 || got[0] != "a" || got[3] != "b" {
		t.Fatalf("slots = %v, want the file before Beta", got)
	}
}

// TestViewDropsFilesOnGridHalf 验证网格单元多出或收起运行时字段（悬停）时，文件仍按
// 左右半边插在条目前或后，不因卡片子元素数量变化而丢失位置。
func TestViewDropsFilesOnGridHalf(t *testing.T) {
	for _, hover := range []bool{false, true} {
		name := "unhovered"
		if hover {
			name = "hovered"
		}
		t.Run(name, func(t *testing.T) {
			app, _, tt := newTestView(t, 720, 520)
			path := filepath.Join(t.TempDir(), "gamma.txt")
			if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			alpha, ok := tt.Find("Alpha")
			if !ok {
				t.Fatal("no Alpha cell")
			}
			cx, cy := alpha.X+alpha.W/2, alpha.Y+alpha.H/2
			if hover {
				tt.Move(cx, cy)
				tt.Frame()
			}
			if !tt.DropFiles(cx+20, cy, path) {
				t.Fatal("the cell's right half must take files")
			}
			if got := tabSlots(app, 0); len(got) != 3 || got[0] != "a" || got[2] != "b" {
				t.Fatalf("slots = %v, want the file after Alpha", got)
			}
		})
	}
}

// TestViewDropsFilesOnTab 验证拖到某个标签上的文件加入该标签。
func TestViewDropsFilesOnTab(t *testing.T) {
	app, _, tt := newTestView(t, 900, 520)
	if err := tt.Click("Add page"); err != nil {
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
	if err := tt.Click("Add page"); err != nil {
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
