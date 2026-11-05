package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

// writeManualStore 模拟用户手动编辑 go-launcher-data.json。
func writeManualStore(t *testing.T, store AppStore) {
	t.Helper()
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeManualBytes(t, data)
}

func writeManualBytes(t *testing.T, data []byte) {
	t.Helper()
	path := filepath.Join(absBase, saveFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	// 保证 mtime 与上一次已知状态不同（部分文件系统时间戳粒度较粗）。
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
}

func testStoreWithHandItem() AppStore {
	return AppStore{
		Apps: map[string]*AppItem{
			"hand": {GUID: "hand", Name: "Hand edited", Path: "C:/tools/hand.exe", RuntimeMs: 12345},
		},
		Categories: []CategoryNode{
			{GUID: "cat-1", Name: "Manual", Slots: []*string{ptrString("hand")}},
		},
	}
}

func TestSyncStoreFileStateBaselinesExistingFile(t *testing.T) {
	app := newTestApp(t)
	writeManualStore(t, testStoreWithHandItem())
	app.syncStoreFileState()

	if got := app.reloadStoreIfChanged(); got != storeReloadNone {
		t.Fatalf("a file already fingerprinted at startup must not be reloaded, got %v", got)
	}
}

func TestReloadStoreIfChangedIgnoresOwnWrites(t *testing.T) {
	app := newTestApp(t)

	app.store.Apps["own"] = &AppItem{GUID: "own", Name: "Own", Path: "C:/own.exe"}
	app.runtimeStats["own"] = 555
	app.store.Categories[0].Slots = []*string{ptrString("own")}
	app.save()

	if got := app.reloadStoreIfChanged(); got != storeReloadNone {
		t.Fatalf("own write must not be treated as an external edit, got %v", got)
	}
	if app.store.Apps["own"] == nil || app.runtimeStats["own"] != 555 {
		t.Fatalf("own write must keep the in-memory store intact, got %+v", app.store)
	}
}

func TestReloadStoreIfChangedAppliesManualEdit(t *testing.T) {
	app := newTestApp(t)
	app.save()

	writeManualStore(t, testStoreWithHandItem())

	if got := app.reloadStoreIfChanged(); got != storeReloadApplied {
		t.Fatalf("manual edit should be applied, got %v", got)
	}
	item := app.store.Apps["hand"]
	if item == nil || item.Name != "Hand edited" {
		t.Fatalf("manual edit not applied: %+v", app.store.Apps)
	}
	if app.runtimeStats["hand"] != 12345 {
		t.Fatalf("hand-edited runtime_ms must win, got %d", app.runtimeStats["hand"])
	}
	if len(app.store.Categories) != 1 || app.store.Categories[0].Name != "Manual" {
		t.Fatalf("manual categories not applied: %+v", app.store.Categories)
	}
	if got := app.reloadStoreIfChanged(); got != storeReloadNone {
		t.Fatalf("the same revision must only be applied once, got %v", got)
	}
}

func TestReloadStoreIfChangedIgnoresInvalidContent(t *testing.T) {
	app := newTestApp(t)
	writeManualStore(t, testStoreWithHandItem())
	if got := app.reloadStoreIfChanged(); got != storeReloadApplied {
		t.Fatalf("manual edit should be applied, got %v", got)
	}

	writeManualBytes(t, []byte(`{"apps": {`))
	if got := app.reloadStoreIfChanged(); got != storeReloadInvalid {
		t.Fatalf("invalid JSON should be reported as invalid, got %v", got)
	}
	if app.store.Apps["hand"] == nil {
		t.Fatal("a broken hand edit must not wipe the in-memory store")
	}
	if got := app.reloadStoreIfChanged(); got != storeReloadNone {
		t.Fatalf("the same broken revision must only be reported once, got %v", got)
	}
}

func TestReloadStoreDropsItemsRemovedOnDisk(t *testing.T) {
	app := newTestApp(t)
	app.store.Apps["hand"] = &AppItem{GUID: "hand", Name: "Hand", Path: "C:/hand.exe"}
	app.runtimeStats["hand"] = 999
	app.running["hand"] = &runningProc{}
	app.save()

	writeManualStore(t, AppStore{
		Apps:       map[string]*AppItem{},
		Categories: []CategoryNode{{GUID: "cat-2", Name: "Kept", Slots: []*string{}}},
	})

	if got := app.reloadStoreIfChanged(); got != storeReloadApplied {
		t.Fatalf("manual edit should be applied, got %v", got)
	}
	if _, ok := app.running["hand"]; ok {
		t.Fatal("an item removed on disk must no longer be tracked as running")
	}
	if _, ok := app.runtimeStats["hand"]; ok {
		t.Fatal("an item removed on disk must not keep runtime stats")
	}
	if len(app.store.Categories) != 1 || app.store.Categories[0].Name != "Kept" {
		t.Fatalf("manual categories not applied: %+v", app.store.Categories)
	}
}

// TestRuntimeText 固定运行时数值的读法：写成短格式，运行中的计时再加一段
// 「(+ 1m)」，没有运行时只显示占位符。
func TestRuntimeText(t *testing.T) {
	app := newTestApp(t)
	v := newView(app)
	app.store.Apps["a"] = &AppItem{GUID: "a", Name: "A", RuntimeMs: 90_000}
	app.runtimeStats["a"] = 90_000
	v.states = app.state()

	if got := v.runtimeText("a"); got != "1m" {
		t.Fatalf("runtime text of 90s should be %q, got %q", "1m", got)
	}
	if got := v.runtimeText("missing"); got != "--" {
		t.Fatalf("an item without runtime should show the placeholder, got %q", got)
	}

	v.prefs.Timers["a"] = time.Now().Add(-65 * time.Second).UnixMilli()
	got := v.runtimeText("a")
	if len(got) <= len("1m") || got[:len("1m")] != "1m" {
		t.Fatalf("a running timer must be added to the runtime, got %q", got)
	}
}

func TestStopStoreWatcherIsIdempotent(t *testing.T) {
	app := newTestApp(t)
	app.startStoreWatcher()
	app.stopStoreWatcher()
	app.stopStoreWatcher()
}

// TestStoreWatchDeliversEdits 验证底层监听：写一次 store 文件后要收到事件，
// 这是手改配置能生效的前提。
func TestStoreWatchDeliversEdits(t *testing.T) {
	app := newTestApp(t)
	app.save()
	path := filepath.Join(absBase, saveFile)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	w, err := mygo.WatchFiles(ctx, path, mygo.FileWatchOptions{})
	if err != nil {
		t.Fatalf("watching %s: %v", path, err)
	}
	defer w.Close()

	writeManualStore(t, testStoreWithHandItem())

	done := make(chan error, 1)
	go func() {
		events, err := w.Next(ctx)
		if err == nil && len(events) == 0 {
			err = errors.New("an empty batch says nothing changed")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the watch ended before an edit: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("no event arrived within 10s of an edit")
	}
}
