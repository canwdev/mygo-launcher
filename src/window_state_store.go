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

// windowStateStore keeps the window's geometry in mygo-launcher-data instead of
// MyGo's own file in the user's config folder, beside the store.
type windowStateStore struct{}

// windowStatesFile is the file's format: the state of every window, by key.
type windowStatesFile struct {
	Windows map[string]mygo.WindowState `json:"windows"`
}

func (windowStateStore) LoadWindowStates() (map[string]mygo.WindowState, error) {
	data, err := os.ReadFile(filepath.Join(dataBase(), windowStateFile))
	if err != nil {
		return nil, nil // first run: the window's options apply
	}
	var file windowStatesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, nil // an unreadable file: the window's options apply
	}
	return file.Windows, nil
}

func (windowStateStore) SaveWindowStates(states map[string]mygo.WindowState) error {
	data, err := json.MarshalIndent(windowStatesFile{Windows: states}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(dataBase(), dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dataBase(), windowStateFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
