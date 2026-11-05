# MyGo Launcher

<p align="center">
  <img src="resources/icon.png" alt="图标" width="180" height="180" />
</p>

一个面向 Windows 的便携式桌面启动器，用 Go 和
[MyGo](https://github.com/egoist/mygo) 的原生 UI 编写——没有 WebView、HTML 或 JavaScript。
界面参照 [MyGo Music](https://github.com/zhangnjsd/mygo-music)：顶部标签导航、统一的浅色/深色
配色、圆角行与按钮，设置放在独立窗口中。

它把文件、程序和文件夹连同图标与累计运行时长列在标签页里，可以启动、停止它们，拖放文件即可
添加（也可以点右下角的圆形 `+`），并通过右键菜单管理条目。可执行程序会自动计时：运行期间累计，
退出时写回（点击时长可按 ±1 小时或按分钟修正）；在设置里关掉「自动计时」后时钟不再走动，只能
手工编辑。条目可以以管理员身份启动（在条目编辑器里勾选 *以管理员身份运行*；该选项写入数据文件，
列表里显示盾牌标记；Windows 会弹出 UAC）。

顶栏齿轮打开设置，里面可以切换主题、选择强调色（预设取自网站 `--bang-1` 至 `--bang-5`，即
Tomori、Anon、Rāna、Soyo、Taki 五位成员的颜色，各有明暗两套、随主题自动切换，默认
`--bang-1`；另有自定义取色器）、自动计时、自动隐藏、始终置顶、路径转换、数据目录与图标维护。
列表/网格切换在顶栏、设置按钮左侧。

启动器数据保存在 `mygo-launcher-data/go-launcher-data.json`；手工修改该文件后运行中的应用会
自动重新加载（见[数据文件](#数据文件)）。

## 环境要求

- Go 1.27 或更高版本（MyGo 的原生 UI 需要）。
- MyGo 模块 `github.com/egoist/mygo`。`go.mod` 用 `replace` 指向本机 `../mygo` 检出；
  想改用发布版模块就删掉 `replace`。
- MyGo CLI：`go tool mygo`（由 `go.mod` 的 `tool` 指令固定）。
- **Windows**：不需要其他东西——没有 WebView2、C 编译器、bun/npm。

## 开发运行

```sh
go tool mygo dev
```

代码、配置或图标变化后会自动重新构建并重启。

## 构建

```sh
go tool mygo build                  # build/windows-amd64/MyGo Launcher.exe + NSIS 安装包
go tool mygo build -skip-installer  # 不生成安装包，快速验证
go build -o build ./src             # 只编译，产物在已忽略的 build/
go test ./...                       # 测试，含无头界面测试
go vet ./...
```

入口包在 `src/`，由 `mygo.json` 的 `"main": "./src"` 指定。注意不要直接跑 `go build ./...`，
它会在仓库根目录生成 `src.exe`。

## 发布

每次推送到 `master` 都会运行
[`.github/workflows/release.yml`](./.github/workflows/release.yml)：在 `windows-latest` 上把
固定提交的 MyGo 克隆到仓库旁边（供 `replace` 使用），运行 `go tool mygo build`，然后发布
GitHub Release，tag 形如 `v<版本>-<构建号>`（例如 `v0.2.0-42`），附带 exe 与安装包。也可以从
Actions 页面手动运行（`workflow_dispatch`）。发版前把 `mygo.json` 的 `version` 与
`src/version.go` 的 `appVersion` 一起改。

## 数据文件

启动器知道的一切都保存在可执行程序工作目录旁的
`mygo-launcher-data/go-launcher-data.json`：

- `apps` — 全局条目池（`guid` / `name` / `path` / `icon` / `args` / `working_dir` /
  `runtime_ms` / `run_as_admin`）
- `categories` — 标签，每个有 `slots` 数组存放条目 guid（`null` 表示网格空格子）
- `settings` — `auto_time` / `auto_hide` / `always_on_top` / `absolute_paths`

应用运行时会监听该文件（`mygo.WatchFiles`，事件驱动，不轮询）：手工保存后立即应用并刷新窗口。
不是合法 JSON 的文件会被忽略（保留当前数据）并给出提示；删除文件同样被忽略；从所有标签中移除的
条目会停止跟踪。设置里的「刷新」会立即重新读取文件，任何改动都会写回 store。

窗口自身的偏好也在同一目录：

- `mygo-launcher-data/ui-state.json` — 主题、强调色、列表/网格视图、当前标签
- `mygo-launcher-data/window-state.json` — 主窗口与设置窗口的位置、大小、最大化状态
- `mygo-launcher-data/cached-icons/` — 从条目指向的文件提取的图标

## 目录结构

- `mygo.json` — 应用名、标识、版本、入口包（`./src`）、输出目录
- `src/main.go` — MyGo 启动：单实例保护、窗口与原生 UI、文件拖入、store 监听
- `src/app.go` — `App`：store、运行中的进程、启动与停止、图标、提示、主线程调度
- `src/store.go` — 数据文件（`AppStore`、`parseStore`、`loadStore`、`writeStoreAtomic`）
- `src/store_watch.go` — 监听 `go-launcher-data.json` 的手工修改并重新加载
- `src/view*.go` — 原生界面，按区域分文件：`view.go`（帧入口、调色板、页面结构、添加
  FAB、忙碌条）、`view_header.go`（顶部标签与操作）、`view_tabs.go`（标签拖放与菜单）、
  `view_settings.go`（设置窗口）、`view_list.go` / `view_grid.go`（两种布局）、
  `view_dialogs.go`（条目/运行时/重命名/确认对话框）、`view_search.go`（搜索浮层）；
  具体动作在 `src/items.go` 与 `src/commands.go`
- `src/theme.go`、`src/ui_kit.go` — 浅色/深色调色板、强调色与共享控件（圆形图标按钮、分段控件）
- `src/icons.go`、`src/icons_svg.go` — 条目图标（`mygo.Shell.FileIcon`）与窗口绘制的 SVG
- `src/ui_state.go`、`src/window_state_store.go` — 视图与窗口自身的设置
- `src/launch_*.go`、`src/lnk_*.go`、`src/singleton_*.go` — 平台辅助：进程启动与提权、
  `.lnk` 解析、按安装目录的单实例保护
- `src/*_test.go` — store、监听器与界面测试；界面测试无头驱动真实视图（`ui.NewTester`）
- `docs/` — [图标说明](docs/ICON.md)、[界面重做交接](docs/UI-REDO.md)、
  [待办](docs/TODO.md)、图标生成脚本

## 相关文档

- [AGENTS.md](AGENTS.md) — 开发约定、界面模型、测试与目录说明
- [docs/UI-REDO.md](docs/UI-REDO.md) — 界面重做的设计方案、验证结果与交接事项
- [docs/ICON.md](docs/ICON.md) — 应用图标的设计、生成与缓存刷新
- [MyGo 文档](https://github.com/egoist/mygo/tree/main/docs)
- [MyGo 原生 UI](https://github.com/egoist/mygo/tree/main/ui)
