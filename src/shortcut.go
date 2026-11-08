package main

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"go-launcher/locales"
)

// The MyGo calls the shortcut lives on, replaced in tests: registering one
// needs the app's message loop, which the headless tester does not run.
var (
	registerShortcut = func(acc string, fn func()) error {
		return mygo.GlobalShortcut.Register(acc, fn)
	}
	unregisterShortcut = func(acc string) { mygo.GlobalShortcut.Unregister(acc) }
)

// registerSavedShortcut binds the shortcut kept in ui-state.json, once the app
// runs; one the system refuses stays in the settings with its reason.
func (v *view) registerSavedShortcut() {
	acc := strings.TrimSpace(v.prefs.Shortcut)
	if acc == "" {
		return
	}
	if err := v.applyShortcut(acc); err != nil {
		v.app.notify(locales.ShortcutUnavailable(shortcutLabel(acc)))
	}
}

// applyShortcut binds acc, replacing the shortcut bound so far, and reports
// whether the system took it. A refused shortcut leaves the previous one
// bound, so the launcher never loses its way back for a typo.
func (v *view) applyShortcut(acc string) error {
	acc = strings.TrimSpace(acc)
	old := v.registeredShortcut
	if old == acc {
		return nil
	}
	if old != "" {
		unregisterShortcut(old)
		v.registeredShortcut = ""
	}
	if acc == "" {
		return nil
	}
	if err := registerShortcut(acc, v.toggleMainWindow); err != nil {
		if old != "" {
			if registerShortcut(old, v.toggleMainWindow) == nil {
				v.registeredShortcut = old
			}
		}
		return err
	}
	v.registeredShortcut = acc
	return nil
}

// changeShortcut applies a shortcut chosen in the settings and keeps it; a
// refused one stays out of the preferences and says why in the row.
func (v *view) changeShortcut(acc string) {
	if err := v.applyShortcut(acc); err != nil {
		v.app.notify(v.msgs().ShortcutUnavailable(shortcutLabel(acc)))
		return
	}
	v.prefs.Shortcut = acc
	v.savePrefs()
	v.syncTray()
}

// toggleMainWindow shows the launcher, or puts it away when it has the focus.
// With the background setting on it goes to the tray; otherwise it minimizes,
// so a taskbar button still leads back to it.
func (v *view) toggleMainWindow() {
	w := v.app.win
	if w == nil {
		return
	}
	if w.IsVisible() && w.IsFocused() {
		if v.app.keepInBackground() {
			v.app.hideWindow()
		} else {
			v.app.minimize()
		}
		return
	}
	// A window called for from anywhere comes up where the pointer is.
	v.showMainAtCursor()
}

// startShortcutRecording listens for the next chord in the settings.
func (v *view) startShortcutRecording() {
	v.shortcutRecording = true
	v.shortcutDraft = ""
	v.refresh()
}

func (v *view) cancelShortcutRecording() {
	v.shortcutRecording = false
	v.shortcutDraft = ""
	v.refresh()
}

// captureShortcut takes the chord pressed in the recorder, and reports it as
// taken: the text input must not type it. Escape cancels, and Backspace or
// Delete with no modifier clears the shortcut.
func (v *view) captureShortcut(k ui.TextKey) bool {
	switch {
	case k.Key == ui.KeyEscape && k.Mods == 0:
		v.cancelShortcutRecording()
		return true
	case (k.Key == ui.KeyBackspace || k.Key == ui.KeyDelete) && k.Mods == 0:
		v.shortcutRecording = false
		v.shortcutDraft = ""
		v.changeShortcut("")
		return true
	}
	acc, ok := shortcutAccelerator(k.Mods, k.Key)
	if !ok {
		if _, supported := shortcutKeyNames[k.Key]; !supported {
			v.app.notify(v.msgs().ShortcutKeyUnsupported())
		} else {
			v.app.notify(v.msgs().ShortcutNeedsModifier())
		}
		return true
	}
	v.shortcutRecording = false
	v.shortcutDraft = ""
	v.changeShortcut(acc)
	return true
}

// shortcutAccelerator writes a key with modifiers as MyGo's global shortcuts
// take it, e.g. "Ctrl+Alt+Space". Other keys need Ctrl, Alt or Super, since a
// bare one would take that key from every program; the function keys are meant
// for this and may stand alone.
func shortcutAccelerator(mods ui.Modifiers, key ui.Key) (string, bool) {
	name, ok := shortcutKeyNames[key]
	if !ok {
		return "", false
	}
	if mods&(ui.Ctrl|ui.Alt|ui.Super) == 0 && !shortcutFunctionKey(key) {
		return "", false
	}
	var parts []string
	if mods&ui.Super != 0 {
		parts = append(parts, "Super")
	}
	if mods&ui.Ctrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&ui.Alt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&ui.Shift != 0 {
		parts = append(parts, "Shift")
	}
	parts = append(parts, name)
	return strings.Join(parts, "+"), true
}

// shortcutFunctionKey reports whether a key may be the whole shortcut.
func shortcutFunctionKey(key ui.Key) bool {
	switch key {
	case ui.KeyF1, ui.KeyF2, ui.KeyF3, ui.KeyF4, ui.KeyF5, ui.KeyF6,
		ui.KeyF7, ui.KeyF8, ui.KeyF9, ui.KeyF10, ui.KeyF11, ui.KeyF12:
		return true
	}
	return false
}

// shortcutControl is the settings row's control: the shortcut as a button that
// starts recording, and a way to clear it; while recording, a read-only input
// takes the next chord and shows nothing of what it types.
func (v *view) shortcutControl(c *ui.Context) {
	m := v.msgs()
	if v.shortcutRecording {
		ui.TextInput(c, &v.shortcutDraft).Bind(&v.shortcutInput).AutoFocus().
			ReadOnly(true).Placeholder(m.ShortcutRecording()).Width(170).
			OnTextKey(v.captureShortcut)
		return
	}
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		label := m.ShortcutUnset()
		if v.prefs.Shortcut != "" {
			label = shortcutLabel(v.prefs.Shortcut)
		}
		if ui.Button(c, label).Clicked() {
			v.startShortcutRecording()
		}
		if v.prefs.Shortcut != "" {
			if ui.Button(c, m.Clear()).Clicked() {
				v.changeShortcut("")
			}
		}
	})
}

// shortcutLabel spells a shortcut as the settings and the user's platform
// show it: the Windows key is Win, not Super.
func shortcutLabel(acc string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(acc, "Super", "Win")
	}
	return acc
}

// shortcutKeyNames are the keys mygo's accelerators name. MyGo's parser takes
// single characters as themselves, and these names for the rest.
var shortcutKeyNames = map[ui.Key]string{
	ui.KeySpace:        "Space",
	ui.KeyEnter:        "Enter",
	ui.KeyTab:          "Tab",
	ui.KeyEscape:       "Escape",
	ui.KeyBackspace:    "Backspace",
	ui.KeyDelete:       "Delete",
	ui.KeyInsert:       "Insert",
	ui.KeyHome:         "Home",
	ui.KeyEnd:          "End",
	ui.KeyPageUp:       "PageUp",
	ui.KeyPageDown:     "PageDown",
	ui.KeyLeft:         "Left",
	ui.KeyRight:        "Right",
	ui.KeyUp:           "Up",
	ui.KeyDown:         "Down",
	ui.KeyF1:           "F1",
	ui.KeyF2:           "F2",
	ui.KeyF3:           "F3",
	ui.KeyF4:           "F4",
	ui.KeyF5:           "F5",
	ui.KeyF6:           "F6",
	ui.KeyF7:           "F7",
	ui.KeyF8:           "F8",
	ui.KeyF9:           "F9",
	ui.KeyF10:          "F10",
	ui.KeyF11:          "F11",
	ui.KeyF12:          "F12",
	ui.KeyMinus:        "-",
	ui.KeyEqual:        "=",
	ui.KeyComma:        ",",
	ui.KeyPeriod:       ".",
	ui.KeySlash:        "/",
	ui.KeySemicolon:    ";",
	ui.KeyQuote:        "'",
	ui.KeyBracketLeft:  "[",
	ui.KeyBracketRight: "]",
	ui.KeyBackslash:    `\`,
	ui.KeyBackquote:    "`",
	ui.Key0:            "0",
	ui.Key1:            "1",
	ui.Key2:            "2",
	ui.Key3:            "3",
	ui.Key4:            "4",
	ui.Key5:            "5",
	ui.Key6:            "6",
	ui.Key7:            "7",
	ui.Key8:            "8",
	ui.Key9:            "9",
	ui.KeyA:            "A",
	ui.KeyB:            "B",
	ui.KeyC:            "C",
	ui.KeyD:            "D",
	ui.KeyE:            "E",
	ui.KeyF:            "F",
	ui.KeyG:            "G",
	ui.KeyH:            "H",
	ui.KeyI:            "I",
	ui.KeyJ:            "J",
	ui.KeyK:            "K",
	ui.KeyL:            "L",
	ui.KeyM:            "M",
	ui.KeyN:            "N",
	ui.KeyO:            "O",
	ui.KeyP:            "P",
	ui.KeyQ:            "Q",
	ui.KeyR:            "R",
	ui.KeyS:            "S",
	ui.KeyT:            "T",
	ui.KeyU:            "U",
	ui.KeyV:            "V",
	ui.KeyW:            "W",
	ui.KeyX:            "X",
	ui.KeyY:            "Y",
	ui.KeyZ:            "Z",
}
