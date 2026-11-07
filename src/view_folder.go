package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// folderPanelWidth is how wide the folder panel is, folderRowHeight how tall
// one entry is, and folderPanelRows the most entries it shows before its list
// scrolls.
const (
	folderPanelWidth float32 = 320
	folderRowHeight  float32 = 36
	folderPanelRows          = 10
	// folderPanelMargin is the room the panel keeps from the window's edges.
	folderPanelMargin float32 = 8
	// folderPanelChrome is what the panel takes besides its list: its padding,
	// the header and the line below it.
	folderPanelChrome float32 = 54
)

// openEntryFile opens a file of the folder panel with the system's default
// handler, as a launch of a file item does; tests replace it.
var openEntryFile = func(path string) error { return openFile(path, nil, filepath.Dir(path), false) }

// showEntryMenu shows a folder panel entry's shell menu; tests replace it.
var showEntryMenu = showShellMenu

// folderEntry is one row of the folder panel: a folder it can enter, or a file
// it opens.
type folderEntry struct {
	path string // absolute
	name string
	dir  bool
}

// folderOfItem reports the folder an item names, or "" when it names something
// else. A folder item lists its contents when its cell is clicked; a file, a
// program, a URL and a shell target launch as they always did. A folder that is
// gone reports "" too, so the click says the path is missing rather than
// showing an empty panel.
func folderOfItem(item *AppItem) string {
	if item == nil {
		return ""
	}
	path := ""
	switch {
	case item.Path != "":
		if isShellTarget(item.Path) {
			return ""
		}
		path = resolveLaunchPath(item.Path)
	case item.WorkingDir != "":
		path = absPath(item.WorkingDir)
	}
	if path == "" {
		return ""
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return ""
	}
	return path
}

// readFolder lists the entries of dir, folders first and then by name, as a
// file manager shows them. Hidden and system entries stay out, as they do in
// Explorer. It reads the disk.
func readFolder(dir string) ([]folderEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	list := make([]folderEntry, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		if skipFolderEntry(e.Name(), info) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		dir := e.IsDir()
		if e.Type()&fs.ModeSymlink != 0 {
			// A link to a folder enters as the folder itself would.
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				dir = true
			}
		}
		list = append(list, folderEntry{path: path, name: e.Name(), dir: dir})
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.dir != b.dir {
			return a.dir
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
	return list, nil
}

// openFolderMenu lists the contents of the folder an item names, in a panel
// centered on the pointer that opened it.
func (v *view) openFolderMenu(guid, dir string, atX, atY float32) {
	v.folderGUID = guid
	v.folderRoot = dir
	v.folderAtX, v.folderAtY = atX, atY
	// The icons of the files browsed stay with the session: going back up
	// shows them at once.
	v.folderIcons = map[string]ui.ImageSource{}
	v.folderIconWanted = map[string]bool{}
	v.folderIconQueue, v.folderIconBusy = nil, false
	v.folderList.ScrollTo(0, ui.Start)
	v.loadFolder(dir)
	v.refresh()
}

// closeFolderMenu drops the panel, and what it read.
func (v *view) closeFolderMenu() {
	v.folderGUID, v.folderRoot, v.folderDir = "", "", ""
	v.folderItems, v.folderErr = nil, nil
	v.folderLoading = false
	// A read still running must not land on the next panel.
	v.folderGen++
	v.folderIcons, v.folderIconWanted = nil, nil
	v.folderIconQueue, v.folderIconBusy = nil, false
	v.refresh()
}

// loadFolder lists dir for the panel: on a goroutine while there is a window,
// so a slow disk or a network folder does not stall the frame; at once without
// one, as in tests.
func (v *view) loadFolder(dir string) {
	v.folderGen++
	gen := v.folderGen
	v.folderDir, v.folderItems, v.folderErr = dir, nil, nil
	v.folderLoading = true
	v.folderList.ScrollTo(0, ui.Start)
	if v.app.win == nil {
		v.folderItems, v.folderErr = readFolder(dir)
		v.folderLoading = false
		return
	}
	go func() {
		items, err := readFolder(dir)
		v.app.dispatch(func() {
			if gen != v.folderGen {
				return
			}
			v.folderLoading = false
			v.folderItems, v.folderErr = items, err
			v.refresh()
		})
	}()
}

// folderMenu shows the folder panel below the cell of the item it lists. It
// leaves the window's edge room, and its list scrolls when the folder holds
// more entries than it shows.
func (v *view) folderMenu(c *ui.Context) {
	if v.folderGUID == "" {
		return
	}
	if !v.folderAnchored {
		// The item left the page, by another page or its removal: the panel
		// goes with it.
		v.closeFolderMenu()
		return
	}
	p := v.pal
	m := v.msgs()
	width, height := c.Size()
	// The panel is centered on the pointer that opened it, and kept inside the
	// window: the room it finds decides how many entries it shows.
	panelWidth := min(folderPanelWidth, width-2*folderPanelMargin)
	rows := int((height - 2*folderPanelMargin - folderPanelChrome) / folderRowHeight)
	rows = max(1, min(rows, len(v.folderItems), folderPanelRows))
	listHeight := float32(rows) * folderRowHeight
	panelHeight := folderPanelChrome + listHeight
	left := max(folderPanelMargin, min(v.folderAtX-panelWidth/2, width-panelWidth-folderPanelMargin))
	top := max(folderPanelMargin, min(v.folderAtY-panelHeight/2, height-panelHeight-folderPanelMargin))

	ui.Overlay(c, func() {
		back := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Modal()
		if back.OverlayShortcut(0, ui.KeyEscape) {
			v.closeFolderMenu()
		}
		back.OnClick(v.closeFolderMenu)
		back.Children(func() {
			panel := ui.Column(c).Width(panelWidth).Padding(6).Radius(12).
				Background(p.bg).Border(1, p.hairline).
				Shadow(0, 12, 30, -6, ui.RGBA(0, 0, 0, 0.3)).
				Role(ui.RolePopup).Label(m.FolderContents()).OnClick(func() {}).
				Absolute().Left(left).Top(top)
			panel.Children(func() {
				v.folderHeader(c)
				ui.Box(c).Height(1).Margin(4, 2).Background(p.hairline)
				switch {
				case v.folderLoading:
					v.folderNote(c, m.FolderLoading(), true)
				case v.folderErr != nil:
					v.folderNote(c, m.FolderUnreadable(), false)
				case len(v.folderItems) == 0:
					v.folderNote(c, m.EmptyFolder(), false)
				default:
					ui.List(c, &v.folderList, len(v.folderItems), func(i int) {
						v.folderRow(c, i)
					}).Height(listHeight).KeepScrollbars(len(v.folderItems) > rows)
				}
			})
		})
	})
}

// folderHeader is the panel's top row: the way up to the item's own folder,
// the folder it lists, with the item's folder icon at the root, and the way to
// open it in the file manager.
func (v *view) folderHeader(c *ui.Context) {
	p := v.pal
	m := v.msgs()
	row := ui.Row(c).Height(32).Gap(2).AlignItems(ui.Center)
	root := v.folderDir == v.folderRoot
	item := v.app.item(v.folderGUID)
	name := filepath.Base(v.folderDir)
	if root {
		name = ""
		if item != nil {
			name = item.Name
		}
	}
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = v.folderDir
	}
	row.Children(func() {
		if !root {
			up := iconButton(c, iconArrowLeft, 15, p.muted, m.ParentFolder())
			up.Tooltip(m.ParentFolder())
			if up.Clicked() {
				dir := filepath.Dir(v.folderDir)
				v.later(func() { v.loadFolder(dir) })
			}
		}
		if root {
			v.folderTitleIcon(c, item)
		}
		ui.Text(c, name).FontSize(13).FontWeight(600).SingleLine().
			TextColor(p.text).Grow(1).Shrink(1).MinWidth(0).Padding(2, 0)
		open := iconButton(c, iconArrowUpRight, 15, p.muted, m.OpenFolderInExplorer())
		open.Tooltip(m.OpenFolderInExplorer())
		if open.Clicked() {
			v.later(v.openFolderInExplorer)
		}
	})
}

// folderTitleIcon is the icon beside the root title: the folder item's own
// icon, as its cell shows it, or the folder the window draws when it has none.
// Its margins keep it off the panel's edge and apart from the title, where the
// back button of a folder below the root leaves the title.
func (v *view) folderTitleIcon(c *ui.Context, item *AppItem) {
	if src := v.app.iconImage(item); src != nil {
		ui.Image(c, src).Size(18, 18).Fit(ui.Contain).MarginStart(6).MarginEnd(4)
		return
	}
	ui.Icon(c, iconFolder).Size(16, 16).TextColor(v.pal.muted).MarginStart(6).MarginEnd(4)
}

// folderNote is a line of the panel in place of its list: the folder is being
// read, holds nothing, or cannot be read.
func (v *view) folderNote(c *ui.Context, text string, spinner bool) {
	p := v.pal
	row := ui.Row(c).Height(folderRowHeight).Padding(0, 8).Gap(8).AlignItems(ui.Center)
	row.Children(func() {
		if spinner {
			ui.Spinner(c).Size(12, 12)
		}
		ui.Text(c, text).FontSize(12.5).TextColor(p.muted).SingleLine()
	})
}

// folderRow is one entry: its icon and name, a chevron for a folder it enters,
// and the whole row is the target.
func (v *view) folderRow(c *ui.Context, i int) {
	p := v.pal
	entry := v.folderItems[i]
	row := ui.Row(c).Height(folderRowHeight).Padding(0, 8).Gap(10).Radius(8).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(p.hover)
	}
	if row.Clicked() {
		// Reading the folder or opening the file happens between frames, so
		// the panel is not changed while its rows build.
		v.later(func() { v.chooseFolderEntry(entry) })
	}
	if row.RightClicked() && canUseShellActions(entry.path) {
		// The shell's own menu, as in Explorer. It blocks until closed; the
		// panel then reads the folder again, for a rename or a delete.
		path, dir := entry.path, v.folderDir
		v.later(func() {
			v.app.dispatch(func() {
				v.app.notifyErr(showEntryMenu(path))
				if dir != "" && v.folderGUID != "" {
					v.loadFolder(dir)
				}
			})
		})
	}
	row.Children(func() {
		v.folderEntryIcon(c, entry)
		ui.Text(c, entry.name).FontSize(13).TextColor(p.text).SingleLine().
			Grow(1).Shrink(1).MinWidth(0)
		if entry.dir {
			ui.Icon(c, iconChevronRight).Size(13, 13).TextColor(p.faint)
		}
	})
}

// chooseFolderEntry takes a row of the panel: a folder is entered, a file opens
// with the system's handler, and the panel goes either way.
func (v *view) chooseFolderEntry(entry folderEntry) {
	if entry.dir {
		v.loadFolder(entry.path)
		return
	}
	v.closeFolderMenu()
	if err := openEntryFile(entry.path); err != nil {
		v.app.notify(err.Error())
		return
	}
	if v.app.store.Settings.AutoHide {
		v.app.minimize()
	}
}

// openFolderInExplorer opens the folder the panel lists in the file manager.
func (v *view) openFolderInExplorer() {
	dir := v.folderDir
	v.closeFolderMenu()
	if err := openPath(dir); err != nil {
		v.app.notify(err.Error())
	}
}

// folderEntryIcon shows an entry's icon: its shell icon, read a screenful at a
// time, or the folder and file icons the window draws while it is on its way.
func (v *view) folderEntryIcon(c *ui.Context, entry folderEntry) {
	p := v.pal
	if src := v.folderIcons[entry.path]; src != nil {
		ui.Image(c, src).Size(20, 20).Fit(ui.Contain)
		return
	}
	v.ensureFolderIcon(entry.path)
	icon := iconFile
	if entry.dir {
		icon = iconFolder
	}
	ui.Icon(c, icon).Size(18, 18).TextColor(p.muted)
}

// ensureFolderIcon asks for the shell icon of an entry the list shows, on a
// goroutine: only the rows in view ask, so a folder of a thousand files reads a
// screenful's icons at a time. Without a window, as in tests, the shell calls
// behind the icons would wait for a main thread that never runs: the drawn
// folder and file icons stand in.
func (v *view) ensureFolderIcon(path string) {
	if v.app.win == nil || v.folderIcons == nil || v.folderIconWanted == nil {
		return
	}
	if _, ok := v.folderIcons[path]; ok || v.folderIconWanted[path] {
		return
	}
	v.folderIconWanted[path] = true
	v.folderIconQueue = append(v.folderIconQueue, path)
	v.startFolderIcons()
}

// startFolderIcons reads the icons asked for so far, a batch at a time, and
// shows them as they come back.
func (v *view) startFolderIcons() {
	if v.folderIconBusy || len(v.folderIconQueue) == 0 {
		return
	}
	v.folderIconBusy = true
	paths := v.folderIconQueue
	v.folderIconQueue = nil
	go func() {
		icons := make(map[string]ui.ImageSource, len(paths))
		for _, path := range paths {
			data, err := mygo.Shell.FileIcon(path, iconPixelsFor(20))
			if err != nil || len(data) == 0 {
				continue
			}
			if src, err := ui.DecodeImage(data); err == nil {
				icons[path] = src
			}
		}
		v.app.dispatch(func() {
			v.folderIconBusy = false
			if v.folderGUID == "" {
				return
			}
			if v.folderIcons == nil {
				v.folderIcons = map[string]ui.ImageSource{}
			}
			if v.folderIconWanted == nil {
				v.folderIconWanted = map[string]bool{}
			}
			for path, src := range icons {
				v.folderIcons[path] = src
			}
			v.refresh()
			v.startFolderIcons()
		})
	}()
}
