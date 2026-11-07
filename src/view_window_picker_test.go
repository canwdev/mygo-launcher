package main

import (
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
	if !tt.HasText("Running windows") || !tt.HasText("Notepad") || !tt.HasText("Untitled - Notepad") {
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
	if !tt.HasText("No windows to add") {
		t.Fatalf("the picker must say it is empty, texts %q", tt.Texts())
	}
	if !tt.HasText("Pick files…") || !tt.HasText("Create…") {
		t.Fatalf("the picker must keep both add actions, texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if tt.HasText("Running windows") {
		t.Fatalf("Escape must close the picker, texts %q", tt.Texts())
	}
}
