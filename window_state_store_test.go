package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

// TestWindowStateStoreRoundTrip 验证窗口几何写进数据文件夹并能读回。
func TestWindowStateStoreRoundTrip(t *testing.T) {
	old := absBase
	absBase = t.TempDir()
	t.Cleanup(func() { absBase = old })

	states, err := (windowStateStore{}).LoadWindowStates()
	if err != nil || states != nil {
		t.Fatalf("a missing file must load as no state, got %v, %v", states, err)
	}

	want := map[string]mygo.WindowState{
		windowStateKey: {X: 120, Y: 80, Width: 720, Height: 520, Maximized: true},
	}
	if err := (windowStateStore{}).SaveWindowStates(want); err != nil {
		t.Fatal(err)
	}
	got, err := (windowStateStore{}).LoadWindowStates()
	if err != nil {
		t.Fatal(err)
	}
	if state := got[windowStateKey]; state != want[windowStateKey] {
		t.Fatalf("state = %+v, want %+v", state, want[windowStateKey])
	}
}

// TestWindowStateStoreReadsTheOldFile 验证旧版（Wails 构建）写的单窗口几何还能读：
// 升级后窗口不该跳回默认大小。
func TestWindowStateStoreReadsTheOldFile(t *testing.T) {
	old := absBase
	absBase = t.TempDir()
	t.Cleanup(func() { absBase = old })

	if err := os.MkdirAll(filepath.Join(absBase, dataDir), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"width": 640, "height": 400, "x": 40, "y": 20, "maximised": true}`
	if err := os.WriteFile(filepath.Join(absBase, windowStateFile), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	states, err := (windowStateStore{}).LoadWindowStates()
	if err != nil {
		t.Fatal(err)
	}
	state, ok := states[windowStateKey]
	if !ok {
		t.Fatalf("the old file must be read into %q, got %+v", windowStateKey, states)
	}
	if state.Width != 640 || state.Height != 400 || state.X != 40 || state.Y != 20 || !state.Maximized {
		t.Fatalf("the old geometry was not kept: %+v", state)
	}
}
