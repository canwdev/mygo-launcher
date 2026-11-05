---
description:
alwaysApply: true
---

# MyGo Launcher

## 项目概览

一个面向 Windows 的便携式桌面启动器，用 Go 和 [MyGo](https://github.com/egoist/mygo) 的原生
UI 编写：窗口由 Go 直接绘制，没有 WebView、HTML 或前端构建。它把文件、程序和文件夹连同图标与
累计运行时长列在页面里，可以启动、停止它们；界面参照 MyGo Music 的设计，使用顶部分页导航、
统一的配色与圆角控件，设置放在独立窗口中。`README.md` 是项目的主要说明文档。

所有 Go 源码（包括测试）都在 `src/`，入口包由 `mygo.json` 的 `"main": "./src"` 指定；仓库文档
放在根目录与 `docs/`。

## 语言

- **仓库文档一律用中文**：`README.md`、`CHANGELOG.md`、`AGENTS.md`、`docs/TODO.md` 和
  `docs/` 下的其他文件都如此。命令、标识符、API 名称保持原文。
- 代码注释不限制语言：已有英文注释不必改写，新注释用中文或英文都可以，同一文件内保持一致。

## 工具链

- Go 1.27+（`go.mod` 为 `go 1.27.3`）；原生 UI 需要它。
- MyGo 是 `github.com/egoist/mygo` 模块，`go.mod` **用 `replace` 指向 `../mygo`**，那个检出
  里带有 `ui` 包。没有这个检出就无法构建：把 MyGo 克隆到本仓库旁边，或删掉 `replace`。CI 在
  `release.yml` 里克隆固定提交。
- MyGo CLI 来自 `go.mod` 的 `tool` 指令：`go tool mygo`。
- 不需要 C 编译器、bun/npm 或 WebView2。

命令：

```sh
go tool mygo dev                 # 开发，改代码自动重启
go tool mygo build               # 产出 build/windows-amd64/MyGo Launcher.exe 与 NSIS 安装包
go tool mygo build -skip-installer   # 不生成安装包，快速验证打包
go build -o build ./src          # 只编译，产物在已忽略的 build/
go test ./...                    # 测试，含无头界面测试
go vet ./...
go tool mygo generate            # 改了 locales/*.json 后重新生成文案访问函数
go tool mygo i18n check          # 检查各语言的键与参数是否对齐
GOOS=linux go build -o build/launcher-linux ./src   # _other.go 桩必须能编译
```

注意：入口在 `src/`，直接运行 `go build ./...` 会在仓库根目录生成 `src.exe`，应改用
`go build -o build ./src`。

`mygo.json` 保存应用名、标识、版本、入口包（`main`）和输出目录；`resources/icon.png` 是应用
图标（见 `docs/ICON.md`）。

## 界面

**窗口每一帧都从状态重新构建。** `src/view.go` 里的 `view.view(c *ui.Context)` 读取 `App` 的
store 和 view 自己的字段来搭界面；帧之间只保留这些字段。由此得到以下规则：

- **帧之间执行的动作必须请求新帧。** 点击和菜单回调在构建之外运行，改变窗口显示内容的回调要调用
  `v.refresh()`（`later`、`savePrefs`、`App.save`、`App.notify` 已经会这么做）。否则变化要等到
  下一次无关重绘才出现。
- **不要在构建集合的循环里修改集合。** 增删、重排和打开对话框都走 `v.later(fn)`：它把动作排到
  下一次构建前，并请求一帧；`view()` 先清空队列，执行过任何动作就再请求一帧。
- **`mygo.Theme.SetSource` 不能在帧内运行**：它会向主线程投递任务，在无头 `ui.Tester` 里会
  死锁。`applyTheme` 只在启动时和设置窗口里运行；主题点击先保存偏好，再交给 `app.dispatch` 在
  帧后执行。无窗口（测试）时不改主题。
- **系统对话框会阻塞**：`mygo.Dialog.Open` 在 goroutine 上运行；没有窗口时（测试）它同步
  执行。**添加文件用自己的打开对话框**（`src/open_dialog_windows.go`）：MyGo 的对话框会解引用
  `.lnk`，选中的快捷方式会变成目标程序并丢掉参数；`openFilesDialog` 用
  `FOS_NODEREFERENCELINKS` 返回 `.lnk` 本身，再交给 `resolveShortcut`。
- `app.win` 在 `WhenReady` 之前以及测试里都是 nil。窗口操作统一走 `app.dispatch`、
  `app.refresh`、`app.minimize` 和 `app.restore`，它们会处理这种情况。

### 配色与主题

- `src/theme.go` 的 `palette` 负责浅色/深色下的背景、侧栏色、文字、分隔线、悬停、强调色等。
- `applyPalette(c, accent)` 每帧复制 `c.Theme()`，覆盖 `Accent`、`AccentText`、`Selection`
  后 `c.SetTheme`。浅色/深色本身仍由 MyGo 的 ThemeSource 决定，调色板只改视图用色。
- 强调色是用户偏好：`ui-state.json` 的 `accent` 存预设 id（`bang-1` 至 `bang-5`，默认
  `bang-1`）或自定义 `#rrggbb`。预设取自网站 `styles.css` 的 `--bang-1` 至 `--bang-5`，即
  MyGO!!!!! 五位成员 Tomori、Anon、Rāna、Soyo、Taki 的颜色；每个预设都有明暗两套，随主题自动
  切换。`accentText` 按亮度选择白字或深色字。强调色只影响调色板，不需要改 ThemeSource。

### 布局

- 界面把 store 的 `categories`（代码里的 tab）叫作「页面」，`slots` 里的 `null` 叫作「空白」；
  内部标识符仍用 tab / slots。
- 窗口是纵向结构：50 DIP 顶栏（`src/view_header.go`）+ 内容区 + 底部忙碌条。
- 顶栏从左到右：横向页面胶囊与「新建页面」按钮、搜索、列表/网格切换、设置。没有品牌、分割线或
  大标题；页面多时横向滚动。
- 页面胶囊悬停 200ms 会自动切换（`hoverTab`/`hoverSince`）。页面多时用 `ScrollHorizontal` 的
  原生横向滚动条滚动；点击或新建页面后，`view()` 末尾按 `tabRects` 把当前胶囊滚到中间
  （`centerNav`）。滚轮不做特殊处理。
- 内容区直接是列表或网格；右下角是圆形的添加 FAB（只有 `+` 图标），打开「选择文件 / 手动创建」
  菜单。FAB 绝对定位、悬浮在内容之上：列表和网格都用 `ui.Scroll` + 内容列/网格，
  内容从 FAB 下面滚过，滚动内容末尾留 68 DIP 保证滚到底时最后一行仍在 FAB 上方。
  网格留小边距（`8, 8, 68, 8`），列表用 `12, 10, 68, 10`。不要把这段留白放回滚动
  容器，那会让内容提前被裁掉，留下一块空白。
- 网格单元之间没有间隙，也不画卡片底色，只在悬停时显示圆角背景；条目比列表更小更紧凑，
  图标与名称之间留 6 DIP：
  `gridBody` 用
  `ui.Grid`（`Gap(0)`）按窗口宽度算出列数，因此不再需要 `GridView` 和紧凑主题。
- 网格单元左上角的时长默认不画，悬停该单元（或进程运行中）才出现。
- 图标与行高由 `ui-state.json` 的 `icon_size`（`small` / `medium` / `large`，默认
  `medium`）决定，`src/icon_size.go` 给出两套视图的尺寸。
- **菜单只通过右键打开**：列表行、网格单元、空白和页面都没有 ⋯ 按钮，各自在元素上设置
  `PopoverMenu` / `ContextMenu`。
- 列表行不显示手型光标，因为单击不会启动；网格单元单击即启动，可以保留手型。

### 拖放

- 在行、单元格或页面上调用 `Drag(value)`，指针移动几个 DIP 后开始拖动；另一侧的 `ui.Drop[T]`
  和 `ui.DragOver[T]` 接收。
- **一个元素只接收一种拖动值**：`Drop`/`DragOver` 在元素状态上只保存一个 `accepts` 闭包，
  后一次调用覆盖前一次。因此页面内部的胶囊接收 `dragItem`（拖入条目），外层包装元素接收
  `dragTab`（页面重排）——不要把两种类型放在同一个元素上。
- 值按类型匹配：条目是 `dragItem`，页面是 `dragTab`（`src/items.go`）。页面内槽位重排走
  `reorderSlots`，以槽位索引为准，包含空白。
- **没有落点就放到末尾**：整个 `page` 也接受 `dragItem`，在行、单元和页面胶囊都没接住时把
  条目追加到当前页面末尾；拖到条目本来所在的页面胶囊上放下则什么都不做（按住 Ctrl/Cmd 仍复制）。
- **插入线统一三种放置**：`insertLine(e, vertical, after, p)` 在网格单元和页面胶囊的左右边、
  列表行的上下边画线；`dropAfter(e, vertical)` 按指针在目标中心前/后决定落点，再经
  `dropTo(from, at, copy)` 换算成 `moveSlot` 的索引。拖动时目标显示线而不是整块边框；页面
  胶囊重排、网格/列表条目换位都用它；`dropDragged` 按 `dragItem.tab` 区分同页换位
  （`reorderSlots`）与跨页移动（`moveItemToTabAt`）。
- **拖动期间所有页面都保留构建并叠放**：`body` 用 `Key(tab.GUID)` 包住每页；`v.itemDragging`
  为真时其他页面也构建，先画（绝对定位铺满），当前页面最后画并带 `p.bg` 背景盖住它们。这样
  指针停在另一个页面胶囊上（`tabHoverDelay` = 200ms）切换页面时，拖动源元素没有被卸载、MyGo
  跟着指针的预览也继续画，可以直接落到新页面的某个位置；否则 MyGo 会结束按下状态，释放时不
  产生 drop，还会留下一个陈旧拖动。`v.itemDragging` 在 `view()` 末尾把本帧所见交给下一帧。
- **从资源管理器拖入文件**：整个 `page` 仍是兜底放置目标（追加到当前页面），拖到页面胶囊上由
  `tabChip` 的 `DroppedFiles()` 接收，加入该页面；文件停在同一胶囊上 200ms 也会切换页面。
  每个条目把两半（网格左右、列表上下）做成透明绝对定位的小盒子，调用 `fileDropHalf` 注册
  `DroppedFiles()` / `FileDragOver()`：原生文件拖动只告诉落在哪个元素，不知道元素内的位置，
  两半用来区分插在条目前还是后，并画出插入线。两半带 `Key`（`file-before` / `file-after`），
  网格单元的运行时字段也始终构建、只按悬停 `Hidden`：原生 drop 按元素 ID 找目标，卡片里子元素
  数量一变就会让两半的 ID 错位，文件要么落空、要么落到另一半。调用 `DroppedFiles()` /
  `FileDragOver()` 才会注册为放置目标；文件拖动不画整窗遮罩，只用条目上的插入线。没有条目
  接收的文件落到页面本身（追加到末尾），最后仍由 `main.go` 的 `OnFileDrop` 兜底。

### 图标

`mygo.Shell.FileIcon(path, px)` 把 PNG 写进图标缓存，`ui.DecodeImage` 再读成
`ui.ImageSource`；没有手工提取图标的路径。图标指向 `mygo-launcher-data/cached-icons/` 的条目
视为自动生成（`isAutoIcon`），「更新图标」和批量更新据此生效。快捷方式的图标取自 `.lnk`
本身（Explorer 画的带应用徽标的图标），取不到才退回目标程序：有些目标（如 `chrome_proxy.exe`）
只有空白图标。条目的 `link` 字段记住这个 `.lnk`，单条和批量更新图标都用它；编辑条目改了目标
路径时清空。
- 条目编辑器可选的图标是图片、`.ico`、`.exe`、`.dll`；`.ico`/`.exe`/`.dll` 旁边显示「序号」，
  取第几个图标（0 为第一个）。保存时 `App.customIcon` 把结果规范化后放进
  `mygo-launcher-data/custom-icons/`：图片解码后最长边缩到 256 并重新编码 PNG（源文件最多
  64MB、4000 万像素，落盘图标不超过 10MB，SVG 原样复制）；程序/DLL 用
  `extractIconResource`（`SHDefExtractIcon`，失败退 `ExtractIconEx`）按序号提取。已经在
  custom-icons 或 cached-icons 里的图标不再复制。`iconTakesIndex` 决定对话框是否显示序号。
- 只有 custom-icons 与 cached-icons 里的图标会显示（`allowedIcon`）；外部路径的图标不显示，
  保存时会复制进 custom-icons。「更新图标」也走同一套：图标在 custom-icons 里就不动，在外面
  就 `customIcon` 收进来（图片缩小），自动图标仍从 `link`/`path` 重画；批量更新同样处理。
  删除旧图标前只在自动/自定义目录里删文件，绝不删用户目录里的原件。`pruneIcons` 在启动、
  删除条目/页面和更换图标后删掉两个目录里无人引用的文件。

### 多语言

- 界面文案都在 `locales/en.json`（源语言）和 `locales/zh-Hans.json`；`mygo.json` 的 `i18n`
  指向这个目录。改完 JSON 运行 `go tool mygo generate`，它写出 `locales/catalog_gen.go`
  （**不要手改**，每个键一个类型化函数），并让 `locales` 包在初始化时注册语言、同时引入
  MyGo 自己的中文翻译。`src` 通过 import `go-launcher/locales` 引用这些函数。
- **帧内文案走 `v.msgs()`**：`view()` 和 `settingsView()` 每帧把 `v.lang` 设为
  `c.Locale()`，`v.msgs()` 返回该语言的 `locales.Messages`。不要直接调用包级的
  `locales.X()`——那读的是应用语言；无头 `ui.Tester` 的语言是 `""`（源语言英语），测试才能
  稳定断言英文，需要中文时用 `tt.SetLocale("zh-Hans")`。
- 应用层的提示、系统对话框标题和错误文字（不在帧内）用包级函数 `locales.X()`，它们跟随
  `mygo.App.Locale()`。新增错误信息若会显示成 toast，也要进文案。
- 语言偏好是 `ui-state.json` 的 `language`（`auto` / `en` / `zh-Hans`，默认 `auto`）。
  `applyLanguage` 在启动时调用一次 `mygo.App.SetLocale`，`auto` 传 `""` 跟随系统语言，所以
  首次启动按 Windows 的显示语言选中；`MYGO_LOCALE`（`mygo dev -lang`）设置时跳过，让它优先。
  设置里的语言是 `ui.Select` 下拉框，只列简体中文和 English（没有「跟随系统」项）：`auto`
  时显示系统选定的语言，选择后走 `changeLanguage`，和主题一样在帧外执行 `SetLocale`。设置窗口
  标题在 `OnLocaleChanged` 里跟着更新。
- 运行时长单位也走文案（`durationDay` 等），`formatRuntime(lang, ms)` 需要语言参数。
- 新增一条文案：两个 JSON 都加键（源语言的复数等元数据写在 `en.json`），运行 generate，再用
  `v.msgs().KeyName()` 或 `locales.KeyName()` 取用。

## 设置窗口

- `src/view_settings.go`：`openSettings` 首次创建设置窗口，之后只显示并聚焦；关闭时清空指针。
  设置窗口是一个独立的 `mygo.NewWindow`，560×720（最小 480×420），`StateKey: "settings"`，
  不提供最大/最小化，背景 `light-dark(#f4f4f6, #1c1c1f)`。
- `app.win` 为 nil 时 `openSettings` 直接返回，所以测试用
  `ui.NewTester(v.settingsView, 560, 720)` 单独驱动设置内容。
- `App.refresh` 同时刷新主窗口和设置窗口；设置里的改动立刻反映到启动器。
- 设置分组：外观（语言下拉框、主题分段控件、强调色色板（悬停提示成员名字）+ 打开取色器的
  显示当前颜色的圆形编辑按钮，色板下的
  `MyGo!!!!!` 是字面文字、不进文案、图标大小分段控件、关闭后在后台运行）、行为（自动计时、
  自动隐藏、始终置顶）、路径（新条目绝对路径、批量转换）、数据（刷新、打开数据目录、
  更新图标、从 CLaunch 迁移）、关于（版本、GitHub、更新）。列表/网格切换在顶栏，不在设置里。

## 自动更新

- `src/updates.go`：`useUpdater` 在 `App.Run` 之前调用
  `mygo.Use(native.New(updater.Options{}))`，启用后台检查和原生更新窗口，并把 `app.updates`
  设为 true。`updater.OnChange` 调用 `app.refresh()`，让设置窗口随检查和用户选择重画。
- 更新窗口跟随 `mygo.App.Locale()`。启动约 10 秒后检查一次，之后每天一次；自动检查、自动
  下载、上次检查时间和跳过的版本由插件保存在 `updater.json`（MyGo 用户数据目录），不在
  便携数据文件夹里。
- 设置 → 关于 只在 `app.updates` 为真时显示更新项：自动检查开关
  （`updateChecks` / `setUpdateChecks`）、自动下载开关（`updateDownloads` /
  `setUpdateDownloads`，自动检查关闭时禁用）、手动检查按钮（`checkForUpdates`）和状态
  （`availableUpdate` / `lastUpdateCheck`）。
- 这些函数在 `src/updates.go` 里是包级变量，测试用 `fakeUpdater` 替换；没有启用插件时直接
  调用会 panic，所以界面也用 `app.updates` 判断。
- 发布方案与 MyGo Music 相同：`mygo.json` 的 `updates.publicKey` + `github`
  （`canwdev/mygo-launcher`）；推送 `v<version>` 标签触发 draft Release，CI 用 secret
  `MYGO_UPDATER_PRIVATE_KEY` 执行 `go tool mygo build -platform windows/amd64 -upload`。
  `CHANGELOG.md` 必须有 `## <version>` 小节，发布 draft 时勾选 latest。细节见
  `docs/UPDATES.md`。

## 托盘与后台运行

- 开关是 store `Settings` 的 `background_on_close`，默认开启；关掉后不创建托盘，关闭窗口
  就是退出。
- `src/tray.go`：`syncTray` 在开关打开时创建 `mygo.NewTray`（稳定 ID `main`，图标把
  `resources/icon.png` 缩放到 64px，`resources.go` 用 `go:embed` 带进二进制），关闭时
  `Destroy`；语言变化或开关变化时重设菜单。托盘只在开关打开时存在，且一直显示。
- 托盘左键和右键菜单的「打开」恢复主窗口（`showMain`，最小化时先 `Restore` 再 `Show`，
  因为 `Show` 不会取消最小化）；菜单还有「设置」（`openSettings`）和「退出」（`App.Quit`）。
- 主窗口的 `OnClose` 交给 `view.handleClose`：开关打开且不在退出流程（`App.quitting`）时
  `PreventDefault` 并 `Hide`；否则照常关闭。`OnBeforeQuit` 在退出开始时置位 `quitting`，
  所以托盘的「退出」不会被拦下；`OnQuit` 销毁托盘。
- 关掉开关时若主窗口正隐藏，先用 `showMain` 显示再移除托盘，否则窗口没有入口。
- 测试里 `syncTray` 因为没有窗口（`app.win == nil`）直接返回：`mygo.NewTray` 要求应用
  就绪；测试覆盖设置持久化和 `handleClose` 的判定。

## 数据

- 所有数据都在工作目录旁的 `mygo-launcher-data/`：
  - `go-launcher-data.json` — store：`apps`、`categories`（页面，`slots` 存条目 guid，`null`
    表示空白）和 `settings`（`auto_time` / `auto_hide` / `always_on_top` /
    `absolute_paths` / `background_on_close`）
  - `ui-state.json` — 窗口自身偏好：语言、主题、强调色、列表/网格、当前页面
  - `window-state.json` — 两个窗口的几何信息，通过 MyGo 的 `WindowStateStore`
  - `cached-icons/` — 自动生成的图标（`writeIcon`）
  - `custom-icons/` — 用户在条目编辑器里选择的图标，按 `<名称>-<时间戳>.png` 保存
- store 只通过 `App.save` → `writeStoreAtomic` 写入，并记录文件指纹（mtime、大小、内容哈希），
  让监听器（`src/store_watch.go`，`mygo.WatchFiles`）区分应用自己的写入和手工编辑。手工改到
  一半（非法 JSON）的文件会被忽略，不会被当作空 store。
- store 里的路径默认相对于工作目录，除非「新条目使用绝对路径」；`convertItemPaths` 负责转换，
  保持往返可用。
- 可执行程序运行期间会被跟踪（`startTracked`），退出时把时长加到 `runtimeStats`；运行时对话框
  通过 `setRuntimeMs` 按分钟或 ±1 小时覆盖已保存的值，「自动计时」关闭时时钟不动。

## 从 CLaunch 迁移

- `src/claunch.go` 只读 `CLaunch.ini`：UTF-16LE（带 BOM）或 UTF-8，按顺序保留重复的
  `[Btn###]` 分节；`[Page###]` 变成新页面，`[SubMenu###]` 的条目在父按钮的位置展开，`Type` 映射见 `docs/MIGRATION.md`。
- 相对路径以 `CLaunch.exe` 所在目录为基准，`%VAR%` 展开；`IconIndex` 无法还原。
- shell 目标（`ms-settings:`、`shell:AppsFolder\…`、`::{GUID}`）由 `isShellTarget` 识别：
  `absPath`、`normalizePath` 不改写它们，`planLaunch` 不跟踪进程，打包应用走
  `mygo.Shell.LaunchApp`。
- 设置 → 数据 最下方是入口：系统对话框会阻塞，因此 `migrateCLaunch` 在 goroutine 上选文件、
  读 ini 和提取图标，结果经 `app.dispatch` 回主线程写 store；无窗口（测试）时入口直接返回。
- 测试在 `src/claunch_test.go`，用一份缩小的 ini 覆盖解析、映射和导入，不打开真实对话框。

## 测试界面

`src/view_test.go` 用 `ui.NewTester` 无头驱动真实视图：

- **先调用 `tt.SetFocused(true)`**：未聚焦窗口的第一次点击只会聚焦。
- **弹出菜单画在窗口内**，按普通元素点击菜单项（打开菜单后 `tt.Click("Edit")`）。
  `Tester.Menu()` / `ChooseMenuItem` 只用于原生系统菜单（`mygo.NewMenu(...).Popup`），不用于
  `PopoverMenu` / `PopoverMenuButton`。
- 用 `Press`、`Move`、`Release` 拖动（`view_test.go` 的 `drag` 辅助函数）；只移动几个 DIP 的
  按压仍算点击。
- 文本输入会保留现有内容，`tt.Type` 是追加。
- `later` 排队的动作需要一帧；`tt.Click` 会让视图请求的帧跑完。
- **设置窗口单独测试**：`newTestSettings` 构造 `ui.NewTester(v.settingsView, 560, 720)`。开关
  使用独立的 `Toggle …` 标签以便点击；不要在测试里点主题，`SetSource` 会阻塞。强调色预设可以
  通过色板的 `Tomori` / `Anon` 等成员名标签点击。
- **更新插件在测试里要替换**：`fakeUpdater` 换掉 `src/updates.go` 的 `checkForUpdates`、
  `updateChecks` 等包级函数，并把 `app.updates` 设为 true 才会显示「关于」里的更新项。
- 测试不能碰真实数据目录：`newTestApp` 把 `absBase` 指向临时目录。
- **测试默认是英语**：`ui.Tester` 的语言起初是 `""`，`v.msgs()` 因此返回源语言。要验证另一种
  语言，用 `tt.SetLocale("zh-Hans")`（它会立即重画一帧），不要点设置里的语言控件——那会调用
  `SetLocale` 并落盘。

## 变更日志

`CHANGELOG.md` 在仓库根目录，保持**精简**：只记录用户能感知的变化，一事一条。

- 最新版本在最上面，新条目加到最新版本标题下。
- 分组按需出现，不保留空标题：`### 破坏性变更` / `### 界面` / `### 功能` / `### 修复` /
  `### 工程`。`工程` 只放影响开发流程（构建、打包、测试）的内容，内部重构不记。
- 用一句话说明用户看到的变化；不要列文件名或标识符——那是 git log 的职责。
- 每条以简短的粗体标签开头，加冒号，再接句子：`- **运行时**：……`、`- **标签**：……`。
- 版本号要在两处一致、发版时同时修改：`mygo.json` 的 `version` 和 `src/version.go` 的
  `appVersion`。

## Git

提交信息不要添加 `Co-authored-by` trailer；提交只属于仓库作者本人。

## 目录结构

| 路径 | 作用 |
| --- | --- |
| `mygo.json` | 应用名、标识、版本、入口包 `./src`、输出目录 |
| `go.mod` / `go.sum` | Go 模块与依赖（MyGo 用 `replace` 指向 `../mygo`） |
| `src/main.go` | 单实例保护、MyGo `WindowOptions`、`ui.View(v.view)`、文件拖入、store 监听 |
| `src/app.go` | `App`：store、运行中的进程、启动/停止、图标、提示（toast）、`dispatch`、设置窗口 |
| `src/store.go` | 数据文件：`AppItem` / `CategoryNode` / `Settings` / `AppStore`、`parseStore`、`loadStore`、`writeStoreAtomic`、路径辅助 |
| `src/store_watch.go` | 监听 store 文件、指纹、重新加载（`storeReloadApplied` / `Invalid`） |
| `src/theme.go` | 浅色/深色调色板与强调色偏好（预设、自定义、对比文字色） |
| `src/ui_kit.go` | 共享控件：圆形图标按钮、分段控件 |
| `src/view.go` | 帧入口：调色板、页面结构、列表/网格主体、添加 FAB、忙碌条、`later`/`refresh` |
| `src/view_header.go` | 顶栏：页面胶囊、新建页面、搜索、列表/网格切换、设置 |
| `src/view_tabs.go` | 页面胶囊的行为：右键菜单、条目拖入、页面重排 |
| `src/view_settings.go` | 设置窗口及其内容（齿轮打开） |
| `src/view_list.go` / `src/view_grid.go` | 两种布局的行与单元格，以及拖放 |
| `src/view_dialogs.go` | 条目、运行时、重命名和确认对话框 |
| `src/view_search.go` | 搜索浮层：过滤、键盘操作、在页面中定位 |
| `src/items.go` / `src/commands.go` | 页面与条目操作；菜单和按钮实际执行的动作 |
| `src/icons.go` / `src/icons_svg.go` | 条目图标与窗口绘制的 SVG |
| `src/icon_extract_*.go` | 从 `.ico` / `.exe` / `.dll` 的图标资源提取 PNG |
| `src/tray.go` | 托盘图标、菜单，以及关闭窗口时隐藏到托盘的拦截 |
| `src/ui_state.go` / `src/window_state_store.go` | `ui-state.json` 偏好；`window-state.json` |
| `src/launch_*.go` / `src/lnk_*.go` / `src/singleton_*.go` | 进程启动与提权、`.lnk` 解析（COM）、按安装目录的单实例 |
| `src/open_dialog_*.go` | 添加文件的打开对话框；Windows 上保留 `.lnk`，其他平台用 MyGo 的 |
| `src/claunch.go` | CLaunch.ini 解析与迁移：解码、分节、Type 映射、导入到 store |
| `src/utils.go` | `formatRuntime`、路径规范化、`splitArgs`、`isExecutable`、`isShellTarget` |
| `src/version.go` | `appVersion`，与 `mygo.json` 保持同步 |
| `src/updates.go` | updater 插件、更新偏好函数与测试替换点 |
| `locales/` | 界面文案 `en.json` / `zh-Hans.json` 与生成的 `catalog_gen.go` |
| `docs/` | `ICON.md`（图标）、`UPDATES.md`（更新与发布）、`MIGRATION.md`（CLaunch 迁移）、`TODO.md`（待办）、`screenshot.webp`（界面截图）、`generate-icon.ps1` |
| `resources/` | 应用图标等打包资源；`resources.go` 把图标嵌入二进制，供托盘使用 |

没有前端。新增一个操作意味着在 `view` 或 `App` 上添加方法，并从需要它的菜单或按钮调用。
