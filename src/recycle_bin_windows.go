//go:build windows

package main

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/egoist/mygo"
	"go-launcher/locales"
)

const recycleBinInfoSize = 24 // SHQUERYRBINFO: DWORD cbSize, padding, two __int64

type recycleBinInfo struct {
	Size  uint32
	_     uint32
	Bytes int64
	Items int64
}

var (
	procSHQueryRecycleBin = shell32.NewProc("SHQueryRecycleBinW")
	procSHEmptyRecycleBin = shell32.NewProc("SHEmptyRecycleBinW")

	recycleBinMu  sync.Mutex
	recycleBinAt  time.Time
	recycleBinNum int64
	recycleBinOK  bool

	recycleBinIconMu    sync.Mutex
	recycleBinIconState = map[string]string{}
)

// recycleBinItemCount reads the Recycle Bin's item count. Tests replace it.
var recycleBinItemCount = func() (int64, bool) {
	info, ok := queryRecycleBin()
	return info.Items, ok
}

func queryRecycleBin() (recycleBinInfo, bool) {
	info := recycleBinInfo{Size: recycleBinInfoSize}
	r, _, _ := procSHQueryRecycleBin.Call(0, uintptr(unsafe.Pointer(&info)))
	if int32(r) < 0 {
		return recycleBinInfo{}, false
	}
	return info, true
}

// recycleBinItems returns the item count, cached briefly: an item's menu is
// built every frame while it is open.
func recycleBinItems() (int64, bool) {
	recycleBinMu.Lock()
	defer recycleBinMu.Unlock()
	if !recycleBinAt.IsZero() && time.Since(recycleBinAt) < 2*time.Second {
		return recycleBinNum, recycleBinOK
	}
	recycleBinNum, recycleBinOK = recycleBinItemCount()
	recycleBinAt = time.Now()
	return recycleBinNum, recycleBinOK
}

func forgetRecycleBinItems() {
	recycleBinMu.Lock()
	recycleBinAt = time.Time{}
	recycleBinMu.Unlock()
}

// recycleBinIcon returns a new icon when the Recycle Bin's shell icon changed,
// which it does when the bin becomes empty or stops being empty. The image
// stays in memory: it changes too often to write a cache file each time.
func recycleBinIcon(item *AppItem) ([]byte, bool) {
	info, ok := queryRecycleBin()
	if !ok {
		return nil, false
	}
	state := fmt.Sprintf("%d/%d", info.Items, info.Bytes)
	recycleBinIconMu.Lock()
	old, seen := recycleBinIconState[item.Path]
	recycleBinIconMu.Unlock()
	if seen && old == state {
		return nil, false
	}
	data, err := mygo.Shell.FileIcon(item.Path, iconPixels())
	if err != nil || len(data) == 0 {
		return nil, false
	}
	recycleBinIconMu.Lock()
	recycleBinIconState[item.Path] = state
	recycleBinIconMu.Unlock()
	return data, true
}

// emptyRecycleBin clears the Recycle Bin without the shell's own confirmation
// or progress window: the launcher asks and shows its busy line instead.
func emptyRecycleBin() error {
	const flags = 0x1 | 0x2 // SHERB_NOCONFIRMATION | SHERB_NOPROGRESSUI
	r, _, _ := procSHEmptyRecycleBin.Call(0, 0, flags)
	if int32(r) < 0 {
		return fmt.Errorf("empty the Recycle Bin: 0x%08x", uint32(int32(r)))
	}
	return nil
}

// emptyRecycleBin empties the bin for an item, showing the busy line while it
// runs and redrawing the item's icon after.
func (a *App) emptyRecycleBin(item *AppItem) {
	if a.win == nil {
		return // tests: no shell window to act for
	}
	a.setBusy(locales.EmptyingRecycleBin())
	go func() {
		err := emptyRecycleBin()
		a.dispatch(func() {
			a.setBusy("")
			forgetRecycleBinItems()
			a.refreshSpecialIcon(item)
			if err != nil {
				a.notify(err.Error())
			} else {
				a.notify(locales.RecycleBinEmptied())
			}
			a.refresh()
		})
	}()
}
