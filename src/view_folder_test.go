package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// folderFixture puts a folder named Stuff in the test's data folder: it holds
// note.txt and the folder Sub, which holds deep.txt. It returns the folder's
// path and the file's.
func folderFixture(t *testing.T) (root, note string) {
	t.Helper()
	root = filepath.Join(absBase, "Stuff")
	sub := filepath.Join(root, "Sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	note = filepath.Join(root, "note.txt")
	if err := os.WriteFile(note, []byte("note"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "deep.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, note
}

// TestViewFolderPanelListsContents 验证点击文件夹条目以指针为中心展开内容，可以进入
// 子目录、返回上级，点击文件则用系统默认程序打开它。
func TestViewFolderPanelListsContents(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	root, note := folderFixture(t)

	// The third cell, so the panel fits around the pointer without being
	// pushed back inside the window.
	app.store.Apps["stuff"] = &AppItem{GUID: "stuff", Name: "Stuff", Path: root}
	app.store.Categories[0].Slots = []*string{ptrString("a"), ptrString("b"), ptrString("stuff")}
	tt.Frame()
	cell, ok := tt.Find("Stuff")
	if !ok {
		t.Fatalf("no folder cell, texts %q", tt.Texts())
	}

	if err := tt.Click("Stuff"); err != nil {
		t.Fatalf("click the folder: %v", err)
	}
	if !tt.HasText("Folder contents") {
		t.Fatalf("a click on a folder must open its panel, texts %q", tt.Texts())
	}
	panel, ok := tt.Access("Folder contents")
	if !ok {
		t.Fatalf("the panel must be named, texts %q", tt.Texts())
	}
	dx := panel.Bounds.X + panel.Bounds.W/2 - (cell.X + cell.W/2)
	dy := panel.Bounds.Y + panel.Bounds.H/2 - (cell.Y + cell.H/2)
	if dx < -2 || dx > 2 || dy < -2 || dy > 2 {
		t.Fatalf("the panel must center on the click, off by (%v, %v)", dx, dy)
	}
	insideWindow(t, panel.Bounds, 720, 460)
	for _, name := range []string{"Sub", "note.txt"} {
		if !tt.HasText(name) {
			t.Fatalf("the panel must list %q, texts %q", name, tt.Texts())
		}
	}

	// A folder of the panel is entered.
	if err := tt.Click("Sub"); err != nil {
		t.Fatalf("enter the subfolder: %v", err)
	}
	if !tt.HasText("deep.txt") || tt.HasText("note.txt") {
		t.Fatalf("the panel must list the subfolder alone, texts %q", tt.Texts())
	}

	// And the way back stops at the item's own folder.
	if err := tt.Click("Parent folder"); err != nil {
		t.Fatalf("go back up: %v", err)
	}
	if !tt.HasText("note.txt") || tt.HasText("deep.txt") {
		t.Fatalf("going up must list the folder again, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("Parent folder"); ok {
		t.Fatal("the item's own folder must have no way up")
	}

	// A file of the panel opens with the system's handler, and the panel goes.
	old := openEntryFile
	openEntryFile = func(path string) error {
		if path != note {
			t.Errorf("the panel must open %q, got %q", note, path)
		}
		return nil
	}
	t.Cleanup(func() { openEntryFile = old })
	if err := tt.Click("note.txt"); err != nil {
		t.Fatalf("open a file of the panel: %v", err)
	}
	if tt.HasText("Folder contents") {
		t.Fatalf("the panel must close after an entry opens, texts %q", tt.Texts())
	}
}

// TestViewFolderPanelRightClickMenu 验证面板条目右键调用 shell 菜单，菜单关闭后
// 面板保留并重新读取目录。
func TestViewFolderPanelRightClickMenu(t *testing.T) {
	if !canUseShellActions(filepath.Join(absBase, "note.txt")) {
		t.Skip("no shell menu on this platform")
	}
	app, _, tt := newTestView(t, 720, 460)
	root, note := folderFixture(t)
	app.store.Apps["stuff"] = &AppItem{GUID: "stuff", Name: "Stuff", Path: root}
	app.store.Categories[0].Slots = []*string{ptrString("a"), ptrString("b"), ptrString("stuff")}
	tt.Frame()

	if err := tt.Click("Stuff"); err != nil {
		t.Fatalf("open the folder panel: %v", err)
	}
	old := showEntryMenu
	got := ""
	showEntryMenu = func(path string) error {
		got = path
		return nil
	}
	t.Cleanup(func() { showEntryMenu = old })

	if err := tt.RightClick("note.txt"); err != nil {
		t.Fatalf("right click the entry: %v", err)
	}
	if got != note {
		t.Fatalf("the shell menu got %q, want %q", got, note)
	}
	if !tt.HasText("Folder contents") || !tt.HasText("note.txt") {
		t.Fatalf("the panel must stay open after the menu, texts %q", tt.Texts())
	}
}

// TestViewFolderPanelHidesSystemFiles 验证文件夹面板不列出 desktop.ini、
// Thumbs.db 这类系统文件，普通文件照常显示。
func TestViewFolderPanelHidesSystemFiles(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	root, _ := folderFixture(t)
	for _, name := range []string{"desktop.ini", "Thumbs.db"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("system"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app.store.Apps["stuff"] = &AppItem{GUID: "stuff", Name: "Stuff", Path: root}
	app.store.Categories[0].Slots = []*string{ptrString("stuff")}
	tt.Frame()

	if err := tt.Click("Stuff"); err != nil {
		t.Fatalf("open the folder: %v", err)
	}
	for _, name := range []string{"desktop.ini", "Thumbs.db"} {
		if tt.HasText(name) {
			t.Fatalf("the panel must hide %q, texts %q", name, tt.Texts())
		}
	}
	if !tt.HasText("note.txt") {
		t.Fatalf("the panel must keep ordinary files, texts %q", tt.Texts())
	}
}

// TestViewFolderPanelExplains 验证空文件夹说明它是空的，读不了的文件夹说明读不了。
func TestViewFolderPanelExplains(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		app, _, tt := newTestView(t, 720, 460)
		empty := filepath.Join(absBase, "Empty")
		if err := os.MkdirAll(empty, 0o755); err != nil {
			t.Fatal(err)
		}
		app.store.Apps["empty"] = &AppItem{GUID: "empty", Name: "Empty", Path: empty}
		app.store.Categories[0].Slots = []*string{ptrString("empty")}
		tt.Frame()
		if err := tt.Click("Empty"); err != nil {
			t.Fatalf("open the empty folder: %v", err)
		}
		if !tt.HasText("This folder is empty") {
			t.Fatalf("an empty folder must say so, texts %q", tt.Texts())
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		app, _, tt := newTestView(t, 720, 460)
		root, _ := folderFixture(t)
		app.store.Apps["stuff"] = &AppItem{GUID: "stuff", Name: "Stuff", Path: root}
		app.store.Categories[0].Slots = []*string{ptrString("stuff")}
		tt.Frame()
		if err := tt.Click("Stuff"); err != nil {
			t.Fatalf("open the folder: %v", err)
		}
		// The listed subfolder is gone by the time it is entered.
		if err := os.RemoveAll(filepath.Join(root, "Sub")); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click("Sub"); err != nil {
			t.Fatalf("enter the missing folder: %v", err)
		}
		if !tt.HasText("Cannot read this folder") {
			t.Fatalf("a folder that cannot be read must say so, texts %q", tt.Texts())
		}
	})
}

// insideWindow checks a panel's box stays in the window's DIPs.
func insideWindow(t *testing.T, r ui.Rect, width, height float32) {
	t.Helper()
	if r.X < 0 || r.Y < 0 || r.X+r.W > width || r.Y+r.H > height {
		t.Fatalf("the panel must stay in the window, box %+v of %v×%v", r, width, height)
	}
}

// TestViewFolderPanelStaysInWindow 验证靠窗口右侧和底部的条目展开时，面板仍完整显示在
// 窗口内。
func TestViewFolderPanelStaysInWindow(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	root, _ := folderFixture(t)

	app.store.Apps["stuff"] = &AppItem{GUID: "stuff", Name: "Stuff", Path: root}
	slots := make([]*string, 0, 16)
	for i := 0; i < 15; i++ {
		guid := fmt.Sprintf("fill%02d", i)
		app.store.Apps[guid] = &AppItem{GUID: guid, Name: "Fill", Path: "C:/tools/fill.exe"}
		slots = append(slots, ptrString(guid))
	}
	app.store.Categories[0].Slots = append(slots, ptrString("stuff"))
	tt.Frame()

	if err := tt.Click("Stuff"); err != nil {
		t.Fatalf("click the folder at the window's end: %v", err)
	}
	panel, ok := tt.Access("Folder contents")
	if !ok {
		t.Fatalf("the panel must open, texts %q", tt.Texts())
	}
	insideWindow(t, panel.Bounds, 720, 460)
}

// TestViewFolderPanelCloses 验证 Escape 和点击面板外面都关闭面板，条目被移除时面板
// 也随着单元格一起消失。
func TestViewFolderPanelCloses(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	root, _ := folderFixture(t)

	app.store.Apps["stuff"] = &AppItem{GUID: "stuff", Name: "Stuff", Path: root}
	app.store.Categories[0].Slots = []*string{ptrString("stuff")}
	app.store.Categories = append(app.store.Categories, CategoryNode{
		GUID: "second", Name: "Second", Slots: []*string{},
	})
	tt.Frame()

	hoverItem := func() {
		if err := tt.Click("Stuff"); err != nil {
			t.Fatalf("open the folder: %v", err)
		}
		if v.folderGUID == "" || !tt.HasText("Folder contents") {
			t.Fatalf("the folder panel must be open, texts %q", tt.Texts())
		}
	}

	hoverItem()
	tt.Key(0, ui.KeyEscape)
	if v.folderGUID != "" || tt.HasText("Folder contents") {
		t.Fatalf("Escape must close the panel, texts %q", tt.Texts())
	}

	hoverItem()
	tt.ClickAt(700, 420)
	if v.folderGUID != "" || tt.HasText("Folder contents") {
		t.Fatalf("a press outside must close the panel, texts %q", tt.Texts())
	}

	hoverItem()
	delete(app.store.Apps, "stuff")
	app.store.Categories[0].Slots = nil
	tt.Frame()
	if v.folderGUID != "" || tt.HasText("Folder contents") {
		t.Fatalf("a removed item must close the panel, texts %q", tt.Texts())
	}
}
