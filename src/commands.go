package main

import (
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
	// An icon outside the app's folders is offered too: updating takes it in.
	autoIcon := isAutoIcon(item.Icon) || !allowedIcon(item.Icon)
	msgs := v.msgs()
	return func(m *ui.Menu) {
		if m.Item(msgs.Edit()).Chosen() {
			v.later(func() { v.openEdit(guid) })
		}
		if m.Item(msgs.Duplicate()).Chosen() {
			v.later(func() { v.duplicateItem(guid, tabGUID, slot) })
		}
		if m.Item(msgs.InsertEmpty()).Chosen() {
			v.later(func() { v.insertEmptyAt(slot) })
		}
		if m.Item(msgs.RevealInFolder()).Chosen() {
			v.later(func() { v.app.notifyErr(v.app.reveal(guid)) })
		}
		m.Separator()
		if m.Item(msgs.Rename()).Chosen() {
			v.later(func() { v.openItemRename(guid, name) })
		}
		if m.Item(msgs.Remove()).Chosen() {
			v.later(func() { v.ask(msgs.RemoveItemConfirm(name), func() { v.removeItem(guid) }) })
		}
		m.Separator()
		if autoIcon {
			if m.Item(msgs.UpdateIcon()).Chosen() {
				v.later(func() { v.app.notifyErr(v.app.updateIcon(guid)) })
			}
		}
		if m.Item(msgs.ConvertToAbsolute()).Chosen() {
			v.later(func() { v.app.notifyErr(v.app.convertItemPath(guid, true)) })
		}
		if m.Item(msgs.ConvertToRelative()).Chosen() {
			v.later(func() { v.app.notifyErr(v.app.convertItemPath(guid, false)) })
		}
	}
}

// runItem runs an item: executables are tracked, so their runtime accumulates
// until they exit and they can be stopped. A launch may wait on an elevation
// prompt, so it runs on a goroutine; what it changes comes back through the
// window.
func (v *view) runItem(guid string) {
	if at, ok := v.launchAt[guid]; ok && time.Since(at) < launchCooldown {
		return
	}
	item := v.app.item(guid)
	if item == nil {
		return
	}
	v.launchAt[guid] = time.Now()
	plan, err := v.app.planLaunch(guid)
	if err != nil {
		if err != errAlreadyRunning {
			v.app.notify(err.Error())
		}
		return
	}
	go v.app.runLaunch(plan)
}

func (v *view) stopItem(guid string) {
	if err := v.app.stop(guid); err != nil {
		v.app.notify(err.Error())
	}
}

// === Runtime ===

// baseline is an item's accumulated runtime, without what a running process is
// adding now.
func (v *view) baseline(guid string) int64 {
	if st, ok := v.states[guid]; ok {
		return st.RuntimeMs
	}
	if item := v.app.item(guid); item != nil {
		return item.RuntimeMs
	}
	return 0
}

// liveMs is what this session adds to the runtime: the time the tracked process
// has been running, while auto timing is on.
func (v *view) liveMs(guid string) int64 {
	if !v.app.store.Settings.AutoTime {
		return 0
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
	text := formatRuntime(v.lang, v.baseline(guid))
	if ms := v.liveMs(guid); ms > 0 {
		text += " (+ " + formatRuntime(v.lang, ms) + ")"
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
	v.editIconIndex = "0"
	v.editAdmin = item.RunAsAdmin
}

func (v *view) openCreate() {
	if v.tab() == nil {
		v.addTab()
	}
	v.editOpen, v.editNew, v.editGUID = true, true, ""
	v.editName, v.editPath, v.editArgs = "", "", ""
	v.editWorkDir, v.editIcon, v.editAdmin = "", "", false
	v.editIconIndex = "0"
}

func (v *view) saveItemDialog() {
	icon, err := v.app.customIcon(v.editIcon, iconIndex(v.editIconIndex))
	if err != nil {
		v.app.notify(err.Error())
		return // the dialog stays open, to choose another icon
	}
	fields := itemFields{
		Name:       strings.TrimSpace(v.editName),
		Path:       strings.TrimSpace(v.editPath),
		Args:       strings.TrimSpace(v.editArgs),
		WorkDir:    strings.TrimSpace(v.editWorkDir),
		Icon:       icon,
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
	v.renameTitle = v.msgs().Rename()
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

// iconIndex reads the icon index field: the first icon when it is not a
// number or negative.
func iconIndex(text string) int {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || n < 0 {
		return 0
	}
	return n
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

// batchUpdateIcons draws every auto icon again, and takes in the icons still
// outside the app's folders: the paths are read on the main thread, the icons
// on a goroutine, and the result is applied back on the main thread.
func (v *view) batchUpdateIcons() {
	type job struct {
		guid string
		path string
		icon bool // the item's own icon, outside the app's folders
	}
	var jobs []job
	for guid, item := range v.app.store.Apps {
		if item == nil {
			continue
		}
		switch {
		case item.Icon != "" && !allowedIcon(item.Icon):
			jobs = append(jobs, job{guid: guid, path: absPath(item.Icon), icon: true})
		case isAutoIcon(item.Icon):
			path := ""
			switch {
			case item.Link != "":
				path = absPath(item.Link)
			case item.Path != "":
				path = resolveLaunchPath(item.Path)
			}
			if path != "" {
				jobs = append(jobs, job{guid: guid, path: path})
			}
		}
	}
	v.app.setBusy(v.msgs().UpdatingIcons())
	go func() {
		type result struct{ guid, icon string }
		results := make([]result, 0, len(jobs))
		for _, j := range jobs {
			icon := ""
			if j.icon {
				icon, _ = v.app.customIcon(j.path, 0)
			} else {
				icon = writeIcon(j.path)
			}
			if icon != "" {
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
					if isAutoIcon(old) || isCustomIcon(old) {
						_ = os.Remove(absPath(old))
					}
					delete(v.app.icons, old)
				}
				item.Icon = r.icon
			}
			v.app.save()
			v.app.notify(v.msgs().UpdatedIcons(len(results)))
		})
	}()
}

func (v *view) convertPaths(absolute bool) {
	m := v.msgs()
	v.app.setBusy(m.ConvertingPaths())
	v.app.convertPaths(absolute)
	v.app.setBusy("")
	if absolute {
		v.app.notify(m.ConvertedToAbsolute())
	} else {
		v.app.notify(m.ConvertedToRelative())
	}
}

// refreshData applies an edit made by hand and says so, as "Refresh" does.
func (v *view) refreshData() {
	v.app.handleStoreFileEvent()
	v.app.notify(v.msgs().DataRefreshed())
}
