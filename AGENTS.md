# AGENTS.md

Guidance for agents working in this repository. Keep it short and true; update it when
conventions change.

## What this is

A minimal desktop launcher for Windows, written in Go with [MyGo](https://github.com/egoist/mygo)'s
**native UI** (`github.com/egoist/mygo/ui`) — the window is drawn from Go, with no web view, no
HTML and no frontend build. `_other.go` stubs keep the module compiling on Linux and macOS. The
launcher lists files/programs with icons and accumulated runtime, and can run/stop them.

## Toolchain

- Go 1.27+ (`go.mod` targets `go 1.27.3`; the native UI needs it)
- MyGo: `go.mod` requires `github.com/egoist/mygo` **replaced by `../mygo`**, a checkout with the
  `ui` package this app uses. Building from a clone without that checkout fails — either clone MyGo
  beside the repo or drop the `replace`. CI clones it at the pinned commit (see `release.yml`).
- The MyGo CLI is a Go tool: `go tool mygo` (the `tool` directive in `go.mod`).
- No C compiler, no bun/npm, no WebView2.

## Commands

```sh
go tool mygo dev               # dev with live reload
go tool mygo build             # build/windows-amd64/Go Launcher.exe + NSIS installer
go build ./...                 # compile only
go test ./...                  # tests, the interface's included (headless)
go vet ./...
GOOS=linux go build ./...      # the _other.go stubs must keep compiling
```

`mygo.json` holds the app name, identifier, version and output folder; `resources/icon.png` is the
app icon (see `docs/ICON.md`).

## How the UI works

**The view is rebuilt from state on every frame.** `view.view(c *ui.Context)` in `view.go` reads
the `App`'s store and the view's own fields and builds the interface; nothing is retained between
frames except those fields. Consequences that bite:

- **An action taken between frames must ask for a frame.** Click and menu handlers run outside the
  build, so a handler that changes what the window shows calls `v.refresh()` (`later` and
  `savePrefs` do it for you, and `App.save`/`notify` redraw too). Without it the change only shows
  at the next unrelated redraw.
- **Never change a collection while the loop that builds it runs.** Actions that add, remove,
  reorder or open a dialog go through `v.later(fn)`, which queues them for the next build pass and
  asks for a frame; `view()` drains the queue first and requests one more frame when it ran
  anything (`drain` reports that).
- **`mygo.Theme.SetSource` must not be called during a frame.** It posts to the main thread and
  deadlocks a headless `ui.Tester`. `applyTheme` runs from `main.go` at startup and from the theme
  button, never from `view()`.
- **System dialogs block**: `mygo.Dialog.Open` runs on a goroutine and its result goes back through
  `app.dispatch` (`win.Update`), which runs inline when there is no window (tests).
- `app.win` is nil until `WhenReady` and in tests. Window actions go through `app.dispatch`,
  `app.refresh`, `app.minimize`, `app.restore`, which all handle that.
- Icons: `mygo.Shell.FileIcon(path, px)` writes a PNG into the icon cache; `ui.DecodeImage` loads it
  back as a `ui.ImageSource`. There is no hand-written Win32 icon extraction any more.

## Data

- Everything lives in `.go-launcher-data/` beside the working directory: `go-launcher-data.json`
  (the store: `apps` / `categories` / `settings`), `ui-state.json` (theme, view mode, active tab,
  timers), `window-state.json` (geometry), `cached-icons/`.
- **The on-disk format is shared with the previous Wails build** (`run_as_admin`, `runtime_ms`,
  `null` slots in `categories[].slots`, the same file names). Do not rename keys without a
  migration. Window geometry uses MyGo's `WindowState`, loaded through `windowStateStore`, which
  still reads the old single-window file.
- Paths in the store are relative to the working directory unless the "Abs path" setting makes them
  absolute; `convertItemPaths` converts between the two.
- Every store write goes through `App.save` → `writeStoreAtomic`, which also records the file
  fingerprint, so the watcher (`store_watch.go`, `mygo.WatchFiles`) can tell the app's own writes
  from an edit made by hand. A file caught mid-edit (invalid JSON) is ignored, not treated as an
  empty store.

## Testing the interface

`view_test.go` drives the real view headlessly with `ui.NewTester`:

- **Call `tt.SetFocused(true)` first**: the first click on an unfocused window only focuses it.
- **Popover menus are drawn in the window**, so click their entry like any element
  (`tt.Click("Edit")` after opening the menu). `Tester.Menu()`/`ChooseMenuItem` are for *native*
  system menus (`mygo.NewMenu(...).Popup`), not for `PopoverMenu`/`PopoverMenuButton`.
- Actions queued with `later` need a frame: the tester builds one while the view asks for it
  (`refresh`), so a click through `tt.Click` already settles it.
- Text inputs keep their value, so `tt.Type` appends to what the field holds.

## Layout

| File | Role |
| --- | --- |
| `main.go` | single-instance guard, MyGo `WindowOptions`, `ui.View(v.view)`, file drop, watcher |
| `app.go` | `App`: store, running processes, launching/stopping, icons, toasts, `dispatch` |
| `store.go` | data file: `AppItem` / `CategoryNode` / `Settings` / `AppStore`, `parseStore`, `loadStore`, `writeStoreAtomic`, path helpers |
| `store_watch.go` | watch the store file, fingerprint, reload (`storeReloadApplied` / `Invalid`) |
| `view.go` | the frame: header, body, busy line, `later`/`drain`/`refresh`, theme |
| `view_tabs.go` / `view_list.go` / `view_grid.go` | tab strip, list rows, grid cells |
| `view_dialogs.go` / `view_search.go` | item, runtime, rename, help, confirm dialogs; search overlay |
| `items.go` / `commands.go` | tab and item operations; what the menus and buttons do |
| `icons.go` / `icons_svg.go` | item icons and the SVGs the window draws |
| `ui_state.go` / `window_state_store.go` | `ui-state.json` prefs; `window-state.json` via `WindowStateStore` |
| `launch_*.go` / `lnk_*.go` / `singleton_*.go` | process launch + elevation, `.lnk` parsing (COM), per-install-dir single instance |
| `utils.go` | `formatRuntime`, path normalization, `splitArgs`, `isExecutable` |
| `version.go` | `appVersion`, kept in step with `mygo.json` |

Frontend-free: there is no `frontend/`, no `wails.json`, no generated bindings. Adding an action
means adding a method on `view`/`App` and calling it from the menu or button that needs it.
