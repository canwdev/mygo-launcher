package main

import "testing"

// TestItemMenuOpensShellTargets 验证 shell 目标和网址的菜单显示「打开」，而不是
// 「打开所在文件夹」。
func TestItemMenuOpensShellTargets(t *testing.T) {
	app, _, tt := newTestView(t, 720, 460)
	app.store.Apps["b"].Path = "https://example.com"
	tt.Frame()

	if err := tt.RightClick("Beta"); err != nil {
		t.Fatalf("open the item menu: %v", err)
	}
	if !tt.HasText("Open") {
		t.Fatalf("a shell target must offer Open, texts %q", tt.Texts())
	}
	if tt.HasText("Open folder…") {
		t.Fatalf("a shell target has no folder to show, texts %q", tt.Texts())
	}
}

// TestItemMenuKeepsRevealForFiles 验证普通文件的菜单仍是「打开所在文件夹」。
func TestItemMenuKeepsRevealForFiles(t *testing.T) {
	_, _, tt := newTestView(t, 720, 460)
	if err := tt.RightClick("Alpha"); err != nil {
		t.Fatalf("open the item menu: %v", err)
	}
	if !tt.HasText("Open folder…") {
		t.Fatalf("a file must still be shown in its folder, texts %q", tt.Texts())
	}
}

// TestItemMenuEndsWithoutSeparator 验证没有 shell 菜单和属性的条目不会以分割线
// 结尾。
func TestItemMenuEndsWithoutSeparator(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)
	item := app.store.Apps["b"]
	item.Path = `shell:AppsFolder\Foo!App`
	entries := v.itemMenu(item, app.store.Categories[0].GUID, 1)
	if len(entries) == 0 || entries[len(entries)-1].label == "" {
		t.Fatalf("the menu must not end with a separator: %+v", entries)
	}
}

// TestItemDialogActions 验证编辑已有条目时，更新图标和路径转换在详情弹窗里；
// 新建条目时没有。
func TestItemDialogActions(t *testing.T) {
	_, _, tt := newTestView(t, 720, 520)
	if err := tt.RightClick("Alpha"); err != nil {
		t.Fatalf("open the item menu: %v", err)
	}
	if err := tt.Click("Edit"); err != nil {
		t.Fatalf("open the editor: %v", err)
	}
	for _, want := range []string{"Update icon", "Convert to absolute path", "Convert to relative path"} {
		if !tt.HasText(want) {
			t.Fatalf("the editor must offer %q, texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatalf("close the editor: %v", err)
	}
	tt.Frame()

	if err := tt.Click("Add"); err != nil {
		t.Fatalf("open the add menu: %v", err)
	}
	if err := tt.Click("Create…"); err != nil {
		t.Fatalf("open the create dialog: %v", err)
	}
	for _, unwanted := range []string{"Update icon", "Convert to absolute path", "Convert to relative path"} {
		if tt.HasText(unwanted) {
			t.Fatalf("a new item has no stored icon or path to change, texts %q", tt.Texts())
		}
	}
	tt.Click("Cancel")
}
