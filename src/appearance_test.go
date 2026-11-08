package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// writePNG writes a picture filled with c, for the appearance tests.
func writePNG(t *testing.T, path string, w, h int, c color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// TestCustomBackgroundStoresAndPrunes 验证背景图被收进 custom-icons，并且不会被
// 图标清理删掉；无人引用的文件仍然会被删。
func TestCustomBackgroundStoresAndPrunes(t *testing.T) {
	app := newTestApp(t)
	src := filepath.Join(absBase, "wall.png")
	writePNG(t, src, 16, 8, color.NRGBA{R: 200, G: 40, B: 20, A: 255})

	stored, err := app.customBackground(src)
	if err != nil {
		t.Fatal(err)
	}
	if !isCustomIcon(stored) {
		t.Fatalf("the background must live in custom-icons, got %q", stored)
	}
	if _, err := os.Stat(dataPath(stored)); err != nil {
		t.Fatalf("the stored background must exist: %v", err)
	}
	prefs := loadPrefs()
	prefs.Background = stored
	savePrefs(prefs)

	unused := filepath.Join(dataBase(), customIconsDir, "unused.png")
	writePNG(t, unused, 2, 2, color.NRGBA{A: 255})
	app.pruneIcons()
	if _, err := os.Stat(dataPath(stored)); err != nil {
		t.Fatalf("pruning must keep the chosen background: %v", err)
	}
	if _, err := os.Stat(unused); !os.IsNotExist(err) {
		t.Fatalf("pruning must remove the unused icon, err=%v", err)
	}
}

// TestCustomBackgroundShrinks 验证大图按显示尺寸缩小、无透明时存成 JPEG，
// 并在加载时再受同一上限保护。
func TestCustomBackgroundShrinks(t *testing.T) {
	app := newTestApp(t)
	src := filepath.Join(absBase, "huge.png")
	writePNG(t, src, 2400, 1200, color.NRGBA{R: 30, G: 120, B: 200, A: 255})

	stored, err := app.customBackground(src)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(stored) != ".jpg" {
		t.Fatalf("an opaque picture must be kept as JPEG, got %q", stored)
	}
	f, err := os.Open(dataPath(stored))
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	side := app.backgroundTargetSide()
	if got := max(cfg.Width, cfg.Height); got != side {
		t.Fatalf("the longest side must shrink to %d, got %d (%dx%d)", side, got, cfg.Width, cfg.Height)
	}
	if cfg.Width != side || cfg.Height != side/2 {
		t.Fatalf("the picture must keep its aspect, got %dx%d", cfg.Width, cfg.Height)
	}

	// A large picture put in the folder by hand is bounded again as it loads.
	manual := filepath.Join(dataBase(), customIconsDir, "manual.png")
	writePNG(t, manual, 3000, 1000, color.NRGBA{R: 200, G: 80, B: 30, A: 255})
	v := newView(app)
	v.prefs.Background = "./" + customIconsDir + "/manual.png"
	bmp := v.backgroundBitmap()
	if bmp == nil {
		t.Fatal("the hand-placed background must load")
	}
	if w, h := bmp.Size(); max(w, h) != side {
		t.Fatalf("loading must bound the picture to %d, got %dx%d", side, w, h)
	}
}

// TestCustomBackgroundKeepsAlpha 验证带透明的图片仍存成 PNG。
func TestCustomBackgroundKeepsAlpha(t *testing.T) {
	app := newTestApp(t)
	src := filepath.Join(absBase, "soft.png")
	writePNG(t, src, 64, 32, color.NRGBA{R: 255, G: 0, B: 0, A: 128})
	stored, err := app.customBackground(src)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(stored) != ".png" {
		t.Fatalf("a picture with transparency must stay PNG, got %q", stored)
	}
}

// TestPruneKeepsCustomIconsWhenPrefsUnreadable 验证偏好文件临时写坏时不会把
// custom-icons 里的自定义程序图标或背景当成孤儿删掉。
func TestPruneKeepsCustomIconsWhenPrefsUnreadable(t *testing.T) {
	app := newTestApp(t)
	dir := filepath.Join(dataBase(), customIconsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, "keep.png")
	writePNG(t, keep, 4, 4, color.NRGBA{A: 255})
	if err := os.WriteFile(filepath.Join(dataBase(), uiStateFile), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	app.pruneIcons()
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("an unreadable preferences file must not delete custom icons: %v", err)
	}
}

// TestCustomAppIconSquaresAndGears 验证自定义程序图标补成正方形，设置窗口的
// 齿轮角标确实画了上去。
func TestCustomAppIconSquaresAndGears(t *testing.T) {
	app := newTestApp(t)
	v := newView(app)
	src := filepath.Join(absBase, "logo.png")
	writePNG(t, src, 16, 8, color.NRGBA{R: 20, G: 80, B: 220, A: 255})
	stored, err := app.customIcon(src, 0)
	if err != nil {
		t.Fatal(err)
	}
	prefs := loadPrefs()
	prefs.AppIcon = stored
	savePrefs(prefs)
	v.prefs = prefs

	custom := v.customAppIcon()
	if custom == nil {
		t.Fatal("the chosen icon must load")
	}
	if b := custom.Bounds(); b.Dx() != b.Dy() || b.Dx() != 16 {
		t.Fatalf("the icon must be square and keep its size, got %v", b)
	}
	plain := v.appIconForDark(false, false)
	geared := v.appIconForDark(true, false)
	if bytes.Equal(plain.Pix, geared.Pix) {
		t.Fatal("the settings window icon must carry the gear")
	}
}

// TestViewDrawsCustomBackground 验证主界面内容区按透明度画出背景图。
func TestViewDrawsCustomBackground(t *testing.T) {
	app := newTestApp(t)
	seedItems(t, app)
	src := filepath.Join(absBase, "wall.png")
	writePNG(t, src, 40, 20, color.NRGBA{R: 255, A: 255})
	stored, err := app.customBackground(src)
	if err != nil {
		t.Fatal(err)
	}
	prefs := loadPrefs()
	prefs.Background = stored
	prefs.BackgroundOpacity = 0.4
	savePrefs(prefs)

	v := newView(app)
	tt := ui.NewTester(v.view, 720, 460)
	tt.SetFocused(true)
	img := tt.Image()
	r, g, b, _ := img.At(400, 200).RGBA()
	if r <= g || r <= b {
		t.Fatalf("the content must show the red background over white, got r=%d g=%d b=%d", r, g, b)
	}
	// The top bar shows the same picture's top, not its own crop.
	hr, hg, hb, _ := img.At(400, 25).RGBA()
	if hr <= hg || hr <= hb {
		t.Fatalf("the top bar must show the background too, got r=%d g=%d b=%d", hr, hg, hb)
	}
}

// TestPrefsAppearanceDefaults 验证背景透明度的默认与清理，以及非应用目录的
// 图标路径不会被采用。
func TestPrefsAppearanceDefaults(t *testing.T) {
	newTestApp(t)
	path := filepath.Join(dataBase(), uiStateFile)
	if err := os.WriteFile(path, []byte(`{"background_opacity":0.7}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPrefs().BackgroundOpacity; got != 0.7 {
		t.Fatalf("the saved opacity must load, got %v", got)
	}
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPrefs().BackgroundOpacity; got != 0.4 {
		t.Fatalf("without a value the opacity defaults to 0.4, got %v", got)
	}
	if err := os.WriteFile(path, []byte(`{"app_icon":"C:/elsewhere.png","background":"../wall.png"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	prefs := loadPrefs()
	if prefs.AppIcon != "" || prefs.Background != "" {
		t.Fatalf("icons outside the data folder must be dropped, got %+v", prefs)
	}
}

// TestSettingsShowsAppearanceRows 验证外观底部有程序图标、背景和透明度三行。
func TestSettingsShowsAppearanceRows(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	settings := newTestSettings(t, v)
	for _, label := range []string{"Program icon", "Background", "Background opacity"} {
		if !settings.HasText(label) {
			t.Fatalf("the appearance group must show %q, texts %q", label, settings.Texts())
		}
	}
	if !settings.HasText("Choose icon…") || !settings.HasText("Choose image…") {
		t.Fatalf("the rows must offer their pickers, texts %q", settings.Texts())
	}
}
