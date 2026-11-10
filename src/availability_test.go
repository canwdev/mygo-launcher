package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCheckAvailability 验证批量检测：存在的程序与文件夹不置灰，缺失的条目
// 置灰，shell 目标不检查；修好路径后再检测会取消置灰。
func TestCheckAvailability(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	dir := t.TempDir()
	tool := filepath.Join(dir, "tool.exe")
	if err := os.WriteFile(tool, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "folder")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	app.store.Apps = map[string]*AppItem{
		"ok":   {GUID: "ok", Name: "Ok", Path: tool},
		"gone": {GUID: "gone", Name: "Gone", Path: filepath.Join(dir, "gone.exe")},
		"dir":  {GUID: "dir", Name: "Dir", WorkingDir: folder},
		"url":  {GUID: "url", Name: "Url", Path: "https://example.com"},
	}
	app.store.Categories[0].Slots = []*string{
		ptrString("ok"), ptrString("gone"), ptrString("dir"), ptrString("url"),
	}
	tt.Frame()

	v.checkAvailability() // tests run the check on the calling goroutine
	if app.isMissing("ok") || app.isMissing("dir") || app.isMissing("url") {
		t.Fatalf("only the missing item may be dimmed, got %+v", app.missing)
	}
	if !app.isMissing("gone") {
		t.Fatalf("a missing file must be dimmed, got %+v", app.missing)
	}
	tt.Frame()
	if !tt.HasText("1 item is missing and dimmed") {
		t.Fatalf("the check must report what it found, texts %q", tt.Texts())
	}

	// Fixing the path through the editor clears the dimming at once.
	v.updateItem("gone", itemFields{Name: "Gone", Path: tool})
	tt.Frame()
	if app.isMissing("gone") {
		t.Fatal("an edited item with an existing path must not stay dimmed")
	}

	// The next check clears what no longer fails, and the store-wide message
	// says everything is available.
	os.Remove(filepath.Join(dir, "gone.exe"))
	v.checkAvailability()
	tt.Frame()
	if len(app.missing) != 0 {
		t.Fatalf("every item is available now, got %+v", app.missing)
	}
	if !tt.HasText("All items are available") {
		t.Fatalf("the check must say everything is available, texts %q", tt.Texts())
	}
}

// TestLaunchUpdatesAvailability 验证启动前的检查同时更新置灰标记：文件缺失时
// 置灰，放回文件后再次准备启动会取消置灰。
func TestLaunchUpdatesAvailability(t *testing.T) {
	app := newTestApp(t)
	tool := filepath.Join(t.TempDir(), "x.exe")
	app.store.Apps["x"] = &AppItem{GUID: "x", Name: "X", Path: tool}

	if _, err := app.planLaunch("x"); err == nil {
		t.Fatal("a missing path must be reported before the launch")
	}
	if !app.isMissing("x") {
		t.Fatal("a failed launch must dim the item")
	}

	if err := os.WriteFile(tool, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.planLaunch("x"); err != nil {
		t.Fatal(err)
	}
	if app.isMissing("x") {
		t.Fatal("an item that launches again must not stay dimmed")
	}
}

// TestSettingsShowsAvailabilityCheck 验证设置的「数据」分组提供检查可用性。
func TestSettingsShowsAvailabilityCheck(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	settings := newTestSettings(t, v)
	if !settings.HasText("Check availability") {
		t.Fatalf("the data group must offer the availability check, texts %q", settings.Texts())
	}
}

// TestSettingsShowsRestartAndExit 验证「关于」底部提供重启与退出。
func TestSettingsShowsRestartAndExit(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	settings := newTestSettings(t, v)
	for _, label := range []string{"Restart", "Exit"} {
		if !settings.HasText(label) {
			t.Fatalf("the about group must offer %q, texts %q", label, settings.Texts())
		}
	}
}
