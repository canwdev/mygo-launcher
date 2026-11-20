package main

import (
	"errors"
	"slices"
	"testing"
)

// TestViewSettingsTogglesOpenAtLogin 验证行为页提供开机自启动，开关写入系统
// 的 Run 项，并把真实状态留在设置里。
func TestViewSettingsTogglesOpenAtLogin(t *testing.T) {
	_, v, _ := newTestView(t, 720, 460)

	oldSet, oldOpen := setOpenAtLogin, openAtLogin
	t.Cleanup(func() { setOpenAtLogin, openAtLogin = oldSet, oldOpen })

	enabled := false
	var calls []bool
	setOpenAtLogin = func(on bool) error {
		calls = append(calls, on)
		enabled = on
		return nil
	}
	openAtLogin = func() bool { return enabled }

	settings := newTestSettings(t, v)
	openSettingsPage(t, settings, "Behavior")
	if !settings.HasText("Start at login") || !settings.HasText("Open in the background when you sign in to Windows") {
		t.Fatalf("the behavior page must offer starting at login, texts %q", settings.Texts())
	}
	if v.openAtLogin {
		t.Fatal("a fresh launcher must not start at login")
	}

	if err := settings.Click("Toggle start at login"); err != nil {
		t.Fatalf("start at login: %v", err)
	}
	settings.Frame()
	if !enabled || !v.openAtLogin {
		t.Fatalf("the toggle must register the Run entry, enabled %v, view %v", enabled, v.openAtLogin)
	}
	if len(calls) != 1 || !calls[0] {
		t.Fatalf("the Run entry must be registered once, calls %v", calls)
	}

	if err := settings.Click("Toggle start at login"); err != nil {
		t.Fatalf("start at login again: %v", err)
	}
	settings.Frame()
	if enabled || v.openAtLogin {
		t.Fatalf("the toggle must remove the Run entry, enabled %v, view %v", enabled, v.openAtLogin)
	}
	if len(calls) != 2 || calls[1] {
		t.Fatalf("the Run entry must be removed once, calls %v", calls)
	}
}

// TestViewSettingsKeepsOpenAtLoginOnFailure 验证系统拒绝时开关保持原状并说明。
func TestViewSettingsKeepsOpenAtLoginOnFailure(t *testing.T) {
	app, v, _ := newTestView(t, 720, 460)

	oldSet, oldOpen := setOpenAtLogin, openAtLogin
	t.Cleanup(func() { setOpenAtLogin, openAtLogin = oldSet, oldOpen })

	openAtLogin = func() bool { return true }
	setOpenAtLogin = func(bool) error { return errors.New("access denied") }
	v.refreshOpenAtLogin()

	settings := newTestSettings(t, v)
	openSettingsPage(t, settings, "Behavior")
	if err := settings.Click("Toggle start at login"); err != nil {
		t.Fatalf("start at login: %v", err)
	}
	settings.Frame()

	if !v.openAtLogin {
		t.Fatal("a refused change must leave the switch on")
	}
	toasts, _ := app.takeMessages()
	if !slices.Contains(toasts, "Cannot change the start-at-login setting") {
		t.Fatalf("the toast must say why, toasts %q", toasts)
	}
}
