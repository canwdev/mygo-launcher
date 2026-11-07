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
	// Shortcut shows or hides the launcher from any window, in the form
	// mygo's global shortcuts take, such as "Ctrl+Alt+Space"; empty is none.
	Shortcut string `json:"shortcut,omitempty"`
}

func defaultPrefs() uiPrefs {
	return uiPrefs{
		Theme:      "auto",
		Language:   "auto",
		IconSize:   "medium",
		WheelPages: true,
	}
}

func loadPrefs() uiPrefs {
	p := defaultPrefs()
	data, err := os.ReadFile(filepath.Join(absBase, uiStateFile))
	if err != nil {
		return p
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return defaultPrefs()
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
	return p
}

func savePrefs(p uiPrefs) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Join(absBase, dataDir), 0o755); err != nil {
		return
	}
	path := filepath.Join(absBase, uiStateFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}
