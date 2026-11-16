package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// TestStartupPlace 验证窗口开在哪里：偏好决定上次位置、指针处与屏幕中央，
// --at-cursor 总是压过偏好；未知的值退回指针处。
func TestStartupPlace(t *testing.T) {
	tests := []struct {
		pref string
		opts launchOptions
		want windowPlace
	}{
		{"last", launchOptions{}, placeLast},
		{"cursor", launchOptions{}, placeCursor},
		{"center", launchOptions{}, placeCenter},
		{"last", launchOptions{atCursor: true}, placeCursor},
		{"center", launchOptions{atCursor: true}, placeCursor},
		{"", launchOptions{}, placeCursor},
		{"elsewhere", launchOptions{}, placeCursor},
	}
	for _, test := range tests {
		_, v, _ := newTestView(t, 720, 460)
		v.prefs.WindowPosition = test.pref
		if got := v.startupPlace(test.opts); got != test.want {
			t.Fatalf("startupPlace(%q, %+v) = %q, want %q", test.pref, test.opts, got, test.want)
		}
	}
}

// TestPrefsWindowPosition 验证窗口位置的默认值与未知值：默认指针处，读到的
// 未知值改回默认。
func TestPrefsWindowPosition(t *testing.T) {
	newTestApp(t)
	path := filepath.Join(dataBase(), uiStateFile)
	if got := loadPrefs().WindowPosition; got != "cursor" {
		t.Fatalf("without a value the position defaults to the cursor, got %q", got)
	}
	if err := os.WriteFile(path, []byte(`{"window_position":"center"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPrefs().WindowPosition; got != "center" {
		t.Fatalf("the saved position must load, got %q", got)
	}
	if err := os.WriteFile(path, []byte(`{"window_position":"nowhere"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPrefs().WindowPosition; got != "cursor" {
		t.Fatalf("an unknown position must fall back to the cursor, got %q", got)
	}
}

// TestPrefsWindowOpacity 验证窗口透明度的默认值：没有记录时完全不透明，
// 超出范围的记录收进可用的区间。
func TestPrefsWindowOpacity(t *testing.T) {
	newTestApp(t)
	path := filepath.Join(dataBase(), uiStateFile)
	if got := loadPrefs().WindowOpacity; got != 1 {
		t.Fatalf("without a value the window is opaque, got %v", got)
	}
	if err := os.WriteFile(path, []byte(`{"window_opacity":0.6}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPrefs().WindowOpacity; got != 0.6 {
		t.Fatalf("the saved opacity must load, got %v", got)
	}
	if err := os.WriteFile(path, []byte(`{"window_opacity":0.01}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPrefs().WindowOpacity; got != minWindowOpacity {
		t.Fatalf("an unreadable opacity must keep its floor, got %v", got)
	}
}

// TestPrefsWindowMaterial 验证背景材质的默认值与未知值：默认没有材质，
// 读到的未知材质改回没有。
func TestPrefsWindowMaterial(t *testing.T) {
	newTestApp(t)
	path := filepath.Join(dataBase(), uiStateFile)
	if got := loadPrefs().WindowMaterial; got != "" {
		t.Fatalf("without a value there is no material, got %q", got)
	}
	if err := os.WriteFile(path, []byte(`{"window_material":"mica"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPrefs().WindowMaterial; got != "mica" {
		t.Fatalf("the saved material must load, got %q", got)
	}
	if err := os.WriteFile(path, []byte(`{"window_material":"wood"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadPrefs().WindowMaterial; got != "" {
		t.Fatalf("an unknown material must fall back to none, got %q", got)
	}
}

// TestWindowMaterial 验证材质 id 到 MyGo 材质的映射。
func TestWindowMaterial(t *testing.T) {
	for id, want := range map[string]string{"": "", "mica": "mica", "acrylic": "acrylic", "wood": ""} {
		if got := string(windowMaterial(id)); got != want {
			t.Fatalf("windowMaterial(%q) = %q, want %q", id, got, want)
		}
	}
}

// TestWindowTransparency 验证透明度与不透明度的换算：越高越透明，两端都不会
// 越过可用的范围。
func TestWindowTransparency(t *testing.T) {
	tests := []struct {
		opacity float64
		want    float64
	}{
		{1, 0},
		{0.8, 0.2},
		{0.6, 0.4},
		{minWindowOpacity, 1 - minWindowOpacity},
		{0.1, 1 - minWindowOpacity}, // below the floor the window keeps it
	}
	for _, test := range tests {
		if got := windowTransparency(test.opacity); math.Abs(got-test.want) > 1e-9 {
			t.Fatalf("windowTransparency(%v) = %v, want %v", test.opacity, got, test.want)
		}
		want := max(minWindowOpacity, test.opacity)
		if got := windowOpacityFrom(test.want); math.Abs(got-want) > 1e-9 {
			t.Fatalf("windowOpacityFrom(%v) = %v, want %v", test.want, got, want)
		}
	}
}

// TestSurfaceFollowsMaterial 验证有材质时面板不再画底色（半透明的那层由窗口
// 自己的底色负责，画两遍就叠厚了），没有材质时原样画。
func TestSurfaceFollowsMaterial(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	white := ui.RGBA(255, 255, 255, 1)
	if got := v.surface(white); got != white {
		t.Fatalf("without a material the pane paints the color, got %+v", got)
	}
	v.material = true
	if got := v.surface(white); got.A != 0 {
		t.Fatalf("with a material the pane leaves the tint to the window, got %+v", got)
	}
	// The window is what paints the tint: the palette's color, part see-through,
	// so texts keep a known tone and the material still shows.
	fill := v.windowFill(white)
	if fill.A == 0 || fill.A == 255 {
		t.Fatalf("the window must tint itself over the material, got %+v", fill)
	}
	if fill.R != white.R || fill.G != white.G || fill.B != white.B {
		t.Fatalf("the tint keeps the palette's color, got %+v", fill)
	}
	v.material = false
	if got := v.windowFill(white); got != white {
		t.Fatalf("without a material the window paints the color, got %+v", got)
	}
}

// TestPlaceWindowInCenter 验证居中按工作区算，窗口比工作区大时从它的边缘开始。
func TestPlaceWindowInCenter(t *testing.T) {
	work := mygo.Rectangle{X: 1920, Y: 0, Width: 1920, Height: 1080}
	if x, y := placeWindowInCenter(720, 520, work); x != 1920+(1920-720)/2 || y != (1080-520)/2 {
		t.Fatalf("placeWindowInCenter = %d,%d", x, y)
	}
	if x, y := placeWindowInCenter(2400, 1400, work); x != work.X || y != work.Y {
		t.Fatalf("a window larger than the work area starts at its edge, got %d,%d", x, y)
	}
}

// TestSettingsShowsWindowPosition 验证行为页的窗口位置三选一。
func TestSettingsShowsWindowPosition(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	settings := newTestSettings(t, v)
	openSettingsPage(t, settings, "Behavior")
	for _, label := range []string{"Window position", "Last position", "At cursor", "Screen center"} {
		if !settings.HasText(label) {
			t.Fatalf("the behavior page must show %q, texts %q", label, settings.Texts())
		}
	}
	if err := settings.Click("Screen center"); err != nil {
		t.Fatalf("choose the screen center: %v", err)
	}
	settings.Frame()
	if v.prefs.WindowPosition != "center" {
		t.Fatalf("the chosen position must be kept, got %q", v.prefs.WindowPosition)
	}
}
