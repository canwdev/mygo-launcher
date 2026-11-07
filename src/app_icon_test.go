package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestAppIconTintsAccent 验证应用图标用强调色重绘，且半透明箭头仍是淡淡的一层。
func TestAppIconTintsAccent(t *testing.T) {
	red := ui.RGB(255, 0, 0)
	icon := appIcon(red, false, false)
	if icon == nil {
		t.Fatal("the icon assets must compose")
	}

	solid, soft := 0, 0
	for i := 0; i < len(icon.Pix); i += 4 {
		if icon.Pix[i+3] == 0 {
			continue
		}
		r, g, b := icon.Pix[i], icon.Pix[i+1], icon.Pix[i+2]
		switch {
		case r > 240 && g < 40 && b < 40:
			solid++
		case r > 245 && g > 190 && g < 235 && b > 190 && b < 235:
			soft++
		}
	}
	if solid == 0 {
		t.Fatal("the accent shapes must take the accent color")
	}
	if soft == 0 {
		t.Fatal("the translucent arrow must stay a tint of the background")
	}

	data := encodeIcon(icon, 64)
	if data == nil {
		t.Fatal("the icon must encode")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("the icon must be 64x64, got %v", b)
	}
}

// TestAboutIconFollowsAccent 验证关于行用当前强调色的图标，并在换色后重画。
func TestAboutIconFollowsAccent(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	first := v.aboutIconImage()
	if first == nil {
		t.Fatal("the About row must show the app icon")
	}
	v.prefs.Accent = "bang-2"
	if second := v.aboutIconImage(); second == first {
		t.Fatal("the About icon must follow the accent")
	}
}

// TestSettingsIconCarriesGear 验证设置窗口的图标右下角多出齿轮。
func TestSettingsIconCarriesGear(t *testing.T) {
	red := ui.RGB(255, 0, 0)
	dark := func(icon image.Image, quarter bool) (int, string) {
		b := icon.Bounds()
		n := 0
		sample := ""
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if quarter && (x < b.Min.X+b.Dx()*3/4 || y < b.Min.Y+b.Dy()*3/4) {
					continue
				}
				r, g, blue, a := icon.At(x, y).RGBA()
				if a>>8 > 128 && r>>8 < 80 && g>>8 < 80 && blue>>8 < 80 {
					n++
					if sample == "" {
						sample = fmt.Sprintf("%d,%d=%d,%d,%d,%d", x, y, r>>8, g>>8, blue>>8, a>>8)
					}
				}
			}
		}
		return n, sample
	}
	plain, geared := appIcon(red, false, false), appIcon(red, false, true)
	if plain == nil || geared == nil {
		t.Fatal("the icon assets must compose")
	}
	if n, sample := dark(plain, true); n != 0 {
		t.Fatalf("the plain icon must carry no gear, got %d at %s", n, sample)
	}
	if n, _ := dark(geared, true); n == 0 {
		t.Fatal("the settings icon must carry a dark gear in its corner")
	}
}

// TestAppIconFollowsDarkAppearance 验证暗色外观下图标改用深色底，齿轮改用浅色。
func TestAppIconFollowsDarkAppearance(t *testing.T) {
	red := ui.RGB(255, 0, 0)
	light, dark := appIcon(red, false, true), appIcon(red, true, true)
	if light == nil || dark == nil {
		t.Fatal("the icon assets must compose")
	}
	lr, lg, lb, _ := light.At(40, 40).RGBA()
	if lr>>8 < 250 || lg>>8 < 250 || lb>>8 < 250 {
		t.Fatalf("the light icon must keep a white background, got %d,%d,%d", lr>>8, lg>>8, lb>>8)
	}
	dr, dg, db, da := dark.At(40, 40).RGBA()
	if da>>8 < 250 || dr>>8 > 60 || dg>>8 > 60 || db>>8 > 60 {
		t.Fatalf("the dark icon must have a dark background, got %d,%d,%d a=%d", dr>>8, dg>>8, db>>8, da>>8)
	}

	// The gear must turn light where the light icon draws it dark.
	lightGear, darkGear := 0, 0
	b := dark.Bounds()
	for y := b.Min.Y + b.Dy()*3/4; y < b.Max.Y; y++ {
		for x := b.Min.X + b.Dx()*3/4; x < b.Max.X; x++ {
			if r, g, bl, a := light.At(x, y).RGBA(); a>>8 > 128 && r>>8 < 80 && g>>8 < 80 && bl>>8 < 80 {
				lightGear++
			}
			if r, g, bl, a := dark.At(x, y).RGBA(); a>>8 > 128 && r>>8 > 200 && g>>8 > 200 && bl>>8 > 200 {
				darkGear++
			}
		}
	}
	if lightGear == 0 || darkGear == 0 {
		t.Fatalf("the gear must follow the appearance, got %d light and %d dark pixels", lightGear, darkGear)
	}
}
