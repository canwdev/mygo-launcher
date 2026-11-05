---
description:
alwaysApply: true
---

# MyGo Launcher

## Overview

A portable desktop launcher for Windows, written in Go with [MyGo](https://github.com/egoist/mygo)'s
native UI: the window is drawn from Go, with no web view, no HTML and no frontend build. It lists
files, programs and folders with their icons and accumulated runtime, runs and stops them, and
keeps them in tabs. `README.md` is the project's main document.

## Language

Write new code comments and repository documents (`README.md`, `CHANGELOG.md`, `AGENTS.md`) in
English. `docs/ICON.md` and the comments in `docs/generate-icon.ps1` are Chinese, written for the
icon workflow.

## Toolchain

- Go 1.27+ (`go.mod` targets `go 1.27.3`); the native UI needs it.
- MyGo is the `github.com/egoist/mygo` module, and `go.mod` **replaces it with `../mygo`**, a
  checkout that carries the `ui` package. A clone without that checkout does not build: clone MyGo
  beside this repository or drop the `replace`. CI clones it at the commit in `release.yml`.
- The MyGo CLI comes from the `tool` directive in `go.mod`: `go tool mygo`.
- No C compiler, no bun/npm, no WebView2.

Commands:

```sh
go tool mygo dev               # develop with live reload
go tool mygo build             # build/windows-amd64/MyGo Launcher.exe + NSIS installer
go build ./...                 # compile only
go test ./...                  # tests, the interface's included (headless)
go vet ./...
GOOS=linux go build ./...      # the _other.go stubs must keep compiling
```

`mygo.json` holds the app name, identifier, version and output folder; `resources/icon.png` is the
app icon (see `docs/ICON.md`).

## The view

**The window is rebuilt from state on every frame.** `view.view(c *ui.Context)` in `view.go` reads
the `App`'s store and the view's own fields and builds the interface; nothing is retained between
frames except those fields. Rules that follow from it:

- **An action taken between frames must ask for a frame.** Click and menu handlers run outside the
  build, so a handler that changes what the window shows calls `v.refresh()` (`later` and
  `savePrefs` do it already, as do `App.save` and `App.notify`). Without it the change shows only at
  the next unrelated redraw.
- **Never change a collection while the loop that builds it runs.** Adding, removing, reordering and
  opening a dialog go through `v.later(fn)`, which queues them for the next build pass and asks for
  a frame; `view()` drains the queue first and asks for one more frame when it ran anything.
- **`mygo.Theme.SetSource` must not run during a frame**: it posts to the main thread and deadlocks
  a headless `ui.Tester`. `applyTheme` runs from `main.go` at startup and from the theme button.
- **System dialogs block**: `mygo.Dialog.Open` runs on a goroutine and its result comes back through
  `app.dispatch` (`Window.Update`), which runs inline when there is no window, as in tests.
- `app.win` is nil until `WhenReady`, and in tests. Window actions go through `app.dispatch`,
  `app.refresh`, `app.minimize` and `app.restore`, which handle that.

### Drag and drop

- `Drag(value)` on a row, a cell or a tab starts a drag once the pointer moves a few DIPs;
  `ui.Drop[T]` and `ui.DragOver[T]` on another element take it.
- **An element takes one type of dragged value**: `Drop`/`DragOver` keep a single `accepts` closure
  on the element's state, so the last call wins. That is why a tab's chip takes `dragItem` (items
  dropped on it) while the wrapper around it takes `dragTab` (tabs reordered onto it) — do not put
  both types on one element.
- Values are matched by type: items drag as `dragItem`, tabs as `dragTab` (`items.go`). Reordering a
  tab's slots goes through `reorderSlots`, which works on slot indices, empty cells included.

### Icons

`mygo.Shell.FileIcon(path, px)` writes a PNG into the icon cache and `ui.DecodeImage` loads it back
as a `ui.ImageSource`; nothing extracts icons by hand. An item whose icon points into
`mygo-launcher-data/cached-icons/` counts as auto-generated (`isAutoIcon`), which is what enables
*Update icon* and the batch update.

## Data

- Everything lives in `mygo-launcher-data/` beside the working directory:
  - `go-launcher-data.json` — the store: `apps`, `categories` (tabs whose `slots` hold app guids,
    `null` for an empty cell) and `settings` (`auto_hide` / `always_on_top` / `absolute_paths`)
  - `ui-state.json` — the window's own settings: theme, list or grid, active tab
  - `window-state.json` — geometry, through MyGo's `WindowStateStore`
  - `cached-icons/` — the icons extracted from the files items point at
- The store is written only through `App.save` → `writeStoreAtomic`, which records the file's
  fingerprint (mtime, size, content hash) so the watcher (`store_watch.go`, `mygo.WatchFiles`) can
  tell the app's own writes from an edit made by hand. A file caught mid-edit (invalid JSON) is
  ignored, not taken as an empty store.
- Paths in the store are relative to the working directory unless the *Abs path* setting makes them
  absolute; `convertItemPaths` converts them, so keep that round trip working.
- An executable is tracked while it runs (`startTracked`), and its time is added to `runtimeStats`
  when it exits; the runtime dialog overrides the stored value (`setRuntimeMs`) by minutes or by
  ±1 hour.

## Testing the interface

`view_test.go` drives the real view headlessly with `ui.NewTester`:

- **Call `tt.SetFocused(true)` first**: the first click on an unfocused window only focuses it.
- **Popover menus are drawn in the window**, so click their entry like any other element
  (`tt.Click("Edit")` after opening the menu). `Tester.Menu()`/`ChooseMenuItem` are for *native*
  system menus (`mygo.NewMenu(...).Popup`), not for `PopoverMenu`/`PopoverMenuButton`.
- Drag with `Press`, `Move` and `Release` (the `drag` helper in `view_test.go`); a press that moves
  only a couple of DIPs is still a click.
- Text inputs keep their value, so `tt.Type` appends to what the field holds.
- Actions queued with `later` need a frame; the tester builds one while the view asks for it, so a
  click through `tt.Click` settles them.

Tests must not touch the real data folder: `newTestApp` points `absBase` at a temporary directory.

## Changelog

`CHANGELOG.md` lives at the repository root and is **minimal**: only record changes users can
perceive, one item per thing.

- Newest version first. Add the item under the latest version heading.
- Groups appear only as needed, with no empty headings: `### Breaking` / `### UI` / `### Features` /
  `### Fixes` / `### Engineering`. `Engineering` only holds things that affect the development
  workflow (build, packaging, tests); internal refactors are not recorded.
- Use one sentence to say what change the user sees; do not list files or identifiers — that is git
  log's job.
- Start every list item with a short bold label, then a colon, then the sentence:
  `- **Runtime**: ...`, `- **Tabs**: ...`.
- The version must match in two places, changed together at release: `version` in `mygo.json` and
  `appVersion` in `version.go`.

## Git

Never add a `Co-authored-by` trailer to a commit message. The commit is the repository author's
alone.

## Layout

| File | Role |
| --- | --- |
| `main.go` | single-instance guard, MyGo `WindowOptions`, `ui.View(v.view)`, file drop, store watch |
| `app.go` | `App`: store, running processes, launching/stopping, icons, toasts, `dispatch` |
| `store.go` | the data file: `AppItem` / `CategoryNode` / `Settings` / `AppStore`, `parseStore`, `loadStore`, `writeStoreAtomic`, path helpers |
| `store_watch.go` | watch the store file, fingerprint, reload (`storeReloadApplied` / `Invalid`) |
| `view.go` | the frame: header, body, busy line, `later`/`drain`/`refresh`, theme, app menu |
| `view_tabs.go` | the tab strip: chips, their menus, item drops and tab reordering |
| `view_list.go` / `view_grid.go` | the two layouts, rows and cells, their drag and drop |
| `view_dialogs.go` | item, runtime, rename and confirm dialogs |
| `view_search.go` | the search overlay: filter, keyboard, locate in tab |
| `items.go` / `commands.go` | tab and item operations; what the menus and buttons do |
| `icons.go` / `icons_svg.go` | item icons and the SVGs the window draws |
| `ui_state.go` / `window_state_store.go` | `ui-state.json` preferences; `window-state.json` |
| `launch_*.go` / `lnk_*.go` / `singleton_*.go` | process launch and elevation, `.lnk` parsing (COM), the per-install-directory single instance |
| `utils.go` | `formatRuntime`, path normalization, `splitArgs`, `isExecutable` |
| `version.go` | `appVersion`, kept in step with `mygo.json` |

There is no frontend. Adding an action means adding a method on `view` or `App` and calling it from
the menu or button that needs it.
