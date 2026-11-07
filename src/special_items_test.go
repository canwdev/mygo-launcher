package main

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

// TestRefreshSpecialIcon 验证特殊条目的刷新钩子把状态图标留在内存里，不写缓存文件，
// 也不改条目的持久化图标。
func TestRefreshSpecialIcon(t *testing.T) {
	app := newTestApp(t)
	item := &AppItem{GUID: "x", Name: "X", Path: "special"}
	app.store.Apps["x"] = item

	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	icon := buf.Bytes()

	old := specialItems
	specialItems = []specialItem{{
		Match:       func(path string) bool { return path == "special" },
		RefreshIcon: func(*AppItem) ([]byte, bool) { return icon, true },
	}}
	t.Cleanup(func() { specialItems = old })

	if !app.refreshSpecialIcon(item) {
		t.Fatal("the special icon must refresh")
	}
	if app.iconImage(item) == nil {
		t.Fatal("the state icon must show")
	}
	if item.Icon != "" {
		t.Fatalf("the state icon must stay in memory, icon %q", item.Icon)
	}
}
