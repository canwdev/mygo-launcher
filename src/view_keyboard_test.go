package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// addPage appends a page holding one item named name.
func addPage(app *App, guid, name, itemGUID string) {
	app.store.Apps[itemGUID] = &AppItem{GUID: itemGUID, Name: name, Path: "C:/tools/" + itemGUID + ".exe"}
	app.store.Categories = append(app.store.Categories, CategoryNode{
		GUID:  guid,
		Name:  name,
		Slots: []*string{ptrString(itemGUID)},
	})
}

// TestKeyboardSelectsItems 验证方向键选中条目，且最后一条按右切到下一页。
func TestKeyboardSelectsItems(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	tt.Key(0, ui.KeyDown)
	if v.keySel != 0 {
		t.Fatalf("Down chooses the first item, keySel=%d", v.keySel)
	}
	tt.Key(0, ui.KeyRight)
	if v.keySel != 1 {
		t.Fatalf("Right moves to the next item, keySel=%d", v.keySel)
	}

	addPage(app, "t2", "Second", "c")
	tt.Frame()
	tt.Key(0, ui.KeyRight) // the last item: turn the page
	if app.store.Categories[1].GUID != v.prefs.ActiveTab {
		t.Fatalf("Right on the last item must show the next page, active=%q", v.prefs.ActiveTab)
	}
	if v.keySel != 0 {
		t.Fatalf("a page turned by the keyboard chooses its first item, keySel=%d", v.keySel)
	}
	tt.Key(0, ui.KeyLeft) // the first item: back a page
	if v.prefs.ActiveTab != app.store.Categories[0].GUID || v.keySel != 0 {
		t.Fatalf("Left on the first item must show the previous page, active=%q keySel=%d", v.prefs.ActiveTab, v.keySel)
	}
}

// TestKeyboardPageKeys 验证 Tab、Home/End、PgUp/PgDn 和数字键切换页面。
func TestKeyboardPageKeys(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	addPage(app, "t2", "Second", "c")
	addPage(app, "t3", "Third", "d")
	tt.Frame()

	tt.Key(0, ui.KeyTab)
	if v.prefs.ActiveTab != "t2" {
		t.Fatalf("Tab must show the next page, active=%q", v.prefs.ActiveTab)
	}
	tt.Key(0, ui.KeyEnd)
	if v.prefs.ActiveTab != "t3" {
		t.Fatalf("End must show the last page, active=%q", v.prefs.ActiveTab)
	}
	tt.Key(0, ui.KeyHome)
	if v.prefs.ActiveTab != app.store.Categories[0].GUID {
		t.Fatalf("Home must show the first page, active=%q", v.prefs.ActiveTab)
	}
	tt.Key(0, ui.KeyPageDown)
	if v.prefs.ActiveTab != "t2" {
		t.Fatalf("PageDown must show the next page, active=%q", v.prefs.ActiveTab)
	}
	tt.Key(0, ui.KeyPageUp)
	if v.prefs.ActiveTab != app.store.Categories[0].GUID {
		t.Fatalf("PageUp must show the previous page, active=%q", v.prefs.ActiveTab)
	}
	tt.Key(0, ui.Key3)
	if v.prefs.ActiveTab != "t3" {
		t.Fatalf("3 must show the third page, active=%q", v.prefs.ActiveTab)
	}
}

// TestKeyboardLettersSelectBySlot 验证字母按槽位选中，空白槽位占字母但不动作。
func TestKeyboardLettersSelectBySlot(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	app.store.Categories[0].Slots = []*string{ptrString("a"), nil, ptrString("b")}
	tt.Frame()

	tt.Key(0, ui.KeyQ)
	if v.keySel != 0 {
		t.Fatalf("Q chooses the first slot, keySel=%d", v.keySel)
	}
	tt.Key(0, ui.KeyW) // the blank slot: nothing to choose
	if v.keySel != 0 {
		t.Fatalf("a blank slot leaves the choice alone, keySel=%d", v.keySel)
	}
	tt.Key(0, ui.KeyE)
	if v.keySel != 2 {
		t.Fatalf("E chooses the third slot, keySel=%d", v.keySel)
	}
}

// TestKeyboardEnterOpensFolder 验证回车打开选中文件夹的面板。
func TestKeyboardEnterOpensFolder(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	root, _ := folderFixture(t)
	app.store.Apps["stuff"] = &AppItem{GUID: "stuff", Name: "Stuff", Path: root}
	app.store.Categories[0].Slots = []*string{ptrString("stuff")}
	tt.Frame()

	tt.Key(0, ui.KeyQ)
	tt.Key(0, ui.KeyEnter)
	if v.folderGUID != "stuff" {
		t.Fatalf("Enter on a folder must open its panel, folderGUID=%q", v.folderGUID)
	}
}

// TestKeyboardOpensSearch 验证 Ctrl+F 打开搜索。
func TestKeyboardOpensSearch(t *testing.T) {
	_, v, tt := newTestView(t, 720, 460)
	tt.Key(ui.Cmd, ui.KeyF)
	if !v.searchOpen {
		t.Fatal("Ctrl+F must open the search")
	}
}

// TestTooltipsShowKeys 验证悬停图标按钮和条目时显示名称与快捷键。
func TestTooltipsShowKeys(t *testing.T) {
	_, _, tt := newTestView(t, 720, 460)

	hover := func(label, want string) {
		t.Helper()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q to hover, texts %q", label, tt.Texts())
		}
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Advance(700 * time.Millisecond)
		tt.Frame()
		if !tt.HasText(want) {
			t.Fatalf("hovering %q must show %q, texts %q", label, want, tt.Texts())
		}
	}
	hover("Search", "Search (Ctrl+F)")
	hover("Settings", "Settings (Ctrl+,)")
}

// TestHoverChoosesAndArrowsContinue 验证悬停条目等同于选中，方向键从悬停项继续，
// 不会因为指针还停在原处而被拉回。
func TestHoverChoosesAndArrowsContinue(t *testing.T) {
	_, v, tt := newTestView(t, 720, 460)
	alpha, ok := tt.Find("Alpha")
	if !ok {
		t.Fatalf("no Alpha cell, texts %q", tt.Texts())
	}
	tt.Move(alpha.X+alpha.W/2, alpha.Y+alpha.H/2)
	tt.Frame()
	if v.keySel != 0 {
		t.Fatalf("hovering the first item must choose it, keySel=%d", v.keySel)
	}

	tt.Key(0, ui.KeyRight)
	if v.keySel != 1 {
		t.Fatalf("arrows must go on from the hovered item, keySel=%d", v.keySel)
	}
	tt.Frame()
	if v.keySel != 1 {
		t.Fatalf("the resting pointer must not pull the choice back, keySel=%d", v.keySel)
	}

	// Moving the pointer again hands the keys over to the mouse.
	tt.Move(alpha.X+alpha.W/2-2, alpha.Y+alpha.H/2)
	tt.Frame()
	if v.keySel != 0 {
		t.Fatalf("moving the pointer must choose the hovered item again, keySel=%d", v.keySel)
	}
}

// TestKeyboardEnterReachesChosenItem 验证用键盘导航后，回车作用于选中的条目，
// 而不是之前点过的按钮。
func TestKeyboardEnterReachesChosenItem(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	root, _ := folderFixture(t)
	app.store.Apps["stuff"] = &AppItem{GUID: "stuff", Name: "Stuff", Path: root}
	app.store.Categories[0].Slots = []*string{ptrString("stuff")}
	tt.Frame()

	// Clicking a page chip focuses it, as a click on the settings button would.
	if err := tt.Click("Default"); err != nil {
		t.Fatalf("click the page chip: %v", err)
	}
	if !tt.Focused("Default") {
		t.Fatal("the clicked page chip must take the focus")
	}
	tt.Key(0, ui.KeyDown)
	if tt.Focused("Default") {
		t.Fatal("keyboard navigation must move the focus off the clicked button")
	}
	tt.Key(0, ui.KeyEnter)
	if v.folderGUID != "stuff" {
		t.Fatalf("Enter must reach the chosen item, folderGUID=%q", v.folderGUID)
	}
}

// TestTabHoverDoesNotTurnBack 验证指针停在页面胶囊上时用键盘切页，悬停不会把页面
// 切回去。
func TestTabHoverDoesNotTurnBack(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	addPage(app, "t2", "Second", "c")
	tt.Frame()

	chip, ok := tt.Find("Default")
	if !ok {
		t.Fatalf("no page chip to hover, texts %q", tt.Texts())
	}
	tt.Move(chip.X+chip.W/2, chip.Y+chip.H/2)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	if v.prefs.ActiveTab != "t2" {
		t.Fatalf("Tab must show the next page, active=%q", v.prefs.ActiveTab)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if v.prefs.ActiveTab != "t2" {
		t.Fatalf("the hovered chip must not turn the page back, active=%q", v.prefs.ActiveTab)
	}
}

// TestKeyboardWaitsForOverlays 验证搜索打开时方向键不选择条目，Esc 只关闭搜索。
func TestKeyboardWaitsForOverlays(t *testing.T) {
	_, v, tt := newTestView(t, 720, 460)
	v.openSearch()
	tt.Frame()

	tt.Key(0, ui.KeyDown)
	if v.keySel != -1 {
		t.Fatalf("the search takes the arrows, keySel=%d", v.keySel)
	}
	tt.Key(0, ui.KeyEscape)
	if v.searchOpen {
		t.Fatal("Escape must close the search, not the window")
	}
}
