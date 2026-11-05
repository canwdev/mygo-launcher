package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// launchCooldown keeps a double click from running an item twice.
const launchCooldown = time.Second

// itemMenu is an item's menu, shared by a row's right click, a cell's ⋯ and a
// row's ⋯.
func (v *view) itemMenu(item *AppItem, slot int) func(m *ui.Menu) {
	guid, name := item.GUID, item.Name
	tabGUID := v.prefs.ActiveTab
	autoIcon := isAutoIcon(item.Icon)
	return func(m *ui.Menu) {
		if m.Item("Edit").Chosen() {
			v.later(func() { v.openEdit(guid) })
		}
		if m.Item("Duplicate").Chosen() {
			v.later(func() { v.duplicateItem(guid, tabGUID, slot) })
		}
		if m.Item("Insert empty").Chosen() {
			v.later(func() { v.insertEmptyAt(slot) })
		}
		if m.Item("Open folder…").Chosen() {
			v.later(func() { v.app.notifyErr(v.app.reveal(guid)) })
		}
		m.Separator()
		if m.Item("Rename").Chosen() {
			v.later(func() { v.openItemRename(guid, name) })
		}
		if m.Item("Delete").Chosen() {
			v.later(func() { v.ask("Delete \""+name+"\"?", func() { v.removeItem(guid) }) })
		}
		m.Separator()
		if autoIcon {
			if m.Item("Update icon").Chosen() {
				v.later(func() { v.app.notifyErr(v.app.updateIcon(guid)) })
			}
		}
		if m.Item("Convert to absolute path").Chosen() {
			v.later(func() { v.app.notifyErr(v.app.convertItemPath(guid, true)) })
		}
		if m.Item("Convert to relative path").Chosen() {
			v.later(func() { v.app.notifyErr(v.app.convertItemPath(guid, false)) })
		}
	}
}

// runItem runs an item: tracked in game mode, or opened without tracking when it
// times itself. A launch may wait on an elevation prompt, so it runs on a
// goroutine; what it changes comes back through the window.
func (v *view) runItem(guid string) {
	if at, ok := v.launchAt[guid]; ok && time.Since(at) < launchCooldown {
		return
	}
	item := v.app.item(guid)
	if item == nil {
		return
	}
	v.launchAt[guid] = time.Now()
	autoTimer := v.prefs.AutoTimers[guid]
	plan, err := func() (launchPlan, error) {
		if autoTimer {
			return v.app.planOpen(guid)
		}
		return v.app.planLaunch(guid)
	}()
	if err != nil {
		if err != errAlreadyRunning {
			v.app.notify(err.Error())
		}
		return
	}
	go func() {
		v.app.runLaunch(plan)
		if autoTimer && plan.path != "" {
			v.app.dispatch(func() { v.startTimer(guid) })
		}
	}()
}

func (v *view) stopItem(guid string) {
	if err := v.app.stop(guid); err != nil {
		v.app.notify(err.Error())
	}
}

// === Timers ===

func (v *view) timerActive(guid string) bool {
	_, ok := v.prefs.Timers[guid]
	return ok
}

func (v *view) elapsed(guid string) int64 {
	start, ok := v.prefs.Timers[guid]
	if !ok || start <= 0 {
		return 0
	}
	return max(0, time.Now().UnixMilli()-start)
}

// startTimer starts a manual timer for an item; one already running keeps
// going, so an auto timer does not reset it.
func (v *view) startTimer(guid string) {
	if v.timerActive(guid) {
		return
	}
	v.prefs.Timers[guid] = time.Now().UnixMilli()
	v.savePrefs()
	v.app.refresh()
}

// stopTimer stops an item's timer and adds what it counted to the runtime the
// store keeps.
func (v *view) stopTimer(guid string) {
	if !v.timerActive(guid) {
		return
	}
	ms := v.elapsed(guid)
	delete(v.prefs.Timers, guid)
	v.savePrefs()
	if ms <= 0 {
		v.app.notify("Timer stopped")
		return
	}
	if err := v.app.setRuntimeMs(guid, v.baseline(guid)+ms); err != nil {
		v.app.notify(err.Error())
		return
	}
	v.app.notify("Runtime saved")
}

// baseline is an item's accumulated runtime, without what a running process or
// a timer is adding now.
func (v *view) baseline(guid string) int64 {
	if st, ok := v.states[guid]; ok {
		return st.RuntimeMs
	}
	if item := v.app.item(guid); item != nil {
		return item.RuntimeMs
	}
	return 0
}

// liveMs is what this session adds to the runtime: a timer's count, or a
// running process' time since it started.
func (v *view) liveMs(guid string) int64 {
	if v.timerActive(guid) {
		return v.elapsed(guid)
	}
	st, ok := v.states[guid]
	if !ok || !st.Running || st.StartAt <= 0 {
		return 0
	}
	live := st.RuntimeMs + (time.Now().UnixMilli() - st.StartAt)
	if live > st.RuntimeMs {
		return live - st.RuntimeMs
	}
	return 0
}

// runtimeText spells an item's runtime as a row shows it: "1h 2m", and what is
// being added now as "(+ 3m)".
func (v *view) runtimeText(guid string) string {
	text := formatRuntime(v.baseline(guid))
	if ms := v.liveMs(guid); ms > 0 {
		text += " (+ " + formatRuntime(ms) + ")"
	}
	return text
}

// === Dialogs ===

func (v *view) openRuntime(guid string) {
	v.runtimeGUID = guid
	v.runtimeMinutes = strconv.FormatInt(v.baseline(guid)/60000, 10)
	v.runtimeOpen = true
}

func (v *view) openEdit(guid string) {
	item := v.app.item(guid)
	if item == nil {
		return
	}
	v.editOpen, v.editNew, v.editGUID = true, false, guid
	v.editName = item.Name
	v.editPath = item.Path
	v.editArgs = item.Args
	v.editWorkDir = item.WorkingDir
	v.editIcon = item.Icon
	v.editAdmin = item.RunAsAdmin
}

func (v *view) openCreate() {
	if v.tab() == nil {
		v.addTab()
	}
	v.editOpen, v.editNew, v.editGUID = true, true, ""
	v.editName, v.editPath, v.editArgs = "", "", ""
	v.editWorkDir, v.editIcon, v.editAdmin = "", "", false
}

func (v *view) saveItemDialog() {
	fields := itemFields{
		Name:       strings.TrimSpace(v.editName),
		Path:       strings.TrimSpace(v.editPath),
		Args:       strings.TrimSpace(v.editArgs),
		WorkDir:    strings.TrimSpace(v.editWorkDir),
		Icon:       strings.TrimSpace(v.editIcon),
		RunAsAdmin: v.editAdmin,
	}
	if v.editNew {
		v.createItem(fields)
	} else {
		v.updateItem(v.editGUID, fields)
	}
	v.editOpen = false
}

func (v *view) openItemRename(guid, name string) {
	v.renameOpen = true
	v.renameTitle = "Rename"
	v.renameName = name
	v.renameGUID = guid
	v.renameTabGUID = ""
}

// applyRename saves the name the rename dialog holds.
func (v *view) applyRename() {
	name := strings.TrimSpace(v.renameName)
	if name == "" {
		return
	}
	if v.renameGUID != "" {
		v.updateItem(v.renameGUID, v.itemFields(v.renameGUID, name))
	} else if v.renameTabGUID != "" {
		v.renameTab(v.renameTabGUID, name)
	}
	v.renameOpen = false
}

// itemFields reads an item's values, with name replacing its own.
func (v *view) itemFields(guid, name string) itemFields {
	item := v.app.item(guid)
	if item == nil {
		return itemFields{Name: name}
	}
	return itemFields{
		Name:       name,
		Path:       item.Path,
		Args:       item.Args,
		WorkDir:    item.WorkingDir,
		Icon:       item.Icon,
		RunAsAdmin: item.RunAsAdmin,
	}
}

// === Store-wide actions ===

// batchUpdateIcons extracts every auto icon again: the paths are read on the
// main thread, the icons on a goroutine, and the result is applied back on the
// main thread.
func (v *view) batchUpdateIcons() {
	type job struct{ guid, path string }
	var jobs []job
	for guid, item := range v.app.store.Apps {
		if item != nil && item.Path != "" && isAutoIcon(item.Icon) {
			jobs = append(jobs, job{guid, resolveLaunchPath(item.Path)})
		}
	}
	v.app.setBusy("Updating icons…")
	go func() {
		type result struct{ guid, icon string }
		results := make([]result, 0, len(jobs))
		for _, j := range jobs {
			if icon := writeIcon(j.path); icon != "" {
				results = append(results, result{j.guid, icon})
			}
		}
		v.app.dispatch(func() {
			v.app.setBusy("")
			for _, r := range results {
				item := v.app.item(r.guid)
				if item == nil {
					continue
				}
				if old := item.Icon; old != "" {
					_ = os.Remove(absPath(old))
					delete(v.app.icons, old)
				}
				item.Icon = r.icon
			}
			v.app.save()
			plural := "s"
			if len(results) == 1 {
				plural = ""
			}
			v.app.notify(fmt.Sprintf("Updated %d icon%s", len(results), plural))
		})
	}()
}

func (v *view) convertPaths(absolute bool) {
	v.app.setBusy("Converting paths…")
	v.app.convertPaths(absolute)
	v.app.setBusy("")
	if absolute {
		v.app.notify("Converted to absolute path")
	} else {
		v.app.notify("Converted to relative path")
	}
}

// refreshData applies an edit made by hand and says so, as "Refresh" does.
func (v *view) refreshData() {
	v.app.handleStoreFileEvent()
	v.app.notify("Data refreshed")
}
