package main

import (
	"testing"

	"github.com/egoist/mygo"
)

// TestParseLaunchArgs 验证启动参数：两个开关都认得，未知参数忽略，同时出现时
// 静默优先。
func TestParseLaunchArgs(t *testing.T) {
	tests := []struct {
		args []string
		want launchOptions
	}{
		{nil, launchOptions{}},
		{[]string{"--silent"}, launchOptions{silent: true}},
		{[]string{"--at-cursor"}, launchOptions{atCursor: true}},
		{[]string{"--other", "--silent"}, launchOptions{silent: true}},
		{[]string{"--silent", "--at-cursor"}, launchOptions{silent: true}},
		{[]string{"--AT-CURSOR"}, launchOptions{atCursor: true}},
	}
	for _, test := range tests {
		if got := parseLaunchArgs(test.args); got != test.want {
			t.Fatalf("parseLaunchArgs(%q) = %+v, want %+v", test.args, got, test.want)
		}
	}
}

// TestExistingInstanceOpts 验证第二次启动的默认行为：没有参数时和在光标处显示
// 一样，只有静默启动不打扢已在运行的窗口。
func TestExistingInstanceOpts(t *testing.T) {
	tests := []struct {
		args []string
		want launchOptions
	}{
		{nil, launchOptions{atCursor: true}},
		{[]string{"--at-cursor"}, launchOptions{atCursor: true}},
		{[]string{"--other"}, launchOptions{atCursor: true}},
		{[]string{"--silent"}, launchOptions{}},
		{[]string{"--silent", "--at-cursor"}, launchOptions{}},
	}
	for _, test := range tests {
		if got := existingInstanceOpts(parseLaunchArgs(test.args)); got != test.want {
			t.Fatalf("existingInstanceOpts(%q) = %+v, want %+v", test.args, got, test.want)
		}
	}
}

// TestPlaceWindowAtCursor 验证窗口居中于光标，并在工作区边缘收住。
func TestPlaceWindowAtCursor(t *testing.T) {
	work := mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1040}
	tests := []struct {
		name   string
		cursor mygo.Point
		wantX  int
		wantY  int
	}{
		{"middle", mygo.Point{X: 960, Y: 520}, 600, 260},
		{"top left", mygo.Point{X: 100, Y: 100}, 0, 0},
		{"bottom right", mygo.Point{X: 1900, Y: 1030}, 1200, 520},
	}
	for _, test := range tests {
		x, y := placeWindowAtCursor(720, 520, test.cursor, work)
		if x != test.wantX || y != test.wantY {
			t.Fatalf("%s: placeWindowAtCursor = (%d, %d), want (%d, %d)", test.name, x, y, test.wantX, test.wantY)
		}
	}

	x, y := placeWindowAtCursor(2000, 2000, mygo.Point{X: 900, Y: 500}, work)
	if x != work.X || y != work.Y {
		t.Fatalf("a window larger than the work area starts at its edge, got (%d, %d)", x, y)
	}

	x, y = placeWindowAtCursor(720, 520, mygo.Point{X: 500, Y: 400}, mygo.Rectangle{})
	if x != 140 || y != 140 {
		t.Fatalf("without a work area the window centers on the pointer, got (%d, %d)", x, y)
	}
}
