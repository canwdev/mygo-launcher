package main

import (
	"errors"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestShortcutAccelerator 验证按键和修饰键写法，以及必须有 Ctrl/Alt/Super。
func TestShortcutAccelerator(t *testing.T) {
	cases := []struct {
		mods ui.Modifiers
		key  ui.Key
		want string
		ok   bool
	}{
		{ui.Ctrl | ui.Alt, ui.KeySpace, "Ctrl+Alt+Space", true},
		{ui.Super | ui.Shift, ui.KeyF5, "Super+Shift+F5", true},
		{ui.Ctrl, ui.KeyMinus, "Ctrl+-", true},
		{0, ui.KeyF1, "F1", true},
		{0, ui.KeyF12, "F12", true},
		{ui.Shift, ui.KeyF2, "Shift+F2", true},
		{ui.Shift, ui.KeyA, "", false},
		{0, ui.KeyA, "", false},
		{0, ui.KeySpace, "", false},
		{ui.Ctrl, ui.KeyContextMenu, "", false},
	}
	for _, c := range cases {
		got, ok := shortcutAccelerator(c.mods, c.key)
		if got != c.want || ok != c.ok {
			t.Errorf("shortcutAccelerator(%v, %v) = %q, %v; want %q, %v", c.mods, c.key, got, ok, c.want, c.ok)
		}
	}
}

// TestViewSettingsRecordsShortcut 验证设置里录制快捷键、写回偏好并注册。
func TestViewSettingsRecordsShortcut(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	oldRegister, oldUnregister := registerShortcut, unregisterShortcut
	var registered, unregistered string
	registerShortcut = func(acc string, fn func()) error {
		registered = acc
		return nil
	}
	unregisterShortcut = func(acc string) {
		unregistered, registered = acc, ""
	}
	t.Cleanup(func() {
		registerShortcut, unregisterShortcut = oldRegister, oldUnregister
	})

	settings := newTestSettings(t, v)
	if err := settings.Click("Not set"); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	settings.Key(ui.Ctrl|ui.Alt, ui.KeySpace)
	settings.Frame()

	if v.prefs.Shortcut != "Ctrl+Alt+Space" {
		t.Fatalf("the shortcut must be kept, got %q", v.prefs.Shortcut)
	}
	if registered != "Ctrl+Alt+Space" || v.registeredShortcut != "Ctrl+Alt+Space" {
		t.Fatalf("the shortcut must be registered, registered %q, view %q", registered, v.registeredShortcut)
	}
	if got := loadPrefs().Shortcut; got != "Ctrl+Alt+Space" {
		t.Fatalf("the shortcut must be saved, got %q", got)
	}
	if !settings.HasText("Ctrl+Alt+Space") {
		t.Fatalf("the row must show the shortcut, texts %q", settings.Texts())
	}

	if err := settings.Click("Clear"); err != nil {
		t.Fatalf("clear the shortcut: %v", err)
	}
	if v.prefs.Shortcut != "" || loadPrefs().Shortcut != "" || v.registeredShortcut != "" {
		t.Fatalf("clearing must remove the shortcut, prefs %q, view %q", v.prefs.Shortcut, v.registeredShortcut)
	}
	if unregistered != "Ctrl+Alt+Space" {
		t.Fatalf("clearing must unbind the shortcut, unregistered %q", unregistered)
	}
}

// TestViewSettingsRecordsFunctionKey 验证 F1 这样的功能键可以单独录制。
func TestViewSettingsRecordsFunctionKey(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)
	oldRegister, oldUnregister := registerShortcut, unregisterShortcut
	var registered string
	registerShortcut = func(acc string, fn func()) error {
		registered = acc
		return nil
	}
	unregisterShortcut = func(string) {}
	t.Cleanup(func() {
		registerShortcut, unregisterShortcut = oldRegister, oldUnregister
	})

	settings := newTestSettings(t, v)
	if err := settings.Click("Not set"); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	settings.Key(0, ui.KeyF1)
	settings.Frame()

	if v.prefs.Shortcut != "F1" || registered != "F1" {
		t.Fatalf("F1 must be a shortcut of its own, prefs %q, registered %q", v.prefs.Shortcut, registered)
	}
	if got := loadPrefs().Shortcut; got != "F1" {
		t.Fatalf("the shortcut must be saved, got %q", got)
	}
}

// TestViewSettingsRejectsTakenShortcut 验证注册失败时保留旧值并在设置里说明。
func TestViewSettingsRejectsTakenShortcut(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)
	oldRegister, oldUnregister := registerShortcut, unregisterShortcut
	registerShortcut = func(acc string, fn func()) error {
		if acc == "Ctrl+Shift+X" {
			return errors.New("taken")
		}
		return nil
	}
	unregisterShortcut = func(string) {}
	t.Cleanup(func() {
		registerShortcut, unregisterShortcut = oldRegister, oldUnregister
	})

	settings := newTestSettings(t, v)
	if err := settings.Click("Not set"); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	settings.Key(ui.Ctrl|ui.Shift, ui.KeyX)
	settings.Frame()

	if v.prefs.Shortcut != "" || loadPrefs().Shortcut != "" {
		t.Fatalf("a refused shortcut must not be kept, got %q", v.prefs.Shortcut)
	}
	toasts, _ := app.takeMessages()
	if !slices.Contains(toasts, "Cannot register Ctrl+Shift+X") {
		t.Fatalf("the toast must say why, toasts %q", toasts)
	}
}
