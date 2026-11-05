package main

import (
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
