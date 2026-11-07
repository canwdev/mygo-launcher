package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"go-launcher/locales"
)

// The store file is watched so edits made by hand apply to the running app: a
// change to mygo-launcher-data/go-launcher-data.json is read back, compared with
// the fingerprint of the last write (content hash) to tell it from the app's
// own saves, and applied. Content that is not valid JSON keeps the store as it
// is, and says so, so a half-finished edit is not taken as an empty store.
const (
	// storeWatchRetry is how long to wait before watching again, after the
	// folder or the file went away.
	storeWatchRetry = 2 * time.Second
)

// storeFileState is the last known state of saveFile: mtime and size for the
// cheap "did not change" test, and the content hash that tells the app's own
// writes from someone else's.
type storeFileState struct {
	modTime time.Time
	size    int64
	hash    string
}

func (s storeFileState) matches(info os.FileInfo) bool {
	return !s.modTime.IsZero() && s.size == info.Size() && s.modTime.Equal(info.ModTime())
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type storeReloadResult int

const (
	storeReloadNone    storeReloadResult = iota // unchanged, our own write, or unreadable for now
	storeReloadApplied                          // an edit from outside was applied
	storeReloadInvalid                          // the file is not valid JSON; it was ignored
)

// syncStoreFileState records the file as it is on disk, as the app starts.
func (a *App) syncStoreFileState() {
	data, err := os.ReadFile(filepath.Join(dataBase(), saveFile))
	if err != nil {
		return
	}
	a.fileMu.Lock()
	a.rememberStoreFile(data)
	a.fileMu.Unlock()
}

// rememberStoreFile records the fingerprint of the bytes just written or read.
// The caller holds fileMu.
func (a *App) rememberStoreFile(data []byte) {
	state := storeFileState{hash: contentHash(data)}
	if info, err := os.Stat(filepath.Join(dataBase(), saveFile)); err == nil {
		state.modTime = info.ModTime()
		state.size = info.Size()
	}
	a.storeFile = state
}

// reloadStoreIfChanged applies an edit made by hand, if the file changed since
// the last write. It runs on the main thread, and does not redraw.
func (a *App) reloadStoreIfChanged() storeReloadResult {
	path := filepath.Join(dataBase(), saveFile)
	info, err := os.Stat(path)
	if err != nil {
		return storeReloadNone // being replaced, or gone: the next event tells
	}
	a.fileMu.Lock()
	known := a.storeFile
	a.fileMu.Unlock()
	if known.matches(info) {
		return storeReloadNone
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return storeReloadNone
	}
	hash := contentHash(data)
	info, err = os.Stat(path)
	if err != nil {
		return storeReloadNone
	}
	next := storeFileState{modTime: info.ModTime(), size: info.Size(), hash: hash}

	a.fileMu.Lock()
	defer a.fileMu.Unlock()
	if hash == a.storeFile.hash {
		// Only the timestamp or size moved: this is a write of the app's own.
		a.storeFile = next
		return storeReloadNone
	}
	store, ok := parseStore(data)
	a.storeFile = next
	if !ok {
		return storeReloadInvalid
	}
	a.applyStoreLocked(store)
	if a.win != nil {
		a.win.SetAlwaysOnTop(store.Settings.AlwaysOnTop)
	}
	return storeReloadApplied
}

// applyStoreLocked replaces the store with one edited by hand. The file is
// authoritative for the runtime, and items it dropped are no longer tracked
// (their processes keep running, untimed and unstoppable).
func (a *App) applyStoreLocked(store AppStore) {
	stats := make(map[string]int64, len(store.Apps))
	for guid, item := range store.Apps {
		if item != nil {
			stats[guid] = item.RuntimeMs
		}
	}
	a.runtimeStats = stats
	for guid := range a.running {
		if store.Apps[guid] == nil {
			delete(a.running, guid)
		}
	}
	a.store = store
}

// handleStoreFileEvent applies an edit from outside and tells the user.
func (a *App) handleStoreFileEvent() {
	switch a.reloadStoreIfChanged() {
	case storeReloadApplied:
		a.notify(locales.ReloadedStore())
	case storeReloadInvalid:
		a.notify(locales.StoreInvalid())
	}
	a.refresh()
}

// startStoreWatcher watches the store file for edits made by hand.
func (a *App) startStoreWatcher() {
	ctx, cancel := context.WithCancel(context.Background())
	a.watchCancel = cancel
	path := filepath.Join(dataBase(), saveFile)
	go func() {
		for ctx.Err() == nil {
			w, err := mygo.WatchFiles(ctx, path, mygo.FileWatchOptions{})
			if err != nil {
				// Not there yet: the app writes it on its first save.
				if !sleepCtx(ctx, storeWatchRetry) {
					return
				}
				continue
			}
			for {
				if _, err := w.Next(ctx); err != nil {
					break // the file went away or the watch was stopped
				}
				a.dispatch(func() { a.handleStoreFileEvent() })
			}
			_ = w.Close()
			if !sleepCtx(ctx, storeWatchRetry) {
				return
			}
		}
	}()
}

func (a *App) stopStoreWatcher() {
	if a.watchCancel != nil {
		a.watchCancel()
		a.watchCancel = nil
	}
}

// sleepCtx waits for d, and reports false when ctx was canceled meanwhile.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
