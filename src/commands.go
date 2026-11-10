package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// launchCooldown keeps a double click from running an item twice.
const launchCooldown = time.Second

// itemMenu is an item's menu, opened by a right click on its cell.
func (v *view) itemMenu(item *AppItem, tabGUID string, slot int) []menuEntry {
	guid, name := item.GUID, item.Name
	msgs := v.msgs()
	// A shell folder or a URL has no file to show in Explorer, so its entry
	// opens it instead.
	open := menuEntry{icon: iconFolderOpen, label: msgs.RevealInFolder(), action: func() {
		v.later(func() { v.app.notifyErr(v.app.reveal(guid)) })
	}}
	if isShellTarget(item.Path) {
		open = menuEntry{icon: iconArrowUpRight, label: msgs.OpenMain(), action: func() {
			v.later(func() { v.runItem(guid) })
		}}
	}
	entries := []menuEntry{
		{icon: iconEdit, label: msgs.Edit(), action: func() {
			v.later(func() { v.openEdit(guid) })
		}},
		{icon: iconCopy, label: msgs.Duplicate(), action: func() {
			v.later(func() { v.duplicateItem(guid, tabGUID, slot) })
		}},
		{icon: iconSquarePlus, label: msgs.InsertEmpty(), action: func() {
			v.later(func() { v.insertEmptyAt(slot) })
		}},
		open,
		{},
		{icon: iconRename, label: msgs.Rename(), action: func() {
			v.later(func() { v.openItemRename(guid, name) })
		}},
		{icon: iconTrash, label: msgs.Remove(), action: func() {
			v.later(func() { v.ask(msgs.RemoveItemConfirm(name), func() { v.removeItem(guid) }) })
		}},
	}
	// The shell's own context menu, for files, folders and shell folders.
	// Packaged apps and URLs have none. Stored paths may be relative to the
	// data base, as launching resolves them.
	shellPath := item.Path
	if shellPath != "" && !isShellTarget(shellPath) {
		shellPath = absPath(shellPath)
	}
	shellActions := canUseShellActions(shellPath)
	if shellActions {
		entries = append(entries, menuEntry{}, menuEntry{icon: iconMenu, label: msgs.SystemMenu(), action: func() {
			// The native menu pumps its own messages; show it after this frame.
			v.app.dispatch(func() {
				v.app.notifyErr(showShellMenu(shellPath))
				v.app.refresh()
			})
		}})
	}
	// A special object's own entries, as emptying the Recycle Bin. They read
	// the shell, so they are built only while the menu shows.
	if isOpen := v.menus[itemMenuKey(tabGUID, guid, slot)]; isOpen != nil && *isOpen {
		if sp := specialItemFor(item.Path); sp != nil && sp.Menu != nil {
			entries = append(entries, sp.Menu(v, item)...)
		}
	}
	// The shell's properties page is the last entry, as in Explorer's menu.
	if shellActions {
		entries = append(entries, menuEntry{icon: iconProperties, label: msgs.Properties(), action: func() {
			v.app.dispatch(func() { v.app.notifyErr(showItemProperties(shellPath)) })
		}})
	}
	return entries
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
	wasMissing := v.app.isMissing(guid)
	plan, err := v.app.planLaunch(guid)
	if err != nil {
		if err != errAlreadyRunning {
			v.app.notify(err.Error())
		}
		return
	}
	if wasMissing {
		v.refresh() // the item is back, so its dimming goes now
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

// runtimeText spells an item's runtime as a row shows it: "1h2m", and what is
// being added now as "(+ 3m)".
func (v *view) runtimeText(guid string) string {
	text := formatRuntime(v.baseline(guid))
	if ms := v.liveMs(guid); ms > 0 {
		text += " (+ " + formatRuntime(ms) + ")"
	}
	return text
}

// === Dialogs ===

// openRuntime opens the dialog that edits an item's runtime. Only a program
// that is timed has one; a folder's or a file's cell shows no timer to open it
// from.
func (v *view) openRuntime(guid string) {
	if !timeableItem(v.app.item(guid)) {
		return
	}
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

// updateIconFromDialog redraws the edited item's icon and shows the stored
// icon in the dialog.
func (v *view) updateIconFromDialog() {
	if v.editGUID == "" {
		return
	}
	if err := v.app.updateIcon(v.editGUID); err != nil {
		v.app.notify(err.Error())
		return
	}
	if item := v.app.item(v.editGUID); item != nil {
		v.editIcon = item.Icon
		v.editIconIndex = "0"
	}
	v.refresh()
}

// convertFromDialog stores the edited item's paths absolute or relative, and
// shows the stored forms in the dialog.
func (v *view) convertFromDialog(absolute bool) {
	if v.editGUID == "" {
		return
	}
	if err := v.app.convertItemPath(v.editGUID, absolute); err != nil {
		v.app.notify(err.Error())
		return
	}
	if item := v.app.item(v.editGUID); item != nil {
		v.editPath = item.Path
		v.editWorkDir = item.WorkingDir
		v.editIcon = item.Icon
	}
	v.refresh()
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
// number. A negative index is a resource ID, as the shell's picker returns
// for icons a file holds only by ID.
func iconIndex(text string) int {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
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
				icon = contentIcon(j.path)
				if icon == "" {
					icon = writeIcon(j.path)
				}
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

// checkAvailability checks every item's file or folder, on a goroutine since a
// network path may be slow, and dims the items that are gone. Items the check
// no longer covers, as shell targets, lose their dimming.
func (v *view) checkAvailability() {
	type check struct{ guid, path string }
	var checks []check
	missing := map[string]bool{}
	for guid, item := range v.app.store.Apps {
		if item == nil {
			continue
		}
		switch {
		case isShellTarget(item.Path):
			// The shell resolves it; there is no file to check.
		case item.Path == "":
			if item.WorkingDir == "" {
				missing[guid] = true
				continue
			}
			checks = append(checks, check{guid, absPath(item.WorkingDir)})
		default:
			checks = append(checks, check{guid, resolveLaunchPath(item.Path)})
		}
	}
	run := func() {
		for _, c := range checks {
			if !fileExists(c.path) {
				missing[c.guid] = true
			}
		}
		v.app.dispatch(func() {
			v.app.setBusy("")
			v.app.missing = missing
			v.app.notify(v.msgs().AvailabilityChecked(len(missing)))
		})
	}
	if v.app.win == nil {
		run() // tests: there is no message loop to wait for
		return
	}
	v.app.setBusy(v.msgs().CheckAvailability())
	go run()
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
