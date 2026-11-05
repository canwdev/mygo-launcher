# Changelog

The version number lives in `mygo.json` (`version`) and `version.go` (`appVersion`); change both together at release.

## 0.2.0

### Breaking

- **Name**: the app is now MyGo Launcher, so its executable and installer are `MyGo Launcher.exe` and `mygo-launcher-setup-<version>-amd64.exe`.
- **Data folder**: data lives in `mygo-launcher-data/` beside the executable, and the `.go-launcher-data/` folder of earlier builds is not read; rename it to keep the entries.
- **Timers**: the manual and auto timers are gone. An executable is timed whenever it runs, and the saved runtime is corrected by ±1 hour or by typing minutes. *Auto timing* in the window menu turns the clock off.
- **Game mode**: the toggle is gone; an executable is tracked, and can be stopped, every time.

### UI

- **Native window**: the interface is drawn from Go by MyGo's native UI, with no web view and no WebView2 runtime.
- **Drag and drop**: items move between slots in both views, can be dropped on a tab to move them there (Ctrl/Cmd copies), and tabs reorder by dragging.
- **Runtime**: an item's accumulated time sits beside its name, or in a cell's corner in the grid, and clicking it edits it.
- **Search**: Cmd/Ctrl+F opens the search overlay; Enter runs the chosen item, and the crosshair jumps to it in its tab.
- **Tabs**: add, rename, duplicate, delete and drag to reorder, with the same menu on a right click.
- **Two layouts**: a list and a grid, remembered across restarts, with empty cells in the grid as places to put items.

### Features

- **Items**: edit, duplicate, insert an empty slot, open the containing folder, rename, delete, update the icon, and convert paths between absolute and relative.
- **Administrator**: *Run as administrator* in the item editor launches the item elevated, and the row shows a shield.
- **Data file**: still `go-launcher-data.json` with the same keys, watched while the app runs, so an edit by hand applies at once and invalid JSON is ignored instead of wiping the store.
- **Window**: size, position and maximized state are kept in `mygo-launcher-data/window-state.json`; *Always on top* and *Auto-hide* are in the window menu.
- **Portable**: everything the app keeps is in one folder beside the executable, and a second launch from the same folder focuses the window already open.

### Engineering

- **Build**: Wails, Vue, bun and the frontend build are gone; `go tool mygo build` produces the executable and the NSIS installer from a single Go module.
- **Tests**: the interface is driven headlessly (`ui.NewTester`), covering tabs, dialogs, search, menus and the three drag gestures.
