package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"go-launcher/locales"
)

// The launcher keeps its data in a folder of its own: the store, the icon cache
// and the window's settings. The names are part of the on-disk format, and the
// folder is what "Open data directory…" shows. It sits in dataBase: beside the
// executable's working directory for a portable copy, in the user's local
// application data for an installed one.
const (
	dataDir        = "mygo-launcher-data"
	saveFile       = dataDir + "/go-launcher-data.json"
	iconsDir       = dataDir + "/cached-icons"
	customIconsDir = dataDir + "/custom-icons"
	uiStateFile    = dataDir + "/ui-state.json"
)

// absBase is the directory stored paths are relative to, and dataBase the
// folder holding mygo-launcher-data.
var absBase, _ = filepath.Abs(".")

// AppItem is one entry of the launcher: a program, a file or a folder.
type AppItem struct {
	GUID       string `json:"guid"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Icon       string `json:"icon,omitempty"`
	RuntimeMs  int64  `json:"runtime_ms,omitempty"`
	Args       string `json:"args,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
	// RunAsAdmin comes from the "Run as administrator" check box of the item
	// dialog: true launches it elevated every time (ShellExecuteEx's "runas"
	// verb on Windows, so the system asks for UAC), false only falls back to
	// "runas" when the program itself needs elevation.
	RunAsAdmin bool `json:"run_as_admin,omitempty"`
	// Link is the shortcut the item was added from, kept to draw the
	// shortcut's icon; empty for an item added from its target directly.
	Link string `json:"link,omitempty"`
}

// CategoryNode is a tab: a row of slots, nil being an empty cell.
type CategoryNode struct {
	GUID  string    `json:"guid"`
	Name  string    `json:"name"`
	Slots []*string `json:"slots"`
}

type Settings struct {
	AbsolutePaths bool `json:"absolute_paths"`
	// AutoTime shows and accumulates an item's runtime while its process runs.
	// With it off the runtime is hidden and the clock never moves: the item
	// still starts and stops.
	AutoTime bool `json:"auto_time"`
	// AutoHide minimizes the launcher when an item is launched. It is the
	// master switch of the window hiding: with it off nothing is minimized at
	// all, and nothing brings the window back either way.
	AutoHide bool `json:"auto_hide"`
	// AlwaysOnTop keeps the window above the others (the app menu's "Always on
	// top"), restored at startup and applied as it is toggled.
	AlwaysOnTop bool `json:"always_on_top"`
	// BackgroundOnClose hides the window in the tray instead of quitting when
	// it is closed, and keeps the tray icon shown.
	BackgroundOnClose bool `json:"background_on_close"`
}

type AppStore struct {
	Apps       map[string]*AppItem `json:"apps"` // global app pool
	Categories []CategoryNode      `json:"categories"`
	Settings   Settings            `json:"settings"`
}

func toStoredPath(abs string) string {
	rel, err := filepath.Rel(absBase, abs)
	if err != nil {
		return abs
	}
	return rel
}

func absPath(p string) string {
	if p == "" || filepath.IsAbs(p) || isShellTarget(p) {
		return p
	}
	return filepath.Join(absBase, p)
}

// resolveLaunchPath resolves a bare command name (no separator, no drive
// letter) through PATH, so "notepad" and "cmd" launch their real executable;
// other inputs go through absPath. Empty stays empty.
func resolveLaunchPath(p string) string {
	if p == "" {
		return ""
	}
	if !strings.ContainsAny(p, "\\/") && !(len(p) >= 2 && p[1] == ':') {
		if full, err := exec.LookPath(p); err == nil {
			return full
		}
	}
	return absPath(p)
}

func defaultStore() AppStore {
	return AppStore{
		Apps: map[string]*AppItem{},
		Categories: []CategoryNode{
			{GUID: uuid.NewString(), Name: locales.DefaultTabName(), Slots: []*string{}},
		},
		Settings: Settings{AbsolutePaths: true, AutoTime: true, AutoHide: true, AlwaysOnTop: false, BackgroundOnClose: true},
	}
}

// settingsMissingKey reports whether the raw JSON's settings object lacks the
// given key (or the whole settings block is missing).
func settingsMissingKey(data []byte, key string) bool {
	var probe struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(data, &probe); err != nil || len(probe.Settings) == 0 {
		return true
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(probe.Settings, &keys); err != nil {
		return true
	}
	_, ok := keys[key]
	return !ok
}

// parseStore reads the store JSON and fills in defaults (the auto_time and
// auto_hide keys of older files, missing guids and names, path normalization).
// ok=false means the content is not valid JSON: a file caught mid-edit must not
// be taken as an empty store.
func parseStore(data []byte) (AppStore, bool) {
	var store AppStore
	if err := json.Unmarshal(data, &store); err != nil {
		return AppStore{}, false
	}
	if settingsMissingKey(data, "auto_time") {
		store.Settings.AutoTime = true
	}
	if settingsMissingKey(data, "auto_hide") {
		store.Settings.AutoHide = true
	}
	if settingsMissingKey(data, "background_on_close") {
		store.Settings.BackgroundOnClose = true
	}
	if store.Apps == nil {
		store.Apps = map[string]*AppItem{}
	}
	if len(store.Apps) == 0 && len(store.Categories) == 0 {
		return defaultStore(), true
	}
	for guid, app := range store.Apps {
		if app == nil {
			continue
		}
		app.Path = normalizePath(app.Path)
		if app.GUID == "" {
			app.GUID = guid
		}
		if app.Name == "" {
			app.Name = defaultTitle(absPath(app.Path))
		}
	}
	if len(store.Categories) == 0 {
		store.Categories = []CategoryNode{{GUID: uuid.NewString(), Name: locales.DefaultTabName(), Slots: []*string{}}}
	}
	for i := range store.Categories {
		cat := &store.Categories[i]
		if cat.GUID == "" {
			cat.GUID = uuid.NewString()
		}
		if cat.Name == "" {
			cat.Name = locales.UntitledTabName()
		}
		if cat.Slots == nil {
			cat.Slots = []*string{}
		}
	}
	return store, true
}

func loadStore() AppStore {
	data, err := os.ReadFile(filepath.Join(dataBase(), saveFile))
	if err != nil {
		return defaultStore()
	}
	store, ok := parseStore(data)
	if !ok {
		return defaultStore()
	}
	return store
}

// writeStoreAtomic writes the store and returns the bytes written, so the
// caller can record the file fingerprint that tells its own writes from edits
// made by hand.
func writeStoreAtomic(store AppStore) ([]byte, error) {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(dataBase(), dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dataBase(), saveFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}
	return data, nil
}

// convertItemPaths rewrites an item's paths as absolute or relative to absBase.
// Icons are left as they are: an icon path is kept relative to the data folder
// and does not follow the item's target.
func convertItemPaths(item *AppItem, absolute bool) {
	if item == nil {
		return
	}
	if absolute {
		if item.Path != "" {
			item.Path = absPath(item.Path)
		}
		if item.WorkingDir != "" {
			item.WorkingDir = absPath(item.WorkingDir)
		}
		if item.Link != "" {
			item.Link = absPath(item.Link)
		}
		return
	}
	if item.Path != "" {
		item.Path = toStoredPath(absPath(item.Path))
	}
	if item.WorkingDir != "" {
		item.WorkingDir = toStoredPath(absPath(item.WorkingDir))
	}
	if item.Link != "" {
		item.Link = toStoredPath(absPath(item.Link))
	}
}
