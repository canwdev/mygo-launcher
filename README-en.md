# MyGo Launcher

A portable Windows desktop launcher written in Go with the native UI of [MyGo](https://github.com/egoist/mygo) — no WebView.

[简体中文](README.md) | **English**

<p align="center">
  <img src="docs/screenshot.webp" alt="MyGo Launcher main window and settings window" width="960" />
</p>

## Installation

Supported today: **Windows 10/11 x64 and arm64**. 32-bit Windows and Windows 7/8 are not supported.

Download the installer or portable build for your architecture from [Releases](https://github.com/canwdev/mygo-launcher/releases/latest).

## Features

- **Pages**: group frequently used programs, files, and folders into pages; pages can be created, renamed, duplicated, removed, and reordered by dragging
- **Grid**: icon size in three steps (small, medium, large), with blank cells to control placement; item names wrap automatically, up to two lines
- **Launching**: single click launches an item, and running programs can be stopped; a folder item shows its contents instead, and its folders can be entered one level at a time; `.lnk` shortcuts launch with their saved target and arguments, and `.url` files are added as the page they name; items can run as administrator, prompting Windows UAC on launch
- **Adding**: drop files from Explorer to add them; the `+` button adds running windows, files, or hand-made items, and can also add common locations such as the Recycle Bin, This PC, Control Panel, your user folder, and the network
- **Icons**: extracted automatically for programs, files, and shortcuts; dropping in an image or `.ico` shows the image itself, and you can pick a specific icon from an image, `.ico`, `.exe`, or `.dll`, or choose one in the Windows icon picker
- **Arranging items**: reorder by dragging in the grid; hovering over a page tab switches page, so you can keep dropping at the desired position (hold Ctrl/Cmd to copy)
- **Timer**: off by default; when enabled, runtime is accumulated automatically, and clicking the duration adjusts it by ±1 hour, ±30 minutes, or minutes. When disabled, all timing stops at once and the same program can run again. Folders, files, and URLs are never timed
- **Search and keyboard**: `Ctrl/Cmd+F` searches and Enter launches; arrows choose items and turn pages, letters choose items in grid order; PgUp/PgDn, Tab, Home/End, and 1–9 switch pages; Esc hides or minimizes; `Ctrl+,` opens settings; icon button tooltips show their keys
- **Global hotkey**: recordable (function keys can be used alone) to show or hide the launcher from any window; a window called for by the hotkey, a second launch, or the tray appears centered on the pointer
- **Tray**: closing the window keeps it running in the system tray; settings can make close quit instead; “Hide from taskbar” removes the taskbar button and disables minimize
- **Appearance**: minimize after launching an item, always on top, light / dark / follow system, custom accent color; replaceable program icon and main window background picture (with adjustable opacity), with window and tray icons following the theme
- **Wheel**: switches pages by default, and Shift+wheel scrolls the grid; settings can invert this
- **Right click**: Properties and the native shell menu for files and folders; the Recycle Bin can be emptied from its menu and its icon follows its state
- **Language**: Simplified Chinese and English, chosen on first launch from the Windows display language
- **Environment**: launched programs use the system's current environment variables, so variables changed while the launcher runs take effect immediately
- **Migration**: import pages from `CLaunch.ini`, keeping button names, paths, arguments, and icons wherever possible

The portable build keeps its data in `mygo-launcher-data/` next to the executable; the installed build keeps it in
`%LOCALAPPDATA%\MyGo Launcher\mygo-launcher-data\`. Old data is migrated on upgrade, and uninstalling asks whether
to delete it. Manual edits to `go-launcher-data.json` take effect immediately.

## Command line

```bash
"MyGo Launcher.exe" --silent      # start without showing the main window
"MyGo Launcher.exe" --at-cursor   # show the main window at the pointer
```

When both are passed, silent wins. If the app is already running, launching it again acts like
`--at-cursor`: the existing window moves to the pointer and takes focus. With `--silent` the second
launch just exits and leaves the windows alone.

## Building

Requires Go 1.27+. `go.mod` uses `replace` pointing at a local `../mygo` checkout; you can remove the `replace` to use the published module instead.

```bash
go tool mygo dev     # run in dev mode, restarting on code changes
go tool mygo build   # package into build/, including the NSIS installer
go test ./...        # run tests (including headless UI tests)
```

Release, update signing, and installer details are in [docs/UPDATES.md](docs/UPDATES.md).

## Credits

- [MyGo](https://github.com/egoist/mygo): native UI and app framework
- [MyGo Music](https://github.com/egoist/mygo-cloudmusic): reference for the UI design and overall style
- [MyGO!!!!!](https://bang-dream.com/artist/mygo/): this project is not affiliated with the official group
- [CLaunch](https://ss1.xrea.com/pyonkichi.g1.xrea.com/en/): style reference

## License

[MIT](LICENSE)

Free and open source software, provided without any warranty.
