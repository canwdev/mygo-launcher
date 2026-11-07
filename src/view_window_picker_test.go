package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestWindowPickerAddsProgram 打开添加入口，选择运行中的窗口后，程序被追加到
// 当前页面。
func TestWindowPickerAddsProgram(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	old := listWindows
	listWindows = func() []runningWindow {
		return []runningWindow{{
			Name:  "Notepad",
			Title: "Untitled - Notepad",
			Path:  `C:\Windows\notepad.exe`,
			PID:   42,
		}}
	}
	t.Cleanup(func() { listWindows = old })

	if err := tt.Click("Add"); err != nil {
		t.Fatalf("open the window picker: %v", err)
	}
	if err := tt.Click("Running windows"); err != nil {
		t.Fatalf("open the running windows list: %v", err)
	}
	if !tt.HasText("Notepad") || !tt.HasText("Untitled - Notepad") {
		t.Fatalf("the picker must list the running window, texts %q", tt.Texts())
	}
	if err := tt.Click("Notepad"); err != nil {
		t.Fatalf("choose the window: %v", err)
	}
	tt.Frame()

	tab := app.store.Categories[0]
	if len(tab.Slots) != 3 {
		t.Fatalf("the item must be appended, slots %d", len(tab.Slots))
	}
	item := app.store.Apps[*tab.Slots[2]]
	if item == nil || item.Name != "Notepad" || !strings.Contains(item.Path, "notepad.exe") {
		t.Fatalf("the item must be the window's program, got %+v", item)
	}
	if tt.HasText("Running windows") {
		t.Fatalf("the picker must close after choosing, texts %q", tt.Texts())
	}
}

// TestWindowPickerAddsSystemLocation 切到系统位置并选择回收站后，它作为 shell
// 目标条目加入当前页面，路径原样保存、不填工作目录。
func TestWindowPickerAddsSystemLocation(t *testing.T) {
	if len(systemLocations) == 0 {
		t.Skip("no system locations on this platform")
	}
	app, _, tt := newTestView(t, 720, 460)
	old := listWindows
	listWindows = func() []runningWindow { return nil }
	t.Cleanup(func() { listWindows = old })

	if err := tt.Click("Add"); err != nil {
		t.Fatalf("open the window picker: %v", err)
	}
	if err := tt.Click("System locations"); err != nil {
		t.Fatalf("show the system locations: %v", err)
	}
	for _, name := range []string{
		"Recycle Bin", "This PC", "Control Panel", "User folder", "Network",
		"Startup", "Recent", "Desktop", "Downloads", "Documents", "Pictures",
		"Music", "Videos",
	} {
		if !tt.HasText(name) {
			t.Fatalf("the picker must list %q, texts %q", name, tt.Texts())
		}
	}
	if err := tt.Click("Recycle Bin"); err != nil {
		t.Fatalf("choose the location: %v", err)
	}
	tt.Frame()

	tab := app.store.Categories[0]
	if len(tab.Slots) != 3 {
		t.Fatalf("the item must be appended, slots %d", len(tab.Slots))
	}
	item := app.store.Apps[*tab.Slots[2]]
	if item == nil || item.Name != "Recycle Bin" ||
		item.Path != "::{645FF040-5081-101B-9F08-00AA002F954E}" || item.WorkingDir != "" {
		t.Fatalf("the item must be the Recycle Bin, got %+v", item)
	}
}

// TestWindowPickerMarksAddedLocation 验证已在页面上的系统位置被标记，再选它只会定位
// 到已有条目，不再重复添加。
func TestWindowPickerMarksAddedLocation(t *testing.T) {
	if len(systemLocations) == 0 {
		t.Skip("no system locations on this platform")
	}
	app, _, tt := newTestView(t, 720, 460)
	old := listWindows
	listWindows = func() []runningWindow { return nil }
	t.Cleanup(func() { listWindows = old })

	if err := tt.Click("Add"); err != nil {
		t.Fatalf("open the add menu: %v", err)
	}
	if err := tt.Click("System locations"); err != nil {
		t.Fatalf("open the system locations list: %v", err)
	}
	if err := tt.Click("Recycle Bin"); err != nil {
		t.Fatalf("add the Recycle Bin: %v", err)
	}
	tt.Frame()
	tab := app.store.Categories[0]
	if len(tab.Slots) != 3 {
		t.Fatalf("the Recycle Bin must be added, slots %d", len(tab.Slots))
	}
	// The cell and the picker row would share a label; rename the cell so the
	// test clicks the picker row.
	app.store.Apps[*tab.Slots[2]].Name = "My Bin"
	tt.Frame()

	if err := tt.Click("Add"); err != nil {
		t.Fatalf("open the add menu again: %v", err)
	}
	if err := tt.Click("System locations"); err != nil {
		t.Fatalf("open the system locations list again: %v", err)
	}
	if err := tt.Click("Recycle Bin"); err != nil {
		t.Fatalf("choose the added location: %v", err)
	}
	tt.Frame()
	if len(tab.Slots) != 3 {
		t.Fatalf("an added location must not be added twice, slots %d", len(tab.Slots))
	}
}

// TestWindowPickerReloadsRunningWindows 验证每次打开「运行中的窗口」都重新枚举：
// 切换列表后和关闭菜单后再打开。
func TestWindowPickerReloadsRunningWindows(t *testing.T) {
	calls := 0
	old := listWindows
	listWindows = func() []runningWindow {
		calls++
		return []runningWindow{{
			Name:  fmt.Sprintf("App%d", calls),
			Title: "Window",
			Path:  fmt.Sprintf(`C:\tools\app%d.exe`, calls),
			PID:   uint32(calls),
		}}
	}
	t.Cleanup(func() { listWindows = old })

	_, _, tt := newTestView(t, 720, 460)
	openWindows := func() {
		if err := tt.Click("Add"); err != nil {
			t.Fatalf("open the add menu: %v", err)
		}
		if err := tt.Click("Running windows"); err != nil {
			t.Fatalf("open the running windows list: %v", err)
		}
	}
	openWindows()
	if calls != 1 || !tt.HasText("App1") {
		t.Fatalf("the first open must load, calls %d, texts %q", calls, tt.Texts())
	}
	if err := tt.Click("System locations"); err != nil {
		t.Fatalf("open the system locations list: %v", err)
	}
	if err := tt.Click("Running windows"); err != nil {
		t.Fatalf("switch back to the running windows list: %v", err)
	}
	if calls != 2 || !tt.HasText("App2") {
		t.Fatalf("switching back must reload, calls %d, texts %q", calls, tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	openWindows()
	if calls != 3 || !tt.HasText("App3") {
		t.Fatalf("reopening must reload, calls %d, texts %q", calls, tt.Texts())
	}
}

// TestBuildRunningItemKeepsPackageTarget 打包应用按 shell 路径保存，不填工作
// 目录（它不是文件路径）。
func TestBuildRunningItemKeepsPackageTarget(t *testing.T) {
	newTestApp(t)
	w := runningWindow{
		Name:  "计算器",
		Title: "计算器",
		Path:  `shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`,
		PID:   1,
	}
	item := buildRunningItem(w, false, false)
	if item == nil {
		t.Fatal("the item must build")
	}
	if item.Name != "计算器" || item.Path != w.Path || item.WorkingDir != "" {
		t.Fatalf("the package item must keep its target, got %+v", item)
	}
}

// TestWindowPickerEmpty 没有可添加的窗口时，面板保留入口并说明为空。
func TestWindowPickerEmpty(t *testing.T) {
	_, _, tt := newTestView(t, 720, 460)
	old := listWindows
	listWindows = func() []runningWindow { return nil }
	t.Cleanup(func() { listWindows = old })

	if err := tt.Click("Add"); err != nil {
		t.Fatalf("open the window picker: %v", err)
	}
	if !tt.HasText("Pick files…") || !tt.HasText("Create…") {
		t.Fatalf("the picker must keep both add actions, texts %q", tt.Texts())
	}
	if err := tt.Click("Running windows"); err != nil {
		t.Fatalf("open the running windows list: %v", err)
	}
	if !tt.HasText("No windows to add") {
		t.Fatalf("the picker must say it is empty, texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if tt.HasText("Running windows") {
		t.Fatalf("Escape must close the picker, texts %q", tt.Texts())
	}
}

// TestWindowPickerSwitchesSubmenu 验证两个二级列表共用同一块面板：打开系统位置
// 后再点运行中的窗口，列表随之切换。
func TestWindowPickerSwitchesSubmenu(t *testing.T) {
	if len(systemLocations) == 0 {
		t.Skip("no system locations on this platform")
	}
	_, _, tt := newTestView(t, 720, 460)
	old := listWindows
	listWindows = func() []runningWindow { return nil }
	t.Cleanup(func() { listWindows = old })

	if err := tt.Click("Add"); err != nil {
		t.Fatalf("open the window picker: %v", err)
	}
	if err := tt.Click("System locations"); err != nil {
		t.Fatalf("open the system locations list: %v", err)
	}
	if !tt.HasText("Recycle Bin") {
		t.Fatalf("the system locations must show, texts %q", tt.Texts())
	}
	if err := tt.Click("Running windows"); err != nil {
		t.Fatalf("switch to the running windows list: %v", err)
	}
	if !tt.HasText("No windows to add") || tt.HasText("Recycle Bin") {
		t.Fatalf("the list must switch, texts %q", tt.Texts())
	}
}
