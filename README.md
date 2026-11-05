# MyGo Launcher

<p align="center">
  <img src="resources/icon.png" alt="图标" width="180" height="180" />
</p>

一个面向 Windows 的便携式桌面启动器，用 Go 和
[MyGo](https://github.com/egoist/mygo) 的原生 UI 编写——没有 WebView、HTML 或 JavaScript。
界面参照 [MyGo Music](https://github.com/zhangnjsd/mygo-music)：顶部标签导航、统一的浅色/深色
配色、圆角行与按钮，设置放在独立窗口中。

它把文件、程序和文件夹连同图标与累计运行时长列在标签页里，可以启动、停止它们，拖放文件即可
添加（拖到某个标签上会加入该标签，也可以点右下角的圆形 `+`），并通过右键菜单管理条目。可执行程序会自动计时：运行期间累计，
退出时写回（点击时长可按 ±1 小时或按分钟修正）；在设置里关掉「自动计时」后时钟不再走动，只能
手工编辑。条目可以以管理员身份启动（在条目编辑器里勾选 *以管理员身份运行*；该选项写入数据文件，
列表里显示盾牌标记；Windows 会弹出 UAC）。

默认情况下，点击窗口的关闭按钮不会退出，应用留在系统托盘继续运行：托盘图标一直显示，左键恢复
窗口，右键菜单有「打开」「设置」和「退出」。不需要时可以在设置的外观里关掉「关闭后在后台运行」，关闭
按钮就会像普通应用一样退出，托盘图标也不再显示。

顶栏齿轮打开设置，里面可以切换语言（简体中文、English）、主题、选择强调色（预设取自网站
`--bang-1` 至 `--bang-5`，即 Tomori、Anon、Rāna、Soyo、Taki 五位成员的颜色，各有明暗两套、随
主题自动切换，默认 `--bang-1`；另有自定义取色器）、关闭后在后台运行、自动计时、自动隐藏、
始终置顶、路径转换、数据目录与图标维护。列表/网格切换在顶栏、设置按钮左侧。应用启动后会检查
GitHub Releases，之后每天一次；「关于」里可以关闭自动检查、打开自动下载，或手动检查并安装新
版本。

界面支持简体中文和英语，第一次启动按 Windows 的显示语言选定，之后可以在设置的「语言」下拉框里
随时切换。

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
go tool mygo dev -lang zh-Hans   # 临时以中文运行（en 为英文），不写入设置
```

代码、配置或图标变化后会自动重新构建并重启。

## 构建

```sh
go tool mygo build                  # build/windows-amd64/MyGo Launcher.exe + NSIS 安装包
go tool mygo build -skip-installer  # 不生成安装包，快速验证
go build -o build ./src             # 只编译，产物在已忽略的 build/
go test ./...                       # 测试，含无头界面测试
go vet ./...
go tool mygo generate               # 改了 locales/*.json 后重新生成文案访问函数
go tool mygo i18n check             # 检查两种语言的键与参数是否对齐
```

入口包在 `src/`，由 `mygo.json` 的 `"main": "./src"` 指定。注意不要直接跑 `go build ./...`，
它会在仓库根目录生成 `src.exe`。

## 发布与更新

发版前把 `mygo.json` 的 `version` 与 `src/version.go` 的 `appVersion` 一起改，并在
`CHANGELOG.md` 顶部写好 `## <版本>` 小节；MyGo 会把这一节放进更新窗口和 Release notes。

推送 `v<版本>` 标签（例如 `v0.3.0`）会运行
[`.github/workflows/release.yml`](./.github/workflows/release.yml)：校验 tag 与版本一致，从
CHANGELOG 取说明，创建 **draft** Release，然后用 `MYGO_UPDATER_PRIVATE_KEY` 签名并执行
`go tool mygo build -platform windows/amd64 -upload`，把安装包、应用归档、`SHA256SUMS`、
更新包/增量包和 `update-windows-amd64.json` 一起上传。检查附件后手动发布 draft，并勾选
**Set as the latest release**，安装后的应用才会更新过去。

更新签名私钥在 MyGo 的用户配置目录（`mygo keygen` 的默认位置），并应保存为 GitHub secret
`MYGO_UPDATER_PRIVATE_KEY`；公钥写在 `mygo.json` 的 `updates.publicKey`。详细流程见
[docs/UPDATES.md](docs/UPDATES.md)。

## 数据文件

启动器知道的一切都保存在可执行程序工作目录旁的
`mygo-launcher-data/go-launcher-data.json`：

- `apps` — 全局条目池（`guid` / `name` / `path` / `icon` / `args` / `working_dir` /
  `runtime_ms` / `run_as_admin`）
- `categories` — 标签，每个有 `slots` 数组存放条目 guid（`null` 表示网格空格子）
- `settings` — `auto_time` / `auto_hide` / `always_on_top` / `absolute_paths` /
  `background_on_close`

应用运行时会监听该文件（`mygo.WatchFiles`，事件驱动，不轮询）：手工保存后立即应用并刷新窗口。
不是合法 JSON 的文件会被忽略（保留当前数据）并给出提示；删除文件同样被忽略；从所有标签中移除的
条目会停止跟踪。设置里的「刷新」会立即重新读取文件，任何改动都会写回 store。

窗口自身的偏好也在同一目录：

- `mygo-launcher-data/ui-state.json` — 语言、主题、强调色、列表/网格视图、当前标签
- `mygo-launcher-data/window-state.json` — 主窗口与设置窗口的位置、大小、最大化状态
- `mygo-launcher-data/cached-icons/` — 从条目指向的文件提取的图标

更新相关的偏好（自动检查、自动下载、上次检查时间、跳过的版本）和下载的安装包由 MyGo 的
updater 插件管理，保存在 MyGo 的用户数据目录，不在便携数据文件夹里。

## 目录结构

- `mygo.json` — 应用名、标识、版本、入口包（`./src`）、输出目录
- `src/main.go` — MyGo 启动：单实例保护、更新插件、窗口与原生 UI、文件拖入、store 监听
- `src/app.go` — `App`：store、运行中的进程、启动与停止、图标、提示、主线程调度
- `src/updates.go` — 自动更新插件、偏好与测试替换点
- `src/tray.go` — 托盘图标与菜单；关闭窗口时按设置隐藏到托盘
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
- `locales/` — 界面文案：`en.json`、`zh-Hans.json`，以及 `mygo generate` 写出的
  `catalog_gen.go`（不要手改）
- `resources/` — 应用图标；`resources.go` 把图标嵌入二进制，供托盘使用
- `docs/` — [图标说明](docs/ICON.md)、[更新与发布](docs/UPDATES.md)、
  [待办](docs/TODO.md)、图标生成脚本

## 相关文档

- [AGENTS.md](AGENTS.md) — 开发约定、界面模型、测试与目录说明
- [docs/UPDATES.md](docs/UPDATES.md) — 自动更新、签名密钥与发布流程
- [docs/ICON.md](docs/ICON.md) — 应用图标的设计、生成与缓存刷新
- [MyGo 文档](https://github.com/egoist/mygo/tree/main/docs)
- [MyGo 原生 UI](https://github.com/egoist/mygo/tree/main/ui)
