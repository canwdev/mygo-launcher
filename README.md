# MyGo Launcher

<p align="center">
  <img src="resources/icon.png" alt="icon" width="180" height="180" />
</p>

A minimal desktop launcher for Windows, written in Go with
[MyGo](https://github.com/egoist/mygo)'s native UI — no web view, no HTML, no
JavaScript. It shows a window listing files/programs (with icons and accumulated
runtime), lets you run/stop them, drag & drop to add more, and manage entries via
a per-item menu. Executables are timed automatically: the runtime counts while
the program runs and is saved when it exits (click it to correct it by ±1h or by
typing minutes). *Auto timing* in the window menu turns that clock off, leaving
the runtime to be edited by hand. Each item can be launched as administrator
(check *Run as administrator* in the item editor — it is stored in the data file
and shows a shield marker; Windows asks for UAC). Launcher data lives in
`mygo-launcher-data/go-launcher-data.json`; edit that file by hand and the running
app reloads it automatically (see [Data file](#data-file)).

## Requirements

- Go 1.27 or later (MyGo's native UI needs it)
- MyGo as the module `github.com/egoist/mygo`. `go.mod` points at a local checkout
  with `replace github.com/egoist/mygo => ../mygo`; drop that `replace` to build
  against the published module instead.
- The MyGo CLI, as a Go tool: `go tool mygo` (pinned in `go.mod`).
- **Windows**: nothing else — no WebView2, no C compiler, no bun/npm.

## Run (development)

```sh
go tool mygo dev
```

This rebuilds and restarts the app when the Go code, the config or the icon
changes.

## Build

```sh
go tool mygo build          # build/windows-amd64/MyGo Launcher.exe + installer
go build ./...              # compile only, for a quick check
go test ./...               # tests, the interface's included (headless)
```

## Releases

Every push to `master` runs [`.github/workflows/release.yml`](./.github/workflows/release.yml)
on `windows-latest`: it checks out MyGo at the pinned commit beside the repo (for
the `replace` directive), runs `go tool mygo build`, then publishes a GitHub
Release tagged `v<version>-<run number>` (e.g. `v0.2.0-42`) with the executable
and the installer attached (Windows x64 only). Bump `version` in `mygo.json` and
`appVersion` in `version.go` when you want a new release version. Manual runs are
possible from the Actions tab (`workflow_dispatch`).

## Data file

Everything the launcher knows is kept in `mygo-launcher-data/go-launcher-data.json`
(next to the working directory of the executable):

- `apps` — global item pool (`guid` / `name` / `path` / `icon` / `args` / `working_dir` / `runtime_ms` / `run_as_admin`)
- `categories` — the tabs, each with a `slots` array of app guids (`null` = empty grid cell)
- `settings` — `auto_time` / `auto_hide` / `always_on_top` / `absolute_paths`

The file is watched while the app runs (`mygo.WatchFiles`, event-driven — no
polling): save a manual edit and the launcher applies it right away and refreshes
the window. A file that is not valid JSON is ignored (the current data stays
loaded) and a warning is shown; deleting the file is also ignored, and removing an
item stops tracking it. `Refresh` in the app menu re-reads the file immediately,
and the store is written back on every change.

The window keeps its own settings in the same folder, because they belong to the
window rather than to the launcher's data:

- `mygo-launcher-data/ui-state.json` — theme, list/grid view and active tab (what the
  previous build kept in the page's localStorage)
- `mygo-launcher-data/window-state.json` — window position, size and maximized state
- `mygo-launcher-data/cached-icons/` — icons extracted from the files items point at

## Structure

- `main.go` — MyGo bootstrap: single-instance guard, the window with the native
  UI, file drop, store watch
- `app.go` — the `App` struct: the store, the running processes, launching and
  stopping, icons, toasts, and the main-thread plumbing (`dispatch`)
- `store.go` — the data file (`AppStore`, `parseStore`, `loadStore`, `writeStoreAtomic`)
- `store_watch.go` — watch `go-launcher-data.json` for manual (external) edits and
  reload the store
- `view*.go` — the native UI, one file per area: `view.go` (window, header, theme),
  `view_tabs.go` (tab strip), `view_list.go` / `view_grid.go` (the two layouts),
  `view_dialogs.go` (item/runtime/rename/confirm dialogs), `view_search.go`
  (search overlay), with `items.go` and `commands.go` for what the UI does
- `icons.go`, `icons_svg.go` — item icons (`mygo.Shell.FileIcon`) and the SVGs the
  window draws
- `ui_state.go`, `window_state_store.go` — the view's and the window's own settings
- `launch_*.go`, `lnk_*.go`, `singleton_*.go` — platform helpers (process launching
  and elevation, `.lnk` parsing, the per-install-directory single-instance guard)
- `*_test.go` — store, watcher and view tests; the view tests drive the real
  interface headlessly (`ui.NewTester`)

## References

- [Icon generation](./docs/ICON.md)
- [MyGo documentation](https://github.com/egoist/mygo/tree/main/docs)
- [MyGo native UI](https://github.com/egoist/mygo/tree/main/ui)
