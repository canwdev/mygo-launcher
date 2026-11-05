package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// newTestApp builds an App in a temporary data folder: absBase, which every
// path of the app hangs off, points there for the test's duration.
func newTestApp(t *testing.T) *App {
	t.Helper()
	old := absBase
	absBase = t.TempDir()
	t.Cleanup(func() { absBase = old })

	if err := os.MkdirAll(filepath.Join(absBase, dataDir), 0o755); err != nil {
		t.Fatal(err)
	}
	app := &App{
		store:        loadStore(),
		runtimeStats: map[string]int64{},
		running:      map[string]*runningProc{},
		icons:        map[string]ui.ImageSource{},
	}
	app.syncStoreFileState()
	return app
}

func ptrString(s string) *string { return &s }

// TestStorePersistsRunAsAdmin 验证 item 编辑弹窗里的“Run as administrator”勾选：
// 写入 store 文件、写出的 JSON 键名可被前端/后端读回，未勾选的条目保持不写该键。
func TestStorePersistsRunAsAdmin(t *testing.T) {
	app := newTestApp(t)
	app.store.Apps["admin"] = &AppItem{GUID: "admin", Name: "Admin", Path: "C:/admin.exe", RunAsAdmin: true}
	app.store.Apps["plain"] = &AppItem{GUID: "plain", Name: "Plain", Path: "C:/plain.exe"}
	app.store.Categories[0].Slots = []*string{ptrString("admin"), ptrString("plain")}
	app.save()

	data, err := os.ReadFile(filepath.Join(absBase, saveFile))
	if err != nil {
		t.Fatal(err)
	}

	var raw struct {
		Apps map[string]map[string]any `json:"apps"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if got, _ := raw.Apps["admin"]["run_as_admin"].(bool); !got {
		t.Fatalf("run_as_admin must be persisted, got %v", raw.Apps["admin"])
	}
	if _, ok := raw.Apps["plain"]["run_as_admin"]; ok {
		t.Fatalf("items without the flag must omit run_as_admin, got %v", raw.Apps["plain"])
	}

	store, ok := parseStore(data)
	if !ok {
		t.Fatal("parseStore rejected the store written by save")
	}
	if store.Apps["admin"] == nil || !store.Apps["admin"].RunAsAdmin {
		t.Fatalf("run_as_admin did not survive a reload: %+v", store.Apps["admin"])
	}
	if store.Apps["plain"] == nil || store.Apps["plain"].RunAsAdmin {
		t.Fatalf("run_as_admin must default to false: %+v", store.Apps["plain"])
	}
}

// TestParseStoreReadsHandEditedRunAsAdmin 覆盖用户手改配置文件写入该字段的场景。
func TestParseStoreReadsHandEditedRunAsAdmin(t *testing.T) {
	store, ok := parseStore([]byte(`{
		"apps": {"a": {"guid": "a", "name": "A", "path": "C:/a.exe", "run_as_admin": true}},
		"categories": [{"guid": "c", "name": "Main", "slots": ["a"]}],
		"settings": {}
	}`))
	if !ok {
		t.Fatal("parseStore rejected valid JSON")
	}
	if store.Apps["a"] == nil || !store.Apps["a"].RunAsAdmin {
		t.Fatalf("hand-edited run_as_admin not read: %+v", store.Apps["a"])
	}
}

func TestParseStore(t *testing.T) {
	if _, ok := parseStore([]byte("{ not json")); ok {
		t.Fatal("parseStore must reject invalid JSON")
	}

	// 旧文件缺 auto_time / auto_hide：默认打开
	store, ok := parseStore([]byte(`{"apps":{"a":{"guid":"a","path":"C:/a.exe"}},"categories":[{"guid":"c","name":"Main"}]}`))
	if !ok {
		t.Fatal("parseStore rejected valid JSON")
	}
	if !store.Settings.AutoTime || !store.Settings.AutoHide {
		t.Fatalf("legacy file should default auto_time/auto_hide to true, got %+v", store.Settings)
	}
	if store.Categories[0].Slots == nil {
		t.Fatal("nil slots must be normalized to an empty slice")
	}
	if store.Apps["a"].Name == "" {
		t.Fatal("items without a name must get one from their path")
	}

	// 空 store 回退默认（与 loadStore 的既有语义一致）
	empty, ok := parseStore([]byte(`{"apps":{},"categories":[]}`))
	if !ok {
		t.Fatal("parseStore rejected an empty store")
	}
	if len(empty.Categories) != 1 || len(empty.Apps) != 0 {
		t.Fatalf("empty store should fall back to the default store, got %+v", empty)
	}
}

// TestConvertItemPathsRoundTrip 固定绝对路径与相对路径之间的往返：两种设置切换
// 后 store 里的三个路径字段都要能还原。
func TestConvertItemPathsRoundTrip(t *testing.T) {
	old := absBase
	absBase = t.TempDir()
	t.Cleanup(func() { absBase = old })

	item := &AppItem{
		Path:       filepath.Join("tools", "app.exe"),
		WorkingDir: filepath.Join("tools"),
		Icon:       filepath.Join(iconsDir, "app.png"),
	}
	relative := item.Path

	convertItemPaths(item, true)
	if !filepath.IsAbs(item.Path) || !filepath.IsAbs(item.Icon) {
		t.Fatalf("absolute conversion failed: %+v", item)
	}
	convertItemPaths(item, false)
	if item.Path != relative {
		t.Fatalf("relative conversion did not restore the path: got %q, want %q", item.Path, relative)
	}
	if filepath.IsAbs(item.WorkingDir) {
		t.Fatalf("working directory must be relative again: %q", item.WorkingDir)
	}
}
