package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestPNG writes a PNG of the given size.
func writeTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := range img.Pix {
		img.Pix[i] = 0x80
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCustomIconCopiesAndShrinks 验证选择的图片被复制到 custom-icons、缩到上限，
// 且再次选择同一个文件时不再复制。
func TestCustomIconCopiesAndShrinks(t *testing.T) {
	app := newTestApp(t)
	src := filepath.Join(t.TempDir(), "big.png")
	writeTestPNG(t, src, 600, 300)

	if allowedIcon(src) {
		t.Fatal("an icon outside the app's folders must not be allowed")
	}
	icon, err := app.customIcon(src, 0)
	if err != nil {
		t.Fatalf("copy the icon: %v", err)
	}
	if !isCustomIcon(icon) {
		t.Fatalf("the icon must live in custom-icons, got %q", icon)
	}
	data, err := os.ReadFile(absPath(icon))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the copied icon must be a PNG: %v", err)
	}
	if b := img.Bounds(); max(b.Dx(), b.Dy()) > maxIconSide {
		t.Fatalf("the icon must be drawn down, got %v", b)
	}

	again, err := app.customIcon(icon, 0)
	if err != nil || again != icon {
		t.Fatalf("an icon already in custom-icons must stay: got %q, %v", again, err)
	}
	entries, err := os.ReadDir(filepath.Join(absBase, customIconsDir))
	if err != nil || len(entries) != 1 {
		t.Fatalf("the icon must be copied once, entries %v, err %v", entries, err)
	}
}

// TestBuildItemUsesImageContent 验证添加图片时用图片本身作为图标，而不是文件类型
// 的图标。
func TestBuildItemUsesImageContent(t *testing.T) {
	newTestApp(t)
	src := filepath.Join(t.TempDir(), "poster.png")
	writeTestPNG(t, src, 32, 32)

	item := buildItem(src, true, true)
	if item == nil {
		t.Fatal("the image must become an item")
	}
	if !isAutoIcon(item.Icon) {
		t.Fatalf("the image's own content must be its icon, got %q", item.Icon)
	}
	if _, err := os.Stat(absPath(item.Icon)); err != nil {
		t.Fatalf("the icon must be written: %v", err)
	}
	if iconSource(item.Icon) == nil {
		t.Fatal("the icon must decode")
	}
}

// TestCustomIconExtractsFromProgram 验证选择程序时把它的图标取进 custom-icons，
// 序号越界时报错。
func TestCustomIconExtractsFromProgram(t *testing.T) {
	app := newTestApp(t)
	exe := filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Skipf("no %s: %v", exe, err)
	}
	icon, err := app.customIcon(exe, 0)
	if err != nil {
		t.Fatalf("extract the program's icon: %v", err)
	}
	if !isCustomIcon(icon) {
		t.Fatalf("the icon must live in custom-icons, got %q", icon)
	}
	if _, err := os.Stat(absPath(icon)); err != nil {
		t.Fatalf("the icon must be written: %v", err)
	}
	if _, err := app.customIcon(exe, 9999); err == nil {
		t.Fatal("an index past the program's icons must fail")
	}
}

// TestUpdateIconTakesInExternalIcon 验证「更新图标」把路径外的图标复制并缩小进
// custom-icons，保留用户原文件；已经在里面的图标不再处理。
func TestUpdateIconTakesInExternalIcon(t *testing.T) {
	app := newTestApp(t)
	src := filepath.Join(t.TempDir(), "outside.png")
	writeTestPNG(t, src, 600, 300)
	app.store.Apps["x"] = &AppItem{GUID: "x", Name: "X", Path: `C:\tools\x.exe`, Icon: src}

	if err := app.updateIcon("x"); err != nil {
		t.Fatalf("update the icon: %v", err)
	}
	item := app.store.Apps["x"]
	if !isCustomIcon(item.Icon) {
		t.Fatalf("the icon must be taken into custom-icons, got %q", item.Icon)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("the user's own file must stay: %v", err)
	}
	icon := item.Icon
	if err := app.updateIcon("x"); err != nil {
		t.Fatalf("updating an icon already kept: %v", err)
	}
	if item.Icon != icon {
		t.Fatalf("an icon in custom-icons must not be processed again, got %q", item.Icon)
	}
}

// TestCustomIconRejectsBrokenImage 验证不是图片的文件给出错误，而不是存下坏图标。
func TestCustomIconRejectsBrokenImage(t *testing.T) {
	app := newTestApp(t)
	path := filepath.Join(t.TempDir(), "broken.png")
	if err := os.WriteFile(path, []byte("not a PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.customIcon(path, 0); err == nil {
		t.Fatal("a broken image must fail")
	}
}

// TestIconIndexReadsResourceIDs 验证序号接受负数：系统图标选择器把按资源 ID
// 存放的图标作为负数返回，文件类型也要包含带图标资源的程序与库。
func TestIconIndexReadsResourceIDs(t *testing.T) {
	cases := map[string]int{"": 0, "3": 3, "-101": -101, " x ": 0}
	for text, want := range cases {
		if got := iconIndex(text); got != want {
			t.Errorf("iconIndex(%q) = %d, want %d", text, got, want)
		}
	}
	for _, path := range []string{`C:\Windows\System32\imageres.dll`, "app.exe", "app.ico", "panel.cpl", "lib.ocx", "saver.scr", "icons.icl", "pack.mun"} {
		if !iconTakesIndex(path) {
			t.Errorf("%q holds icon resources and must take an index", path)
		}
	}
	for _, path := range []string{"", "a.txt", "image.png"} {
		if iconTakesIndex(path) {
			t.Errorf("%q has no icon resources to index", path)
		}
	}
}

// TestPathsExpandEnvironment 验证 %SystemRoot%\... 这类路径在图标、条目和工作
// 目录上都会先展开：系统图标选择器返回的就是这种写法。
func TestPathsExpandEnvironment(t *testing.T) {
	app := newTestApp(t)
	root := os.Getenv("SystemRoot")
	if root == "" {
		t.Skip("no SystemRoot")
	}
	exe := filepath.Join(root, "System32", "notepad.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Skipf("no %s: %v", exe, err)
	}
	envPath := `%SystemRoot%\System32\notepad.exe`

	if got := expandPath(envPath); got != exe {
		t.Fatalf("the variable must expand, got %q", got)
	}
	if got := expandPath(`%NO_SUCH_VARIABLE%\a.exe`); got != `%NO_SUCH_VARIABLE%\a.exe` {
		t.Fatalf("an unknown variable must stay as it is, got %q", got)
	}
	if got := absPath(envPath); got != exe {
		t.Fatalf("a path with variables must resolve as an absolute one, got %q", got)
	}
	item := &AppItem{GUID: "env", Name: "Env", Path: envPath}
	if !itemPresent(item) {
		t.Fatal("an item behind a variable must be present")
	}

	icon, err := app.customIcon(envPath, 0)
	if err != nil {
		t.Fatalf("read the icon behind the variable: %v", err)
	}
	if !isCustomIcon(icon) {
		t.Fatalf("the icon must live in custom-icons, got %q", icon)
	}
}

// TestPruneIconsRemovesUnused 验证删除条目时删掉无人引用的图标，共享的图标保留。
func TestPruneIconsRemovesUnused(t *testing.T) {
	app := newTestApp(t)
	v := newView(app)
	dir := filepath.Join(absBase, customIconsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(dir, "shared.png"), 16, 16)
	icon := "./" + filepath.ToSlash(filepath.Join(customIconsDir, "shared.png"))

	app.store.Apps["a"] = &AppItem{GUID: "a", Name: "A", Icon: icon}
	app.store.Apps["b"] = &AppItem{GUID: "b", Name: "B", Icon: icon}
	app.store.Categories[0].Slots = []*string{ptrString("a"), ptrString("b")}

	v.removeItem("a")
	if _, err := os.Stat(filepath.Join(dir, "shared.png")); err != nil {
		t.Fatalf("an icon another item uses must stay: %v", err)
	}
	v.removeItem("b")
	if _, err := os.Stat(filepath.Join(dir, "shared.png")); !os.IsNotExist(err) {
		t.Fatalf("an unreferenced icon must be removed, err %v", err)
	}
}

// TestAllowedIconKeepsOnlyAppIcons 验证只展示自动生成或自定义目录里的图标。
func TestAllowedIconKeepsOnlyAppIcons(t *testing.T) {
	allowed := []string{"", iconsDir + "/a.png", customIconsDir + "/a.png", "." + "/" + iconsDir + "/a.png"}
	for _, icon := range allowed {
		if !allowedIcon(icon) {
			t.Errorf("%q must be allowed", icon)
		}
	}
	denied := []string{`C:\pictures\a.png`, "../a.png", "custom-icons/a.png"}
	for _, icon := range denied {
		if allowedIcon(icon) {
			t.Errorf("%q must not be allowed", icon)
		}
	}
	if !strings.Contains(customIconsDir, "custom-icons") {
		t.Fatal("the custom folder must be custom-icons")
	}
}
