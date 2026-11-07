package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

// cLaunchFixture is a cut-down CLaunch.ini: two pages that migrate, one that
// holds nothing but an unresolvable special folder, a submenu and the button
// kinds the importer maps.
const cLaunchFixture = `; CLaunch 4 writes this
[Pages]
Count=3

[Page000]
Name=OS
Count=8

[Btn000]
Position=0
Type=00000001
Name=Notepad
File=../Tools/notepad.exe
Parameter=notes.txt
Directory=../Tools

[Btn001]
Position=1
Type=00000004
Name=Settings
File=ms-settings:colors

[Btn002]
Position=2
Type=00002011
Name=This PC

[Btn003]
Position=3
Type=00000007
Name=Calculator
File=Microsoft.WindowsCalculator_8wekyb3d8bbwe!App

[Btn004]
Position=4
Type=00400001
Name=Elevated
File=../Tools/elevated.exe

[Btn005]
Position=5
Type=00000006
Name=Control Panel
File=14001f706806ee260aa0d7449371beb064c986830000

[Btn006]
Position=6
Type=00000010
Name=Remote

[Btn007]
Position=7
Type=00004003
Name=Exit CLaunch

[SubMenus]
Count=1

[SubMenu000]
Page=0000
Button=6
Count=2

[Btn000]
Type=00011000
Name=Shutdown

[Btn001]
Type=00011001
Name=Restart

[Page001]
Name=Empty
Count=1

[Btn000]
Type=00000006
Name=Documents
File=14001f7068060000

[Page002]
Name=Tools
Count=2

[Btn000]
Type=00000001
Name=Everything
File=../Tools/everything.exe

[Btn001]
Type=00000001
Name=Alpha
File=../Tools/alpha.exe
`

// utf16LEBytes encodes s as the UTF-16LE with a BOM that CLaunch writes.
func utf16LEBytes(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, 2+2*len(units))
	out = append(out, 0xFF, 0xFE)
	for _, u := range units {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

// TestCLaunchDecodesItsEncodings 验证带 BOM 的 UTF-16LE 和 UTF-8 都能读成同一段
// 文本。
func TestCLaunchDecodesItsEncodings(t *testing.T) {
	text := "[Pages]\r\nCount=1\r\n[Page000]\r\nName=工具\r\n"
	if got := decodeCLaunch(utf16LEBytes(text)); got != text {
		t.Fatalf("UTF-16 with a BOM must decode to the same text, got %q", got)
	}
	if got := decodeCLaunch([]byte(text)); got != text {
		t.Fatalf("UTF-8 must decode to the same text, got %q", got)
	}
}

// TestReadCLaunchIniPagesAndSubmenus 验证重复的 [Btn###] 分节按顺序归属各自的页面，
// 子菜单按 Page/Button 挂回父按钮。
func TestReadCLaunchIniPagesAndSubmenus(t *testing.T) {
	pages := readCLaunchIni(cLaunchFixture)
	if len(pages) != 3 {
		t.Fatalf("three pages must be read, got %d", len(pages))
	}
	if pages[0].Name != "OS" || pages[1].Name != "Empty" || pages[2].Name != "Tools" {
		t.Fatalf("the page names must be kept, got %q %q %q", pages[0].Name, pages[1].Name, pages[2].Name)
	}
	if len(pages[0].Buttons) != 8 {
		t.Fatalf("page OS has 8 buttons, got %d", len(pages[0].Buttons))
	}
	if len(pages[0].Buttons[6].Menu) != 2 {
		t.Fatalf("the submenu must attach to button 6, got %+v", pages[0].Buttons[6])
	}
	if pages[0].Buttons[6].Menu[0].Name != "Shutdown" {
		t.Fatalf("the submenu's own buttons must be read, got %+v", pages[0].Buttons[6].Menu)
	}
}

// TestBuildCLaunchImport 验证页面转新页面、按钮转条目：相对路径按 CLaunch.exe
// 的目录解析，URL、Store 应用、已知文件夹、管理员位和电源项各走各的映射，
// 二进制 ID list 与退出项被跳过，全空的页面不建新页面。
func TestBuildCLaunchImport(t *testing.T) {
	base := t.TempDir()
	imp := buildCLaunchImport(readCLaunchIni(cLaunchFixture), base, true, false)

	if len(imp.Tabs) != 2 {
		t.Fatalf("two pages have items, got %d tabs: %+v", len(imp.Tabs), imp.Tabs)
	}
	if imp.Tabs[0].Name != "OS" || imp.Tabs[1].Name != "Tools" {
		t.Fatalf("the tabs must be named after the pages, got %q and %q", imp.Tabs[0].Name, imp.Tabs[1].Name)
	}
	if len(imp.Tabs[0].Items) != 7 {
		t.Fatalf("OS must have 7 items (the submenu takes its button's place), got %d", len(imp.Tabs[0].Items))
	}
	if imp.Skipped != 3 {
		t.Fatalf("the two ID lists and Exit CLaunch are skipped, got %d", imp.Skipped)
	}

	byName := map[string]*AppItem{}
	for _, item := range imp.Tabs[0].Items {
		byName[item.Name] = item
	}
	notepad := byName["Notepad"]
	if notepad == nil {
		t.Fatalf("no Notepad item: %+v", imp.Tabs[0].Items)
	}
	if want := filepath.Join(filepath.Dir(base), "Tools", "notepad.exe"); notepad.Path != want {
		t.Fatalf("a relative path must resolve against CLaunch.exe's folder, got %q want %q", notepad.Path, want)
	}
	if notepad.Args != "notes.txt" || notepad.WorkingDir != filepath.Join(filepath.Dir(base), "Tools") {
		t.Fatalf("parameters and working folder must be kept, got %+v", notepad)
	}
	if item := byName["Settings"]; item == nil || item.Path != "ms-settings:colors" {
		t.Fatalf("a URL must be kept as written, got %+v", item)
	}
	if item := byName["This PC"]; item == nil || item.Path != "::{20D04FE0-3AEA-1069-A2D8-08002B30309D}" {
		t.Fatalf("a known folder must become its shell namespace, got %+v", item)
	}
	if item := byName["Calculator"]; item == nil || item.Path != `shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App` {
		t.Fatalf("a Store app must become its Applications folder item, got %+v", item)
	}
	if item := byName["Elevated"]; item == nil || !item.RunAsAdmin {
		t.Fatalf("the privilege option must become RunAsAdmin, got %+v", item)
	}
	if item := byName["Shutdown"]; item == nil || !strings.HasSuffix(item.Path, "shutdown.exe") || item.Args != "/s /t 0" {
		t.Fatalf("a power button must become shutdown.exe, got %+v", item)
	}
	if item := byName["Restart"]; item == nil || !strings.HasSuffix(item.Path, "shutdown.exe") || item.Args != "/r /t 0" {
		t.Fatalf("a restart button must become shutdown.exe, got %+v", item)
	}
	if len(imp.Tabs[1].Items) != 2 {
		t.Fatalf("Tools must have its two items, got %d", len(imp.Tabs[1].Items))
	}
}

// TestCLaunchBase 验证相对路径的基准优先取 CLaunch.exe 所在的目录，其次是 Data
// 目录的上一级。
func TestCLaunchBase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLaunch.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ini := filepath.Join(dir, "Data", "CLaunch.ini")
	if got := cLaunchBase(ini); got != dir {
		t.Fatalf("the folder of CLaunch.exe must be the base, got %q want %q", got, dir)
	}

	// No CLaunch.exe, but the ini sits in CLaunch's Data folder.
	other := t.TempDir()
	if got := cLaunchBase(filepath.Join(other, "Data", "CLaunch.ini")); got != other {
		t.Fatalf("a Data folder's parent must be the base, got %q want %q", got, other)
	}

	// An ini copied somewhere else resolves against its own folder.
	loose := t.TempDir()
	if got := cLaunchBase(filepath.Join(loose, "CLaunch.ini")); got != loose {
		t.Fatalf("a loose ini must use its folder, got %q want %q", got, loose)
	}
}

// TestCLaunchImportAddsTabs 验证导入的页面追加到 store，条目进入公共池并登记
// 运行时。
func TestCLaunchImportAddsTabs(t *testing.T) {
	app := newTestApp(t)
	imp := cLaunchImport{
		Tabs: []cLaunchTab{
			{Name: "OS", Items: []*AppItem{{Name: "A", Path: "C:/a.exe"}, {Name: "B", Path: "C:/b.exe"}}},
			{Name: "Tools", Items: []*AppItem{{Name: "C", Path: "C:/c.exe"}}},
		},
	}
	before := len(app.store.Categories)
	first, tabs, items := app.addCLaunchImport(imp)
	if tabs != 2 || items != 3 || first == "" {
		t.Fatalf("the import must add 2 tabs and 3 items, got %d %d first %q", tabs, items, first)
	}
	if len(app.store.Categories) != before+2 {
		t.Fatalf("two tabs must be appended, got %d categories", len(app.store.Categories))
	}
	added := app.store.Categories[before:]
	if added[0].Name != "OS" || len(added[0].Slots) != 2 || added[1].Name != "Tools" || len(added[1].Slots) != 1 {
		t.Fatalf("the tabs must keep their names and items, got %+v", added)
	}
	for _, guid := range []string{*added[0].Slots[0], *added[0].Slots[1], *added[1].Slots[0]} {
		if app.item(guid) == nil {
			t.Fatalf("item %q must be in the store", guid)
		}
		if _, ok := app.runtimeStats[guid]; !ok {
			t.Fatalf("item %q must have a runtime", guid)
		}
	}
	data, err := os.ReadFile(filepath.Join(absBase, saveFile))
	if err != nil {
		t.Fatalf("the import must write the store: %v", err)
	}
	if !strings.Contains(string(data), `"OS"`) || !strings.Contains(string(data), `"Tools"`) {
		t.Fatalf("the written store must hold the new tabs, got %s", data)
	}
}

// TestShellTargetsAreNotPaths 验证 URL、shell 目标和 shell 命名空间不会被当成
// 相对路径，盘符路径仍按文件处理。
func TestShellTargetsAreNotPaths(t *testing.T) {
	for _, c := range []struct {
		path string
		want bool
	}{
		{"ms-settings:colors", true},
		{"shell:AppsFolder\\X", true},
		{"::{20D04FE0-3AEA-1069-A2D8-08002B30309D}", true},
		{"https://example.com", true},
		{"mailto:someone@example.com", true},
		{`C:\Windows\notepad.exe`, false},
		{"C:/tools/alpha.exe", false},
		{`\\server\share`, false},
		{`relative\file.exe`, false},
		{"", false},
	} {
		if got := isShellTarget(c.path); got != c.want {
			t.Fatalf("isShellTarget(%q) = %v, want %v", c.path, got, c.want)
		}
	}

	if got := absPath("ms-settings:colors"); got != "ms-settings:colors" {
		t.Fatalf("a shell target must not be joined to the data folder, got %q", got)
	}
	if got := normalizePath("https://example.com/a/b"); got != "https://example.com/a/b" {
		t.Fatalf("a URL must not have its slashes rewritten, got %q", got)
	}

	app := newTestApp(t)
	app.store.Apps["url"] = &AppItem{GUID: "url", Name: "Settings", Path: "ms-settings:colors", RunAsAdmin: true}
	plan, err := app.planLaunch("url")
	if err != nil {
		t.Fatal(err)
	}
	if plan.path != "ms-settings:colors" || plan.track || plan.elevated {
		t.Fatalf("a shell target must be opened as written, without tracking or elevation, got %+v", plan)
	}
}

// TestPlanLaunchReportsMissingPath 验证启动前就发现路径失效，而不是交给系统报错。
func TestPlanLaunchReportsMissingPath(t *testing.T) {
	app := newTestApp(t)
	app.store.Apps["gone"] = &AppItem{GUID: "gone", Name: "Gone", Path: "C:/tools/gone.exe"}
	if _, err := app.planLaunch("gone"); err == nil {
		t.Fatal("a missing path must be reported before the launch")
	}
	app.store.Apps["none"] = &AppItem{GUID: "none", Name: "None"}
	if _, err := app.planLaunch("none"); err == nil {
		t.Fatal("an item without a path or folder must be reported")
	}
}

// TestViewSettingsShowsCLaunchMigration 验证设置的数据分组最下面是迁移那一条。
func TestViewSettingsShowsCLaunchMigration(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	settings := newTestSettings(t, v)
	for _, text := range []string{"Migrate from CLaunch", "Choose CLaunch.ini…", "Create new pages from a CLaunch.ini"} {
		if !settings.HasText(text) {
			t.Fatalf("the data group must show %q, texts %q", text, settings.Texts())
		}
	}
}

// TestViewAppliesCLaunchImport 验证导入完成后切到第一个新页面，并把结果写在
// 设置的迁移行上。
func TestViewAppliesCLaunchImport(t *testing.T) {
	app, v, tt := newTestView(t, 720, 460)
	imp := buildCLaunchImport(readCLaunchIni(cLaunchFixture), t.TempDir(), true, false)
	v.applyCLaunchImport(imp)

	tab := app.tab(v.prefs.ActiveTab)
	if tab == nil || tab.Name != "OS" {
		t.Fatalf("the first imported tab must become active, got %+v (active %q)", tab, v.prefs.ActiveTab)
	}
	if !strings.Contains(v.migrateStatus, "Imported") || !strings.Contains(v.migrateStatus, "skipped") {
		t.Fatalf("the status must say what was imported and skipped, got %q", v.migrateStatus)
	}
	tt.Frame()
	if !tt.HasText("OS") || !tt.HasText("Tools") {
		t.Fatalf("the tab strip must show the imported tabs, texts %q", tt.Texts())
	}
	settings := newTestSettings(t, v)
	if !settings.HasText(v.migrateStatus) {
		t.Fatalf("the settings row must show %q, texts %q", v.migrateStatus, settings.Texts())
	}
}
