---
description:
alwaysApply: true
---

# MyGo Launcher

## 项目概览

面向 Windows 的便携式桌面启动器，用 Go 和 [MyGo](https://github.com/egoist/mygo) 的原生 UI
编写：窗口由 Go 直接绘制，没有 WebView、HTML 或前端构建。它把程序、文件和文件夹连同图标与
累计运行时长列在页面里，可以启动、停止；界面参照 MyGo Music，顶部分页导航，设置放在独立窗口。
`README.md` 是面向用户的主要文档。

源码（含测试）都在 `src/`，入口由 `mygo.json` 的 `"main": "./src"` 指定。

## 语言

- 仓库文档一律用中文（`README.md`、`CHANGELOG.md`、`AGENTS.md`、`docs/`），命令、标识符和
  API 名称保持原文。唯一例外是 `README-en.md`：`README.md` 的英文版，两份要同步改。
- 代码注释不限语言，同一文件内保持一致。

## 工具链

Go 1.27+（`go.mod`）；MyGo 用 `go.mod` 里的发布版模块，不从本机检出替换；CLI 是 `go tool mygo`。
不需要 C 编译器、bun/npm 或 WebView2。

```sh
go tool mygo dev                     # 开发，改代码自动重启
go tool mygo build                   # build/windows-amd64/ 里的 exe 与 NSIS 安装包
go build -o build ./src              # 只编译；直接 go build ./... 会在根目录生成 src.exe
go test ./...                        # 含无头界面测试
go vet ./...
go tool mygo generate                # 改了 locales/*.json 后重新生成文案访问函数
go tool mygo i18n check              # 检查各语言的键与参数
GOOS=linux go build -o build/launcher-linux ./src   # _other.go 桩必须能编译
```

- CI 发布 Windows 10/11 的 x64 与 arm64。Windows 386 能编译但运行时退出，Go 1.27 也不再支持
  Windows 7/8。
- UPX 曾试过，因内存从约 50MB 涨到约 90MB 且要维护 MyGo 补丁而弃用（`docs/UPDATES.md`）。

## 界面

**窗口每一帧都从状态重新构建**：`src/view.go` 的 `view.view(c *ui.Context)` 读取 `App` 的
store 和 view 自己的字段来搭界面，帧之间只保留这些字段。由此有四条硬规则：

- **帧之间执行的动作必须请求新帧**：点击与菜单回调要调 `v.refresh()`（`later`、`savePrefs`、
  `App.save`、`App.notify` 已经会）。
- **不要在构建集合的循环里改集合**：增删、重排、开对话框都走 `v.later(fn)`。
- **`mygo.Theme.SetSource` 不能在帧内运行**：它向主线程投递任务，无头 `ui.Tester` 里会死锁。
  主题先存偏好，再交给 `app.dispatch` 在帧后执行；无窗口（测试）时不改主题。
- **系统对话框会阻塞**：`mygo.Dialog.Open`、`PickIconDlg`、CLaunch 迁移都在 goroutine 上跑，
  测试里同步执行或用包级变量替换。添加文件必须用自己的打开对话框
  （`src/open_dialog_windows.go`）：MyGo 的会解引用 `.lnk`，让快捷方式变成目标程序并丢参数。
- `app.win` 在 `WhenReady` 之前和测试里都是 nil，窗口操作统一走 `app.dispatch` / `refresh` /
  `minimize` / `restore`。

### 配色与主题

`src/theme.go` 的 `palette` 给浅色/深色两套背景、侧栏、文字、分隔线、悬停色；`applyPalette`
每帧复制 `c.Theme()`，覆盖 `Accent`、`AccentText`、`AccentHover`、`AccentPressed`、
`InactiveAccent` 和 `Selection`（只设 `Accent` 的话，按钮悬停或窗口失焦会闪回 MyGo 默认的蓝色），
明暗仍由 MyGo 的 ThemeSource 决定。强调色来自 `ui-state.json` 的 `accent`（`bang-1`…`bang-5`，即 MyGO!!!!! 五位成员的配色，
各有明暗两套；或自定义 `#rrggbb`），`accentText` 按亮度选字色。

### 背景材质

设置 → 外观的「背景材质」（`ui-state.json` 的 `window_material`：空 / `mica` / `acrylic`）让
Windows 11 在窗口背后画系统材质（Mica、Acrylic）；Win10 没有，MyGo 报告没材质。

- 窗口创建时带 `Transparent` + `Vibrancy`，所以改材质要重启才生效（设置里换材质直接
  `mygo.App.Relaunch()`，不提示「重启后生效」）。
- `c.Vibrancy()` 报告本帧是否真有材质。有材质时根节点和顶栏都是透明的，材质原样露出来；**只有内容
  列**（`body()` 外面那层 `ui.Column`）画半透明底色 `v.windowFill(p.bg)`（`src/window_material.go`，
  透明度是 `materialTintAlpha`，调它即可更透或更实），网格里的字才有已知的明暗可依。
  **别把根节点或内容列都设成透明**——那样谁也画不出这层底色，内容区就完全裸在材质上，鲜艳壁纸下字
  看不清（踩过两次：「底色消失」的根因就是根节点被设成 `ui.Transparent`）。
- 有材质时窗格（`v.surface(...)`）返回 `ui.Transparent`：顶栏、`body()` 里每页的绝对定位层都走它，
  这样露出来的才是材质本身；漏一个就在磨砂背景上留一块纯色（当初内容区那片白就是页面层直接画
  `v.pal.bg` 造成的）。内容列是唯一例外，它画 `windowFill`。
- `Theme.Background` 仍是调色板底色（不透明）：它是 MyGo 自己的对话框（`coreModal`）、弹出面板
  （`stylePanel`）、输入框的底色，设成透明会让这些面板没有底。tooltip 与 toast 另走
  `t.Inverse` / `t.InverseText`（见下）。
- tooltip / toast 取色是 `ui.(*Theme).inverse()`：`Inverse` 和 `InverseText` 都必须显式设成调色板
  的浮层色（`p.sidebar` / `p.text`），否则 MyGo 回退到「反色」——浅色主题下就是一颗深色胶囊，跟不
  随主题；只设 `InverseText` 而 `Background` 透明时更会变成「黑胶囊 + 看不见的字」。
- 主窗口的对话框都走 `v.modal(...)`（`src/view_modal.go`），它和 `ui.Modal` 一样用
  `ui.DialogBase`，只是面板画调色板底色。设置窗口的对话框仍可直接用 `ui.Modal`（那边没有材质）。
- 设置窗口不参与（单独创建，没有材质），调色板按 `material=false` 应用。

### 布局

- store 的 `categories` 在界面叫「页面」，`slots` 里的 `null` 叫「空白」；内部标识符仍是
  tab / slots。
- 窗口纵向：50 DIP 顶栏（`src/view_header.go`）+ 内容区 + 底部忙碌条。顶栏从左到右是页面胶囊
  与「新建页面」、搜索、设置，没有品牌和大标题。
- 页面胶囊悬停 200ms 自动切换；页面多时用 `ScrollHorizontal` 原生滚动条，点击或新建后
  `view()` 末尾按 `tabRects` 把当前胶囊滚到中间（`centerNav`）。
- 搜索浮层（`src/view_search.go`）是模态框：MyGo Music 风格的圆角搜索框，放大镜内嵌、聚焦时
  强调色描边与光晕、有输入时右侧圆形清除按钮；只在有查询时显示结果，空查询只留搜索框。
- 内容区是网格，右下角有圆形 `+` FAB，点击在它上方开紧凑菜单（「运行中的窗口」「系统位置」
  带二级面板，「选择文件 / 手动创建」）。FAB 绝对定位悬浮在内容之上：网格用 `ui.Scroll`，
  **滚动内容末尾留 68 DIP**、网格边距 `8, 8, 68, 8`，这样最后一行不会被 FAB 盖住；这段留白
  不能放回滚动容器，否则内容提前被裁掉。
- 网格单元之间没有间隙也不画卡片底色，只在悬停时显示圆角背景；`gridBody` 用 `ui.Grid` 按窗口
  宽度算列数（不需要 `GridView`）。名称自动换行居中、最多两行；管理员条目在图标左下角画盾牌
  （`view_item.go` 的 `adminBadge`）；缺失的条目图标 `Grayscale()` 去色、名称用 muted 色。
- 单元左上角的时长默认不画，悬停该单元或进程运行中才出现。计时字段必须在文件拖放的两个半区
  **之后**构建（否则点击落在半区），它自己也注册成文件放置区，并始终构建以保证元素 ID 稳定。
- 图标与单元格大小由 `icon_size`（`small` / `medium` / `large`）决定，见 `src/icon_size.go`。
- **菜单只通过右键打开**，没有 ⋯ 按钮：网格单元、空白和页面胶囊各自用 `src/view_menu.go` 的
  `contextMenu` 挂 `ui.ContextMenuBase` 自绘菜单，打开状态按元素键存在 `view.menus`。例外是
  文件夹条目的面板——那是「打开文件夹」，所以单击展开。
- 条目菜单始终有「插入空白」；菜单项不预先查磁盘，路径不存在时由 `App.reveal` /
  `App.updateIcon` 返回 `PathMissing` 再提示。
- 单元单击即启动（文件夹条目例外，单击开 `src/view_folder.go` 的面板：逐层进入、文件用系统
  程序打开、右键弹系统原生菜单、点外面或 Escape 关闭；目录在 goroutine 上读，图标只对可见行
  异步取，面板打开期间 `wheelInput` 直接返回）。
- 对话框宽度用 `dialogWidth(c, preferred)` 按窗口宽度收缩，高度由 `dialogBodyMax` 控制，放不下
  的字段在对话框内滚动。
- `page` 始终挂 `HandleInput(wheelInput)`：开启「滚轮切换页面」时竖直滚轮被吃掉并切页，Shift
  则滚动网格；一次事件最多切一页。指针停在胶囊上滚轮切页时，用 `hoverIgnoredTab` 压掉它的
  200ms 悬停切换。

### 键盘与 Tooltip

按键由 `src/view_keyboard.go` 用 `Context.OnWindowShortcut` 声明，不依赖焦点；模态或浮层打开时
根视图的快捷键不注册（`overlayOpen()` 兜底），只留 `Ctrl+F` 和 `Ctrl+,`。

- 选中项是槽位下标（`keySel` + `keySelTab`），空白不参与；方向键到边缘时 `switchPageKey` 翻页。
- 悬停等同选中（`hoverSlot`），但指针不动时方向键仍可离开；键盘导航（`keyboardNav`）期间不画
  悬停高亮，鼠标一动就 `pointerBack` 交还。
- `keyboardFocus` 时选中格子里放一个 0×0 的 `key-focus` 盒子抢焦点，让回车落到选中条目而不是
  之前点过的按钮。
- 回车等同单击。Tab / PgUp / PgDn / Home / End / 1~9 翻页，切页后选中第一个条目；Esc 按后台
  运行开关隐藏或最小化。
- 条目字母按**槽位下标**分配（`itemKeyKeys` 依次 QWERTYUIOPASDFGHJKLZXCVBNM），空白也占字母，
  超过 26 个槽位没有字母；字母只选择、不启动。
- Tooltip：搜索/设置显示 `搜索 (Ctrl+F)`、`设置 (Ctrl+,)`，加速键是字面量不进文案；条目只显示
  名称。两个快捷键都由 `OnWindowShortcut` 注册，不能只靠 `c.Shortcut` 查询。

### 拖放

- 一个元素只接收**一种**拖动值：`Drop`/`DragOver` 在元素状态上只存一个 `accepts`，后一次覆盖
  前一次。页面内胶囊接收 `dragItem`，外层包装接收 `dragTab`，不要混在同一个元素上。
- 没有落点就落到当前页面末尾（整个 `page` 也接收 `dragItem`）；拖回原页面胶囊什么都不做。
- 插入线统一用 `insertLine` / `DrawOver` 画 2 DIP 强调色短线，不设 border（不占空间、不随圆角
  弯曲）；**正在拖动的元素自己不画线**，否则会带到跟着指针的预览上。
- 拖动期间所有页面都保留构建并叠放（`v.itemDragging`），否则指针停到另一页胶囊上切页时源元素
  被卸载，MyGo 会结束按下状态、留下陈旧拖动。**除了当前页，这些页面都挂 `Invisible()`**：它们
  只为保住拖动，绝不能画出来——开了背景材质时当前页本身是透明的，盖不住下面的页面，别的页面的
  条目就会从材质里透出来。跟着指针的那份预览由 MyGo 自己从被拖的格子画，不经页面，所以不受影响。
- 原生文件拖入：每个条目用左右两个透明绝对定位盒子（`fileDropHalf`，带固定 `Key`）区分插在前
  还是后；卡片里子元素数量一变就会让两半的 ID 错位。调用 `DroppedFiles()` / `FileDragOver()`
  才算注册成放置目标。

### 图标

`mygo.Shell.FileIcon(path, px)` 写缓存、`ui.DecodeImage` 读成图片；没有手工提取图标的路径。
指向 `cached-icons/` 的算自动图标（`isAutoIcon`）：更新时从 `link` / `path` 重画；自定义图标
在 `custom-icons/` 里则不动。

- 添加时先试 `contentIcon`：图片（含 SVG）按 `normalizeIconImage` 缩小、`.ico` 用
  `extractIconResource`，都存进 cached-icons；取不到才 `writeIcon`。
- 快捷方式图标取自 `.lnk` 本身（Explorer 画的那张带徽标的），取不到才退回目标程序；`.lnk`
  记在条目的 `link` 字段。`.url` 添加时解析 `URL=`，条目直接打开网址。
- 只有 `custom-icons/` 和 `cached-icons/` 里的图标会显示（`allowedIcon`），外部路径保存时会被
  复制进来；删除只在两个目录里删，绝不删用户原件。`pruneIcons` 清理无人引用的文件。
- 回收站这类状态图标放内存里的 `App.dynamicIcons`，不写缓存；用户换自定义图标时清掉。
- 图标选择器：`PickIconDlg`（shell32 未公开导出）弹出 Windows 自己的「更改图标」，返回文件和
  序号；返回的 `%SystemRoot%` 这类路径原样存，读之前 `expandPath` 展开。
- 自定义背景 `customBackground` 按显示器尺寸缩到最长边 1280~2560，不透明存 JPEG(90)、带透明
  存 PNG；`drawBackground` 按**整窗** cover 算源矩形，顶栏和内容各画自己那块，所以是一张连续
  的图。

### 特殊条目与 shell 操作

- `src/special_items_*.go` 的注册表描述「不只是启动」的条目（目前只有回收站）：`Menu` 加额外
  右键项，`RefreshIcon` 给新的状态图标。回收站空/满由 `SHQueryRecycleBin` 判断，清空走自己的
  确认框 + `SHEmptyRecycleBin`；只在窗口可见且该页是当前页时刷新（显示/聚焦/切页立即一次，
  之后 5 秒轮询）。
- 「属性」和「系统菜单」对所有文件和 shell 位置可用（UWP 与网址除外）：属性用
  `ShellExecuteEx` + `SEE_MASK_INVOKEIDLIST`；系统菜单用 `IShellFolder::GetUIObjectOf`
  + `TrackPopupMenuEx` + `InvokeCommand`，会阻塞主线程直到关闭，属预期。`IContextMenu2/3`
  的 owner-draw 没处理，个别扩展项可能与资源管理器不同。

### 应用图标

`src/app_icon.go` 用 `resources/` 里的四张图（浅色底、暗色底、强调色遮罩、齿轮遮罩）按当前
强调色合成主窗口 256px、托盘 64px 的图标，设置窗口叠齿轮。更新时机：启动、强调色变化（180ms
去抖）、主题切换、系统外观变化、打开设置窗口。改了 `icon.svg` 要跑
`docs/generate-icon-assets.ps1` 和 `generate-icon.ps1`。

### 绘制与内存

`main()` 在开窗前按 `ui-state.json` 的 `gpu` 设 `MYGO_GPU`（默认 `0`，内存绘制；显式环境变量
优先），切换后要新进程才生效，所以设置里一动开关就 `relaunchApp()`（`src/view_settings.go` 的
包级变量，测试里替换成计数器，同背景材质）。这个窗口小、变化少，软件绘制可接受：常驻约 40MB，
GPU 约 86MB。窗口离屏（关到托盘、自动隐藏）时 `releaseMemory` 调 `debug.FreeOSMemory`。

### 多语言

- 文案在 `locales/en.json`（源语言）和 `locales/zh-Hans.json`，改完跑 `go tool mygo generate`
  生成 `locales/catalog_gen.go`（**不要手改**）。
- **帧内文案走 `v.msgs()`**，不要直接调包级 `locales.X()`——那读的是应用语言。无头
  `ui.Tester` 的语言是 `""`（英语），测试才能稳定断言英文。应用层提示、对话框标题和错误文字
  用包级函数。
- 语言偏好是 `ui-state.json` 的 `language`（`auto` / `en` / `zh-Hans`），启动时
  `applyLanguage` 调一次 `mygo.App.SetLocale`；`MYGO_LOCALE` 设置时优先。
- 运行时长不翻译：`formatRuntime` 直接写 `1h30m`，没有时长返回空串。
- **语义相同的文案共用一个键**，别为不同位置各加一条；只有中文同字、英文语义不同的才分开
  （`path`/`paths`、`icon`/`icons`）。没人用的键删掉。

## 设置窗口

- `src/view_settings.go`：`openSettings` 首次创建、之后只显示并聚焦；独立窗口 780×640（最小
  640×420），`StateKey: "settings-sections"`，无最大/最小化，标题「设置 - <标题>」。
- 左侧 180 DIP 导航 + 右侧内容页，当前页存在 `view.settingsPage`。`app.win` 为 nil 时
  `openSettings` 返回，测试用 `ui.NewTester(v.settingsView, 780, 640)` + `openSettingsPage`。
- `App.refresh` 同时刷新主窗口和设置窗口。页面：外观（第一张卡不设小标题：语言、主题、强调色、
  背景材质、窗口透明度、背景与背景透明度；第二张小标题「自定义」：图标大小、程序图标、窗口标题）、
  行为（启动与窗口：开机自启动、快捷键、窗口位置、置顶；操作：启动条目后最小化、滚轮切页、
  计时器；后台与性能：GPU 绘制、关闭后在托盘运行、不在任务栏显示）、数据（路径与数据两组）、
  关于（图标、版本、GitHub、更新、重启、退出）。
  GPU 改动后直接重启应用，不提示「重启后生效」。

## 启动参数与窗口位置

- `src/launch_args.go` 解析 `--silent`（静默）和 `--position last|cursor|center`（本次启动的
  位置，空格与 `=` 都行）；未知参数忽略，`--silent` 压过位置，不打印任何内容。
- `ui-state.json` 的 `window_position`（`last` / `cursor` / `center`，默认 `cursor`）决定窗口
  每次被唤出时开在哪里：上次位置、指针中心，或**鼠标所在那块屏幕**的中央。启动、托盘点击与
  托盘「打开」、全局快捷键都走 `showMainAtPreference`；`--position` 压过它。
- 「上次位置」之外的两种先 `Hidden` 再在 `OnReadyToShow` 里 `showMainAt` 定位显示，避免在旧
  位置闪一下。定位是 `src/window_place.go` 的纯函数（`placeWindowAtCursor` /
  `placeWindowInCenter`），窗口比工作区大时贴左上角。
- 第二次启动由 `src/singleton_windows.go` 按安装目录互斥认出，直接 `SetWindowPos` 把已有窗口
  摆到 `--position` 指定的位置（未指定为指针处，`last` 不动）并聚焦，不需要进程间通信；
  `--silent` 与登录时的启动只退出。

## 全局快捷键

`ui-state.json` 的 `shortcut` 由 `src/shortcut.go` 在 `WhenReady` 里注册，改变时先解绑再注册，
失败则保留旧值并提示。录制用只读 `TextInput` + `OnTextKey`：普通键必须带 Ctrl/Alt/Super，
F1–F12 可单独用。按下时主窗口可见且聚焦就隐藏，否则 `showMainAtPreference`。测试要把
`registerShortcut` / `unregisterShortcut` 换成假函数——真注册需要 message loop。

## 开机自启动

设置 → 行为的开关调用 mygo 的 `App.SetOpenAtLogin`，Windows 上写用户的 Run 项，值名是
`mygo.json` 的 identifier（`com.canwdev.golauncher`），命令带 `--mygo-opened-at-login`。
开关状态不另存：打开设置窗口时用 `App.OpenAtLogin()` 读真实状态（任务管理器「启动」里的停用
会如实反映），`src/open_at_login.go` 的 `setOpenAtLogin` / `openAtLogin` 在测试里替换。

`--silent` 与登录启动都不显示窗口：`main` 用 `App.WasOpenedAtLogin()` 认出登录启动，窗口以
`Hidden` 创建，**不能注册 `OnReadyToShow`**——MyGo 对 `Hidden` 窗口也会触发它，在回调里
`showMainAt` 会把窗口弹出来。静默的二次启动（`--silent` 或登录）只退出，其他二次启动按
`--position` / 指针摆放已有窗口，所以窗口藏在后台也能唤出。mygo 在包初始化时就把登录参数从
`os.Args` 移除。卸载器只删除指向 `$INSTDIR` 的 Run 项，便携副本的注册项保留。

## 自动更新

`src/updates.go` 的 `useUpdater` 在 `App.Run` 前启用 MyGo 的原生更新插件；设置 → 关于 只在
`app.updates` 为真时显示更新项，测试用 `fakeUpdater` 替换包级函数（没启用时直接调用会 panic）。
启动约 10 秒后检查一次、之后每天一次，状态存在 MyGo 用户目录的 `updater.json`。

发布：推 `v<version>` 标签触发 CI 的 draft Release，两种架构各生成签名更新包，
`CHANGELOG.md` 必须有 `## <version>` 小节。流程与密钥见 `docs/UPDATES.md`。

## 托盘与后台运行

- 开关是 store 的 `background_on_close`（默认开）。关掉后不创建托盘，关窗即退出。
- `src/tray.go` 的 `syncTray` 按开关创建/销毁托盘；`handleClose` 在开关打开且不在退出流程时
  `PreventDefault` + `Hide`，否则关窗即退出并清掉残留托盘与设置窗口。关掉开关时若窗口正隐藏，
  先 `showMain` 再移除托盘。
- 「不在任务栏显示」（`tray_only`）用 `SkipTaskbar` + `DisableMinimize`，依赖托盘存在，所以
  后台关闭时禁用；它只去掉任务栏按钮，**Alt+Tab 里仍有窗口**。
- 托盘悬停提示只显示窗口标题，不带安装路径；`findMainWindow` 因此不按标题找窗口。

## 数据

- 都在 `mygo-launcher-data/`，位置由 `src/data_root.go` 决定：安装版在
  `%LOCALAPPDATA%\MyGo Launcher\`（按卸载项的 `InstallLocation` 判断），便携版和开发构建在 exe
  旁。旧版留在安装目录的数据由 `migrateDataDir` 首次启动迁走，卸载器默认保留用户数据。
- 内容：`go-launcher-data.json`（store：`apps` / `categories` / `settings`）、`ui-state.json`
  （窗口偏好）、`window-state.json`（两个窗口的几何）、`cached-icons/`、`custom-icons/`。
- store 只经 `App.save` → `writeStoreAtomic` 写入并记录指纹，监听器（`src/store_watch.go`）
  据此区分自己的写入和手工编辑；改到一半的非法 JSON 会被忽略，不当成空 store。
- 条目路径相对 `absBase`（除非开了「新条目使用绝对路径」），图标路径**始终**相对数据文件夹，
  `convertItemPaths` 不转换图标路径。
- 「转为便携」（`toStoredPath`）：数据目录内是普通相对路径，落在已知用户目录时用最长匹配的
  `%APPDATA%` 这类变量，其余相对 `absBase`。`expandPath` 展开变量，字段里保留原样。
- 启动 `.exe` 用 `CreateProcessW` + `CreateEnvironmentBlock` 取当前环境，所以用户之后改的环境
  变量对新进程立即生效；`.bat` / 提权 / shell 目标仍走 `ShellExecuteEx`。
- 计时器默认关闭，只在开启时跟踪可执行程序（`startTracked`）；关闭时清空 `running`、不结束
  进程、不画时长。`timeableItem` 按扩展名判断，不读磁盘。
- 可用性检查把缺失条目记在内存 `App.missing`（不写 store），网格据此置灰。

## 从 CLaunch 迁移

`src/claunch.go` 只读 `CLaunch.ini`（UTF-16LE 或 UTF-8）：`[Page###]` 变成页面，`[SubMenu###]`
在父按钮位置展开，`Type` 映射见 `docs/MIGRATION.md`；`IconIndex` 无法还原。shell 目标
（`ms-settings:`、`shell:AppsFolder\…`、`::{GUID}`）由 `isShellTarget` 识别，路径函数不改写
它们。入口在设置 → 数据，文件对话框在 goroutine 上跑。

## 测试界面

`src/view_test.go` 用 `ui.NewTester` 无头驱动真实视图：

- 先 `tt.SetFocused(true)`，否则第一次点击只聚焦。
- 弹出菜单画在窗口内，按普通元素点击；`Tester.Menu()` 只用于原生系统菜单。
- 拖动用 `Press` / `Move` / `Release`；`tt.Type` 是追加；`later` 排的动作需要一帧。
- 设置窗口单独测（`newTestSettings` + `openSettingsPage`）。不要在测试里点主题，`SetSource`
  会阻塞；强调色可以点色板上的成员名。
- 更新插件、全局快捷键、`openEntryFile` 都要在测试里替换。
- `newTestApp` 把 `absBase` 指向临时目录，测试不碰真实数据。
- 测试默认英语，要测中文用 `tt.SetLocale("zh-Hans")`，不要点语言控件（会落盘）。

## 变更日志

`CHANGELOG.md` 只记用户能感知的变化，一事一条，最新在上；分组按需出现
（`破坏性变更` / `界面` / `功能` / `修复` / `工程`），`工程` 只放影响构建、打包、测试的内容，
内部重构不记。每条以粗体标签开头（`- **运行时**：……`），不列文件名。版本号要在 `mygo.json`
的 `version` 和 `src/version.go` 的 `appVersion` 两处一致。

## Git

提交信息不要添加 `Co-authored-by` trailer，提交只属于仓库作者本人。

## 目录结构

| 路径 | 作用 |
| --- | --- |
| `mygo.json` | 应用名、标识、版本、入口包 `./src`、输出目录、更新公钥 |
| `src/main.go` | 单实例、`WindowOptions`、`ui.View(v.view)`、文件拖入、store 监听 |
| `src/app.go` | `App`：store、运行中的进程、启动/停止、图标、toast、`dispatch`、设置窗口 |
| `src/store.go` / `src/store_watch.go` | 数据文件与结构、原子写入、文件监听与重载 |
| `src/theme.go` / `src/ui_kit.go` | 调色板与强调色；共享控件（图标按钮、分段控件） |
| `src/view*.go` | 界面各块：入口、顶栏、页面胶囊、网格、条目、键盘、对话框、搜索、文件夹面板、设置、外观、添加面板 |
| `src/items.go` / `src/commands.go` | 页面与条目操作；菜单和按钮实际执行的动作 |
| `src/background.go` / `src/view_background.go` | 背景图归一化、解码缓存与整窗绘制 |
| `src/icons*.go` / `src/icon_extract_*.go` / `src/icon_pick_*.go` | 条目与窗口图标、图标资源提取、系统图标选择器 |
| `src/app_icon.go` | 按强调色与主题合成窗口和托盘图标 |
| `src/special_items_*.go` / `src/recycle_bin_windows.go` | 特殊条目注册表；回收站状态、清空与图标刷新 |
| `src/shell_actions_*.go` | 文件与 shell 位置的「属性」和原生系统菜单 |
| `src/running_*.go` / `src/system_locations_*.go` | 枚举运行中的窗口；系统位置列表 |
| `src/tray.go` | 托盘图标、菜单，以及关闭窗口时隐藏到托盘的拦截 |
| `src/ui_state.go` / `src/window_state_store.go` | `ui-state.json` 偏好；`window-state.json` |
| `src/window_place.go` / `src/window_opacity.go` | 窗口位置（指针、屏幕中央、上次）与窗口透明度 |
| `src/launch_*.go` / `src/lnk_*.go` / `src/singleton_*.go` / `src/open_dialog_*.go` | 进程启动与提权、`.lnk` / `.url` 解析、按安装目录单实例、添加文件的打开对话框 |
| `src/claunch.go` / `src/utils.go` / `src/version.go` | CLaunch 迁移；路径与时长辅助；`appVersion` |
| `src/updates.go` / `src/shortcut.go` / `src/data_root*.go` | 更新插件、全局快捷键、数据目录选择与迁移 |
| `installer/installer.nsh` | 安装器钩子：搬走旧版残留数据，卸载时询问是否删除 |
| `locales/` | `en.json` / `zh-Hans.json` 与生成的 `catalog_gen.go` |
| `docs/` | `ICON.md`、`UPDATES.md`、`MIGRATION.md`、`TOUCH.md`、`TODO.md`、截图与图标脚本 |
| `resources/` | 应用图标源文件，`resources.go` 嵌入二进制供托盘使用 |

没有前端：新增一个操作意味着在 `view` 或 `App` 上加方法，再从菜单或按钮调用。
