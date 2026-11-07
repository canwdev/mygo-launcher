//go:build windows

package main

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestCanUseShellActions 验证属性和系统菜单适用于文件、文件夹和 shell 位置，但不适用
// 于 UWP 应用和网址。
func TestCanUseShellActions(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{`C:\Windows\notepad.exe`, true},
		{`C:\Windows`, true},
		{recycleBinCLSID, true},
		{`shell:RecycleBinFolder`, true},
		{`shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`, false},
		{`https://example.com`, false},
		{``, false},
	}
	for _, c := range cases {
		if got := canUseShellActions(c.path); got != c.want {
			t.Errorf("canUseShellActions(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestItemMenuPropertiesLast 验证「属性」在菜单的最下方。
func TestItemMenuPropertiesLast(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)
	if !canUseShellActions(`C:\tools\alpha.exe`) {
		t.Skip("no shell actions on this platform")
	}
	entries := v.itemMenu(app.store.Apps["a"], app.store.Categories[0].GUID, 0)
	last := ""
	for _, entry := range entries {
		if entry.label != "" {
			last = entry.label
		}
	}
	if last != "Properties" {
		t.Fatalf("the last entry = %q, want Properties", last)
	}
}

// TestItemMenuShellActions 验证文件的菜单有「属性」和「系统菜单」，UWP 条目没有。
func TestItemMenuShellActions(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)

	if err := tt.RightClick("Alpha"); err != nil {
		t.Fatalf("open the item menu: %v", err)
	}
	for _, want := range []string{"Properties", "System menu…"} {
		if !tt.HasText(want) {
			t.Fatalf("the menu must offer %q, texts %q", want, tt.Texts())
		}
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	app.store.Apps["b"].Path = `shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`
	tt.Frame()
	if err := tt.RightClick("Beta"); err != nil {
		t.Fatalf("open the UWP item menu: %v", err)
	}
	if tt.HasText("Properties") || tt.HasText("System menu…") {
		t.Fatalf("a packaged app must not offer shell actions, texts %q", tt.Texts())
	}
}

// TestRecycleBinMenu 验证回收站有内容时菜单提供清空，为空时不提供；点击清空先询问。
func TestRecycleBinMenu(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	app.store.Apps["b"].Path = recycleBinCLSID
	app.store.Apps["b"].Name = "Recycle Bin"
	old := recycleBinItemCount
	recycleBinItemCount = func() (int64, bool) { return 2, true }
	t.Cleanup(func() {
		recycleBinItemCount = old
		forgetRecycleBinItems()
	})
	forgetRecycleBinItems()
	tt.Frame()

	if err := tt.RightClick("Recycle Bin"); err != nil {
		t.Fatalf("open the Recycle Bin menu: %v", err)
	}
	if !tt.HasText("Empty Recycle Bin") {
		t.Fatalf("a bin with items must offer emptying, texts %q", tt.Texts())
	}
	if err := tt.Click("Empty Recycle Bin"); err != nil {
		t.Fatalf("choose emptying: %v", err)
	}
	if !tt.HasText("Empty the Recycle Bin? This cannot be undone.") {
		t.Fatalf("emptying must ask first, texts %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	recycleBinItemCount = func() (int64, bool) { return 0, true }
	forgetRecycleBinItems()
	if err := tt.RightClick("Recycle Bin"); err != nil {
		t.Fatalf("open the empty bin's menu: %v", err)
	}
	if tt.HasText("Empty Recycle Bin") {
		t.Fatalf("an empty bin must not offer emptying, texts %q", tt.Texts())
	}
}

// TestShellContextMenuBuilds 验证能取到回收站的原生 shell 菜单（不弹出）。
func TestShellContextMenuBuilds(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ok, uninit := coInitCOM()
	if !ok {
		t.Skip("COM is not available")
	}
	if uninit {
		defer procCoUninitialize.Call()
	}

	hMenu, _, cleanup, err := shellContextMenu(recycleBinCLSID)
	if err != nil {
		t.Skipf("the shell has no menu for the Recycle Bin: %v", err)
	}
	defer cleanup()
	count, _, _ := procGetMenuItemCount.Call(hMenu)
	if count == 0 {
		t.Fatal("the shell menu has no entries")
	}
}
