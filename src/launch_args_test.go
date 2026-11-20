package main

import "testing"

// TestParseLaunchArgs 验证启动参数：静默与指定位置；未知参数忽略，--silent
// 压过位置。
func TestParseLaunchArgs(t *testing.T) {
	tests := []struct {
		args []string
		want launchOptions
	}{
		{nil, launchOptions{}},
		{[]string{"--silent"}, launchOptions{silent: true}},
		{[]string{"--position", "center"}, launchOptions{place: placeCenter}},
		{[]string{"--position=last"}, launchOptions{place: placeLast}},
		{[]string{"--POSITION=CURSOR"}, launchOptions{place: placeCursor}},
		{[]string{"--position", "center", "--silent"}, launchOptions{silent: true}},
		{[]string{"--position"}, launchOptions{}},
		{[]string{"--position", "--silent"}, launchOptions{silent: true}},
		{[]string{"--position", "nowhere"}, launchOptions{}},
		{[]string{"--position="}, launchOptions{}},
		{[]string{"--other", "--silent"}, launchOptions{silent: true}},
		{[]string{"--help"}, launchOptions{}},
		{[]string{"--version"}, launchOptions{}},
	}
	for _, test := range tests {
		if got := parseLaunchArgs(test.args); got != test.want {
			t.Fatalf("parseLaunchArgs(%q) = %+v, want %+v", test.args, got, test.want)
		}
	}
}

// TestExistingInstanceOpts 验证第二次启动：未指定位置时移到指针处，--position
// 指定的位置照用，--silent 则什么都不做。
func TestExistingInstanceOpts(t *testing.T) {
	tests := []struct {
		args []string
		want launchOptions
	}{
		{nil, launchOptions{place: placeCursor}},
		{[]string{"--position", "center"}, launchOptions{place: placeCenter}},
		{[]string{"--position=last"}, launchOptions{place: placeLast}},
		{[]string{"--other"}, launchOptions{place: placeCursor}},
		{[]string{"--silent"}, launchOptions{}},
		{[]string{"--silent", "--position", "center"}, launchOptions{}},
	}
	for _, test := range tests {
		if got := existingInstanceOpts(parseLaunchArgs(test.args)); got != test.want {
			t.Fatalf("existingInstanceOpts(%q) = %+v, want %+v", test.args, got, test.want)
		}
	}
}
