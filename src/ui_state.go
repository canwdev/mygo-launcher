package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// uiPrefs are the view's own settings, which the store does not hold: they
// belong to this window rather than to the launcher's data, and the Wails build
// kept them in the page's localStorage. They live in ui-state.json beside the
// store.
type uiPrefs struct {
	Theme     string `json:"theme"`               // auto, light or dark
	ViewMode  string `json:"view_mode"`           // list or grid
	ActiveTab string `json:"active_tab"`          // guid of the tab shown
	Accent    string `json:"accent,omitempty"`    // preset id or #rrggbb; empty is blue
	Language  string `json:"language,omitempty"`  // auto, en or zh-Hans
	IconSize  string `json:"icon_size,omitempty"` // small, medium or large
}

func defaultPrefs() uiPrefs {
	return uiPrefs{
		Theme:    "auto",
		ViewMode: "list",
		Language: "auto",
		IconSize: "medium",
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
	if p.ViewMode != "grid" {
		p.ViewMode = "list"
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
