package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/google/uuid"
	"go-launcher/locales"
)

// CLaunch keeps its pages in Data\CLaunch.ini, a UTF-16 INI whose section names
// repeat: every [Page###] is followed by [Btn000] and up, then the next page
// starts over at [Btn000]. This file reads that format and turns it into tabs
// of the launcher, best effort: what cannot be resolved (a binary shell ID
// list, an unknown special folder) is skipped and counted.

// The Type field of a CLaunch button: the low 16 bits are the kind, the high
// bits are options. The values are the ones its own data and documentation
// use.
const (
	cLaunchURL         = 0x0004 // a URL or a shell target, as written
	cLaunchSpecial     = 0x0006 // a special folder, given as a binary ID list
	cLaunchStoreApp    = 0x0007 // a packaged app, given as an AppUserModelID
	cLaunchSubmenu     = 0x0010 // a submenu: its items take its place
	cLaunchPowerOff    = 0x1000 // shut down
	cLaunchPowerReboot = 0x1001 // restart
	cLaunchCSIDL       = 0x2000 // a known folder: 0x2000 | CSIDL
	cLaunchExitApp     = 0x4003 // quits CLaunch, which the launcher has no use for
	// cLaunchRunAsAdmin is an option bit of the file kind: the button's "Privilege
	// Level" asks for elevation.
	cLaunchRunAsAdmin = 0x00400000
)

// cLaunchFolders maps the CSIDL CLaunch stores in a known-folder button to
// something the launcher can open. Filesystem folders are their path, the shell's
// virtual folders a shell namespace name; %VAR% is expanded when the item is
// built.
var cLaunchFolders = map[int]string{
	0x00: `%USERPROFILE%\Desktop`,
	0x02: `%APPDATA%\Microsoft\Windows\Start Menu\Programs`,
	0x03: `::{26EE0668-A00A-44D7-9371-BEB064C98683}`, // Control Panel
	0x05: `%USERPROFILE%\Documents`,
	0x06: `%USERPROFILE%\Favorites`,
	0x07: `%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup`,
	0x0a: `::{645FF040-5081-101B-9F08-00AA002F954E}`, // Recycle Bin
	0x0b: `%APPDATA%\Microsoft\Windows\Start Menu`,
	0x11: `::{20D04FE0-3AEA-1069-A2D8-08002B30309D}`, // This PC
	0x12: `::{208D2C60-3AEA-1069-A2D7-08002B30309D}`, // Network
	0x13: `::{208D2C60-3AEA-1069-A2D7-08002B30309D}`,
	0x1a: `%APPDATA%`,
	0x1c: `%LOCALAPPDATA%`,
	0x24: `%SystemRoot%`,
	0x25: `%SystemRoot%\System32`,
	0x26: `%ProgramFiles%`,
	0x27: `%USERPROFILE%\Pictures`,
	0x28: `%USERPROFILE%`,
	0x2a: `%ProgramFiles(x86)%`,
}

// cLaunchIni is an INI read as an ordered list of sections. Names repeat, so a
// map keyed by name would lose the pages.
type cLaunchIni struct {
	sections []cLaunchSection
}

// cLaunchSection is one [section]: its name and the keys it holds, lowercased
// so lookups do not depend on CLaunch's capitalization.
type cLaunchSection struct {
	name string
	keys map[string]string
}

// get reads a key of the section, whose name the caller writes lowercase.
func (s cLaunchSection) get(key string) string { return s.keys[key] }

// cLaunchPage is a [Page###] and the buttons that follow it.
type cLaunchPage struct {
	Name    string
	Buttons []cLaunchButton
}

// cLaunchButton is a [Btn###]: one item of a page, or of a submenu.
type cLaunchButton struct {
	Type      uint32
	Name      string
	File      string
	Parameter string
	Directory string
	IconFile  string
	IconIndex int
	Position  int
	// Menu holds a submenu's buttons, which take the button's place in the tab.
	Menu []cLaunchButton
}

// cLaunchImport is what reading a CLaunch.ini produced: new tabs with their
// items, ready to be added to the store, and how many buttons could not be
// converted.
type cLaunchImport struct {
	Tabs    []cLaunchTab
	Skipped int
}

// cLaunchTab is one CLaunch page as a launcher tab.
type cLaunchTab struct {
	Name  string
	Items []*AppItem
}

// decodeIniText reads the text of an INI file: UTF-16 with a BOM (what Windows
// writes), UTF-16 without one, or UTF-8 as an editor may have saved it.
func decodeIniText(data []byte) string {
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE:
		return decodeUTF16(data[2:], binary.LittleEndian)
	case len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF:
		return decodeUTF16(data[2:], binary.BigEndian)
	}
	if looksUTF16(data) {
		return decodeUTF16(data, binary.LittleEndian)
	}
	return string(data)
}

func decodeUTF16(data []byte, order binary.ByteOrder) string {
	if len(data)%2 == 1 {
		data = data[:len(data)-1]
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = order.Uint16(data[2*i:])
	}
	return string(utf16.Decode(units))
}

// looksUTF16 reports whether data is UTF-16LE without a BOM: a NUL after most
// of the ASCII bytes, as CLaunch's first two bytes are "\r\0".
func looksUTF16(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	limit := min(len(data), 256)
	nuls, pairs := 0, 0
	for i := 1; i < limit; i += 2 {
		pairs++
		if data[i] == 0 {
			nuls++
		}
	}
	return pairs > 0 && nuls > pairs/2
}

// parseCLaunchIni reads an INI into ordered sections. Empty lines, comments and
// keys before the first section are ignored, and the first of a repeated key wins.
func parseCLaunchIni(text string) cLaunchIni {
	text = strings.TrimPrefix(text, "\ufeff")
	ini := cLaunchIni{}
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || line[0] == ';' || line[0] == '#' {
			continue
		}
		if line[0] == '[' {
			end := strings.IndexByte(line, ']')
			if end <= 0 {
				continue
			}
			ini.sections = append(ini.sections, cLaunchSection{
				name: strings.TrimSpace(line[1:end]),
				keys: map[string]string{},
			})
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 || len(ini.sections) == 0 {
			continue
		}
		sec := &ini.sections[len(ini.sections)-1]
		key := strings.ToLower(strings.TrimSpace(line[:eq]))
		if _, dup := sec.keys[key]; dup {
			continue
		}
		sec.keys[key] = strings.TrimSpace(line[eq+1:])
	}
	return ini
}

// numbered reads the number a section name ends with: "Btn017" → 17.
func numbered(name, prefix string) (int, bool) {
	if !strings.HasPrefix(name, prefix) || len(name) == len(prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(name[len(prefix):])
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// readCLaunchIni reads every [Page###] of an INI with the [Btn###] sections that
// follow it, then attaches every [SubMenu###] to the button its Page and Button
// keys name.
func readCLaunchIni(text string) []cLaunchPage {
	ini := parseCLaunchIni(text)
	pages := map[int]*cLaunchPage{}
	var order []int

	// buttonsFrom reads the [Btn###] sections at i and returns where they end.
	buttonsFrom := func(i int) ([]cLaunchButton, int) {
		var out []cLaunchButton
		for i < len(ini.sections) {
			if _, ok := numbered(ini.sections[i].name, "Btn"); !ok {
				break
			}
			out = append(out, readButton(ini.sections[i]))
			i++
		}
		return out, i
	}

	type submenu struct {
		page, button int
		buttons      []cLaunchButton
	}
	var submenus []submenu

	for i := 0; i < len(ini.sections); {
		name := ini.sections[i].name
		if _, ok := numbered(name, "SubMenu"); ok {
			keys := ini.sections[i].keys
			page, _ := strconv.Atoi(strings.TrimSpace(keys["page"]))
			button, _ := strconv.Atoi(strings.TrimSpace(keys["button"]))
			buttons, next := buttonsFrom(i + 1)
			submenus = append(submenus, submenu{page, button, buttons})
			i = next
			continue
		}
		if idx, ok := numbered(name, "Page"); ok {
			page := cLaunchPage{Name: ini.sections[i].get("name")}
			page.Buttons, i = buttonsFrom(i + 1)
			pages[idx] = &page
			order = append(order, idx)
			continue
		}
		i++
	}

	for _, sub := range submenus {
		page, ok := pages[sub.page]
		if !ok || len(sub.buttons) == 0 {
			continue
		}
		if sub.button >= 0 && sub.button < len(page.Buttons) {
			page.Buttons[sub.button].Menu = sub.buttons
			continue
		}
		// A reference that does not name a button any more: keep the items,
		// at the end of the page.
		page.Buttons = append(page.Buttons, cLaunchButton{Type: cLaunchSubmenu, Menu: sub.buttons})
	}

	out := make([]cLaunchPage, 0, len(order))
	for _, idx := range order {
		out = append(out, *pages[idx])
	}
	return out
}

// readButton reads a [Btn###] section.
func readButton(sec cLaunchSection) cLaunchButton {
	b := cLaunchButton{
		Type:      parseCLaunchHex(sec.get("type")),
		Name:      sec.get("name"),
		File:      sec.get("file"),
		Parameter: sec.get("parameter"),
		Directory: sec.get("directory"),
		IconFile:  sec.get("iconfile"),
	}
	b.IconIndex, _ = strconv.Atoi(sec.get("iconindex"))
	b.Position, _ = strconv.Atoi(sec.get("position"))
	return b
}

// parseCLaunchHex reads the hexadecimal numbers CLaunch writes without a 0x.
func parseCLaunchHex(s string) uint32 {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0
	}
	return uint32(v)
}

// importCLaunchFile reads a CLaunch.ini and turns its pages into tabs for the
// launcher. It reads the disk and extracts every icon, so it runs off the main
// thread; absolute is the "absolute paths for new items" setting.
func importCLaunchFile(path string, absolute, withIcon bool) (cLaunchImport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cLaunchImport{}, err
	}
	pages := readCLaunchIni(decodeIniText(data))
	return buildCLaunchImport(pages, cLaunchBase(path), absolute, withIcon), nil
}

// cLaunchBase is the folder CLaunch resolves relative paths against: the one
// holding CLaunch.exe. An ini in CLaunch's Data folder sits one level below it.
func cLaunchBase(iniPath string) string {
	dir := filepath.Dir(iniPath)
	for _, d := range []string{dir, filepath.Dir(dir)} {
		if info, err := os.Stat(filepath.Join(d, "CLaunch.exe")); err == nil && !info.IsDir() {
			return d
		}
	}
	if strings.EqualFold(filepath.Base(dir), "Data") {
		return filepath.Dir(dir)
	}
	return dir
}

// buildCLaunchImport turns the pages of an ini into tabs: one page, one tab,
// appended in the order CLaunch stores them. A page whose items were all skipped
// makes no tab.
func buildCLaunchImport(pages []cLaunchPage, base string, absolute, withIcon bool) cLaunchImport {
	imp := cLaunchImport{}
	for i, page := range pages {
		name := strings.TrimSpace(page.Name)
		if name == "" {
			name = locales.CLaunchTabName(strconv.Itoa(i + 1))
		}
		tab := cLaunchTab{Name: name}
		for _, b := range page.Buttons {
			imp.Skipped += cLaunchItems(b, base, absolute, withIcon, &tab.Items)
		}
		if len(tab.Items) > 0 {
			imp.Tabs = append(imp.Tabs, tab)
		}
	}
	return imp
}

// cLaunchItems appends the items a button stands for. A submenu's items take
// its place, one level down at a time; a button that cannot be converted is
// counted, not added.
func cLaunchItems(b cLaunchButton, base string, absolute, withIcon bool, out *[]*AppItem) int {
	if len(b.Menu) > 0 {
		skipped := 0
		for _, child := range b.Menu {
			skipped += cLaunchItems(child, base, absolute, withIcon, out)
		}
		return skipped
	}
	item, ok := cLaunchItem(b, base, absolute, withIcon)
	if !ok {
		return 1
	}
	*out = append(*out, item)
	return 0
}

// cLaunchItem makes an item of one button, or reports that it cannot be
// converted.
func cLaunchItem(b cLaunchButton, base string, absolute, withIcon bool) (*AppItem, bool) {
	target, args, ok := cLaunchTarget(b, base)
	if !ok {
		return nil, false
	}
	name := strings.TrimSpace(b.Name)
	if name == "" && target != "" && !isShellTarget(target) {
		name = defaultTitle(target)
	}
	if name == "" {
		name = locales.CLaunchItemName()
	}
	workDir := ""
	if b.Directory != "" {
		workDir = resolveCLaunchPath(b.Directory, base)
	}
	// A shortcut starts through its target, as an item added from the file
	// manager does, so it is tracked and timed too; the shortcut is kept for
	// its own icon.
	link := ""
	iconPath := cLaunchIconPath(b, target, base)
	if target != "" && !isShellTarget(target) && isShortcutFile(target) {
		if lnk, err := resolveShortcut(target); err == nil && lnk.Target != "" {
			link = storedForm(target, absolute)
			if args == "" {
				args = lnk.Args
			}
			if workDir == "" && lnk.WorkDir != "" {
				workDir = storedForm(lnk.WorkDir, absolute)
			}
			target, iconPath = lnk.Target, absPath(link)
		}
	}
	item := &AppItem{
		GUID:       uuid.NewString(),
		Name:       name,
		Path:       storedForm(target, absolute),
		WorkingDir: storedForm(workDir, absolute),
		Icon:       "",
		Args:       args,
		RunAsAdmin: b.Type&cLaunchRunAsAdmin != 0,
		Link:       link,
	}
	if withIcon {
		item.Icon = writeIcon(iconPath)
	}
	return item, true
}

// cLaunchTarget reads where a button points: a file or folder, a shell target,
// a known folder, a packaged app or a power action. Resources the launcher cannot
// name, such as a binary shell ID list, report false.
func cLaunchTarget(b cLaunchButton, base string) (string, string, bool) {
	kind := b.Type & 0xffff
	switch {
	case kind == cLaunchSubmenu:
		return "", "", false // its items took its place
	case kind == cLaunchExitApp:
		return "", "", false // quitting CLaunch means nothing here
	case kind >= cLaunchCSIDL && kind <= cLaunchCSIDL+0xff:
		folder, known := cLaunchFolders[int(kind-cLaunchCSIDL)]
		if !known {
			return "", "", false
		}
		return resolveCLaunchPath(folder, base), b.Parameter, true
	case kind == cLaunchPowerOff:
		return cLaunchShutdownExe(), "/s /t 0", true
	case kind == cLaunchPowerReboot:
		return cLaunchShutdownExe(), "/r /t 0", true
	case kind == cLaunchStoreApp:
		if strings.TrimSpace(b.File) == "" {
			return "", "", false
		}
		// MyGo opens a Start menu app by its shell item, which is also how its
		// icon is drawn.
		return "shell:AppsFolder\\" + strings.TrimSpace(b.File), b.Parameter, true
	case kind == cLaunchURL:
		target := strings.TrimSpace(expandCLaunchEnv(b.File))
		if target == "" {
			return "", "", false
		}
		return target, b.Parameter, true
	case kind == cLaunchSpecial:
		if isCLaunchIDList(b.File) {
			return "", "", false // a binary ID list names a folder we cannot resolve
		}
	}
	// A file, a folder, or a kind an older CLaunch wrote: use the File field
	// when it names something.
	file := strings.TrimSpace(expandCLaunchEnv(b.File))
	if file == "" || isCLaunchIDList(file) {
		return "", "", false
	}
	return resolveCLaunchPath(file, base), b.Parameter, true
}

// cLaunchShutdownExe is the system's shutdown program, for the power buttons.
func cLaunchShutdownExe() string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32", "shutdown.exe")
}

// cLaunchIconPath is where an item's icon is drawn from: the target itself, or
// the IconFile when it is an image of its own. CLaunch's IconIndex, which picks
// an icon inside a DLL, cannot be read again, so the target's icon is preferred.
func cLaunchIconPath(b cLaunchButton, target, base string) string {
	if b.IconFile != "" {
		icon := resolveCLaunchPath(b.IconFile, base)
		if target == "" || isImageFile(icon) {
			return icon
		}
	}
	return target
}

// isImageFile reports whether path is an image an item's icon may point at.
func isImageFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ico", ".png", ".jpg", ".jpeg", ".bmp", ".gif", ".svg", ".webp":
		return true
	}
	return false
}

// isCLaunchIDList reports whether a File field is a binary shell ID list CLaunch
// found by dropping a folder on its button, which it writes as hex. There is no
// path in it to open.
func isCLaunchIDList(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 16 || len(s)%2 != 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

// expandCLaunchEnv replaces the %VAR% CLaunch allows in paths with its value, as
// os.ExpandEnv does for $VAR.
func expandCLaunchEnv(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '%' {
			if end := strings.IndexByte(s[i+1:], '%'); end >= 0 {
				b.WriteString(os.Getenv(s[i+1 : i+1+end]))
				i += end + 2
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return os.ExpandEnv(b.String())
}

// resolveCLaunchPath expands a CLaunch path and resolves a relative one against
// base, the folder of CLaunch.exe. Shell targets are kept as they are.
func resolveCLaunchPath(p, base string) string {
	p = strings.TrimSpace(expandCLaunchEnv(p))
	if p == "" {
		return ""
	}
	if isShellTarget(p) {
		return p
	}
	p = normalizePath(p)
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(base, p))
}

// addCLaunchImport appends the tabs of an import to the store and returns the
// guid of the first new tab with how many tabs and items it added. It runs on
// the main thread, as the store does.
func (a *App) addCLaunchImport(imp cLaunchImport) (first string, tabs, items int) {
	for _, tab := range imp.Tabs {
		if len(tab.Items) == 0 {
			continue
		}
		slots := make([]*string, 0, len(tab.Items))
		for _, item := range tab.Items {
			if item.GUID == "" {
				item.GUID = uuid.NewString()
			}
			a.store.Apps[item.GUID] = item
			a.runtimeStats[item.GUID] = 0
			slots = append(slots, &item.GUID)
		}
		guid := uuid.NewString()
		a.store.Categories = append(a.store.Categories, CategoryNode{GUID: guid, Name: tab.Name, Slots: slots})
		if first == "" {
			first = guid
		}
		tabs++
		items += len(tab.Items)
	}
	if tabs > 0 {
		a.save()
	}
	return first, tabs, items
}
