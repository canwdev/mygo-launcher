package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// windowStateKey names the main window in the state store.
const windowStateKey = "main"

// windowStateFile is where the window's geometry is kept, beside the store, as
// the Wails build kept it.
const windowStateFile = dataDir + "/window-state.json"

// windowStateStore keeps the window's geometry in .go-launcher-data instead of
// MyGo's own file in the user's config folder: the launcher keeps all its data
// next to the executable.
type windowStateStore struct{}

// windowStatesFile is the file's format: the state of every window, by key.
type windowStatesFile struct {
	Windows map[string]mygo.WindowState `json:"windows"`
}

// legacyWindowState is the geometry the Wails build saved: a single window, and
// "maximised" spelled the way it was.
type legacyWindowState struct {
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Maximised bool `json:"maximised"`
}

func (windowStateStore) LoadWindowStates() (map[string]mygo.WindowState, error) {
	data, err := os.ReadFile(filepath.Join(absBase, windowStateFile))
	if err != nil {
		return nil, nil // first run: the window's options apply
	}
	var file windowStatesFile
	if err := json.Unmarshal(data, &file); err == nil && len(file.Windows) > 0 {
		return file.Windows, nil
	}
	// The window-state.json of the Wails build: one window, other key names.
	var legacy legacyWindowState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, nil
	}
	if legacy.Width <= 0 || legacy.Height <= 0 {
		return nil, nil
	}
	return map[string]mygo.WindowState{
		windowStateKey: {
			X:         legacy.X,
			Y:         legacy.Y,
			Width:     legacy.Width,
			Height:    legacy.Height,
			Maximized: legacy.Maximised,
		},
	}, nil
}

func (windowStateStore) SaveWindowStates(states map[string]mygo.WindowState) error {
	data, err := json.MarshalIndent(windowStatesFile{Windows: states}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(absBase, dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(absBase, windowStateFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
