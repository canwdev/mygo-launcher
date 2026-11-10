package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// uiPrefs are the view's own settings, which the store does not hold: they
// belong to this window rather than to the launcher's data, and the Wails build
// kept them in the page's localStorage. They live in ui-state.json beside the
// store.
type uiPrefs struct {
	Theme     string `json:"theme"`               // auto, light or dark
	ActiveTab string `json:"active_tab"`          // guid of the tab shown
	Accent    string `json:"accent,omitempty"`    // preset id or #rrggbb; empty is blue
	Language  string `json:"language,omitempty"`  // auto, en or zh-Hans
	IconSize  string `json:"icon_size,omitempty"` // small, medium or large
	// GPU draws with the graphics card; off draws in memory to keep the
	// footprint low. The renderer is chosen when the window opens.
	GPU bool `json:"gpu,omitempty"`
	// WheelPages switches pages with the wheel instead of scrolling lists and
	// grids; it is on unless the settings turn it off.
	WheelPages bool `json:"wheel_pages"`
	// TrayOnly keeps the window out of the taskbar and unable to minimize, so
	// it opens from the tray or the shortcut alone. It needs running in the
	// background: closing would otherwise leave no way back.
	TrayOnly bool `json:"tray_only,omitempty"`
	// Shortcut shows or hides the launcher from any window, in the form
	// mygo's global shortcuts take, such as "Ctrl+Alt+Space"; empty is none.
	Shortcut string `json:"shortcut,omitempty"`
	// AppIcon is the program icon the user chose, kept in custom-icons;
	// empty draws the icon from the accent color.
	AppIcon string `json:"app_icon,omitempty"`
	// Background is the launcher's own picture, kept in custom-icons, and
	// BackgroundOpacity how much of it shows over the content's color.
	Background        string  `json:"background,omitempty"`
	BackgroundOpacity float64 `json:"background_opacity,omitempty"`
}

func defaultPrefs() uiPrefs {
	return uiPrefs{
		Theme:             "auto",
		Language:          "auto",
		IconSize:          "medium",
		WheelPages:        true,
		BackgroundOpacity: 0.4,
	}
}

func loadPrefs() uiPrefs {
	p, _ := readPrefs()
	return p
}

// readPrefs reads the preferences and reports whether the file was there and
// valid. Callers that delete files the preferences may name must know: an
// unreadable file is not a file with no choices.
func readPrefs() (uiPrefs, bool) {
	p := defaultPrefs()
	data, err := os.ReadFile(filepath.Join(dataBase(), uiStateFile))
	if err != nil {
		// No file is no choices, which pruning may take as it is; a file
		// that cannot be read or parsed is not.
		return p, os.IsNotExist(err)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return defaultPrefs(), false
	}
	switch p.Theme {
	case "auto", "light", "dark":
	default:
		p.Theme = "auto"
	}
	if !validAccent(p.Accent) {
		p.Accent = ""
	}
	switch p.Language {
	case "en", "zh-Hans":
	default:
		p.Language = "auto"
	}
	switch p.IconSize {
	case "small", "medium", "large":
	default:
		p.IconSize = "medium"
	}
	p.Shortcut = strings.TrimSpace(p.Shortcut)
	if !allowedIcon(p.AppIcon) {
		p.AppIcon = ""
	}
	if !allowedIcon(p.Background) {
		p.Background = ""
	}
	p.BackgroundOpacity = min(1, max(0, p.BackgroundOpacity))
	return p, true
}

func savePrefs(p uiPrefs) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Join(dataBase(), dataDir), 0o755); err != nil {
		return
	}
	path := filepath.Join(dataBase(), uiStateFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}
