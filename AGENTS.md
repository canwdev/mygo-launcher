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
  `docs/` 下的其他文件都如此。命令、标识符、API 名称保持原文。唯一例外是 `README-en.md`：
  它是 `README.md` 的英文版，改 `README.md` 时要同步翻译；两份文件的标题下都有语言切换链接。
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

CI 发布 Windows 10/11 的 x64 与 arm64（`go tool mygo build -platform windows/amd64,windows/arm64`）。
Windows 386 虽能编译，但运行时会退出（MyGo 的 `windows/386 is not supported yet`）；Go 1.27
也不再支持 Windows 7/8。UPX 压缩曾在 `db2c8ec` 试过，因常驻内存从约 50MB 升到约 90MB、
且要给 MyGo 固定提交打补丁，已弃用，见 `docs/UPDATES.md`。

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
- 顶栏从左到右：横向页面胶囊与「新建页面」按钮、搜索、设置。没有品牌、分割线或大标题；
  页面多时横向滚动。
- 页面胶囊悬停 200ms 会自动切换（`hoverTab`/`hoverSince`）。页面多时用 `ScrollHorizontal` 的
  原生横向滚动条滚动；点击或新建页面后，`view()` 末尾按 `tabRects` 把当前胶囊滚到中间
  （`centerNav`）。滚轮不做特殊处理。
- 内容区直接是网格；右下角是圆形的添加 FAB（只有 `+` 图标），点击后在它上方打开一个紧凑菜单：
  「运行中的窗口」「系统位置」各带右箭头，点击后在菜单左侧弹出二级面板（`AttachTo` 到该项，
  窗口放不下时 MyGo 自动翻到另一侧），下面仍是「选择文件 / 手动创建」。两个二级列表共用
  `pickerSubmenu` / `pickerRow`：滚动、图标（`pickerIcons`）和点击添加走同一套逻辑。
  「运行中的窗口」在每次打开这个二级列表时重新枚举（切换列表、关闭菜单再打开也一样），
  每个程序只列一条，取窗口所属的可执行文件；「系统位置」列出回收站、此电脑、
  控制面板、用户文件夹、网络、启动、最近使用，以及桌面、下载、文档、图片、音乐、视频
  （`src/system_locations_*.go` 的 CLSID / `shell:` 解析路径，作为 shell 目标加入，不计时）。
  已经在当前页面的系统位置带勾，点击它定位到已有条目而不是重复添加。
  点是遮罩加锚定在 FAB 上的面板（`src/view_window_picker.go`），点外部、按 Escape 或选中
  一项后关闭。FAB 绝对定位、悬浮在内容之上：网格用 `ui.Scroll` + 内容网格，内容从 FAB 下面滚过，
  滚动内容末尾留 68 DIP 保证滚到底时最后一行仍在 FAB 上方；网格边距是 `8, 8, 68, 8`。
  不要把这段留白放回滚动容器，那会让内容提前被裁掉，留下一块空白。
- 网格单元之间没有间隙，也不画卡片底色，只在悬停时显示圆角背景；图标与名称之间留 6 DIP：
  `gridBody` 用 `ui.Grid`（`Gap(0)`）按窗口宽度算出列数，因此不需要 `GridView` 和紧凑主题。
- 网格单元左上角的时长默认不画，悬停该单元（或进程运行中）才出现，计时器关闭时始终不画；
  时长字段未运行时的 Tooltip 复用设置里的「计时器」（`AutoTiming`），运行中才换成停止提示；
  条目还没有时长时只画计时图标，不画占位的「--」。
  计时字段在文件拖放的两个半区之后构建（两个半区盖住整个单元），这样点击落在计时上而不是
  半区；它自己也注册成「插在单元前」的文件放置区，并始终构建以保证半区的元素 ID 稳定。
- 图标与单元格大小由 `ui-state.json` 的 `icon_size`（`small` / `medium` / `large`，默认
  `medium`）决定，`src/icon_size.go` 给出三档尺寸。
- **菜单只通过右键打开**：网格单元、空白和页面胶囊都没有 ⋯ 按钮，各自用
  `src/view_menu.go` 的 `contextMenu` 挂上 `ui.ContextMenuBase` 自绘菜单，菜单项是带 SVG
  图标的行；键盘导航、Esc 和点外部关闭仍由 MyGo 的菜单负责。每个菜单的打开状态按元素键
  （页面与槽位）存进 view 的 `menus`。唯一的例外是文件夹条目的面板：它不是菜单，而是「打开
  文件夹」，所以单击就展开。
- 条目的右键菜单（`itemMenu`）始终有「插入空白」。菜单项不预先访问磁盘判断可用性：文件或
  文件夹不存在或为空时，点「打开所在文件夹」或「更新图标」会由 `App.reveal` /
  `App.updateIcon` 返回 `PathMissing` 并显示提示。
- 网格单元单击即启动，可以保留手型光标。**文件夹条目例外**：单击弹出
  `src/view_folder.go` 的面板，列出目录内容；Windows 上隐藏和系统属性的条目（`desktop.ini`、
  `Thumbs.db` 等）不列出，按 `folder_entries_windows.go` 的 `skipFolderEntry` 判断，其他平台
  按文件名和点开头过滤；条目里的文件夹进入下一层（`folderRoot` 为界，
  可以用「上级文件夹」返回），文件用系统默认程序打开后关闭；面板条目右键弹出系统原生菜单，
  菜单关闭后重读当前目录，重命名或删除会立即反映；根菜单的标题左侧画条目的文件夹
  图标（`folderTitleIcon`，没有图标时用 `iconFolder`），进入子目录后换成「上级文件夹」按钮。
  面板是 `ui.Overlay` + `Modal`
  背板，以点击位置（`folderAtX/Y`，由 `PointerPosition` + `LocalToWindow` 取）为中心、
  `Absolute().Left/Top` 定位，四边留 8 DIP，超出窗口就收进窗口；行数按窗口高度算，所以
  内容多时列表滚动而不是撑出窗口。点外面或按 Escape 关闭；`folderAnchored` 记录本帧是否
  构建了那个单元格，页面切换、条目被移除时面板随之关闭。目录读取在 goroutine 上（测试里
  同步），条目图标只对可见行用 `mygo.Shell.FileIcon` 异步取，没有窗口时退回画好的文件夹/文件
  图标。面板打开期间 `wheelInput` 直接返回，滚轮不会切页。
- 对话框内容宽度用 `dialogWidth(c, preferred)` 按窗口宽度收缩（`ui.Modal` 的外边距和 padding
  共留 20 个 spacing），窄窗口下输入框和按钮不会越界；高度仍由 `dialogBodyMax` 控制，它只给
  标题、按钮、间距和对话框边距留位置，小窗口尽量把字段展示完全，仍放不下的字段在对话框内
  滚动。
- `page` 始终挂 `HandleInput(wheelInput)`：「滚轮切换页面」（`ui-state.json` 的
  `wheel_pages`，默认开启）时，普通竖直滚轮被吃掉并 `switchPage` 循环切换上一/下一页，网格
  不再滚动；按住 Shift 则手动滚动网格（`scrollPage`）。关闭后普通滚轮照常滚动，Shift+滚轮
  改为切页。一次滚轮事件最多切一页（超出阈值也重置 `wheelSum`，不连续切多页）；鼠标停在
  页面胶囊上滚轮时，用 `tabAt` 按胶囊矩形找到光标下的页面（当前选中的胶囊也要算），记下
  `hoverIgnoredTab`，它的 200ms 悬停自动切换被忽略，直到指针移到另一个胶囊。

### 键盘与 Tooltip

- 主界面的按键由 `src/view_keyboard.go` 用 `Context.OnWindowShortcut` 声明：不依赖焦点，
  浮层和对话框优先，模态打开时根视图上的快捷键不注册。`overlayOpen()` 再按菜单、文件夹
  面板、添加菜单、搜索和对话框的状态兜底，浮层打开时只保留 `Ctrl+F` 和 `Ctrl+,`。
- 选中项是槽位下标：`view.keySel` 与 `keySelTab` 一起记住，空白格不参与。方向键按行列导航，
  在第一行/第一个条目向上或向左、最后一行/最后一个条目向下或向右时调用 `switchPageKey`
  翻页；选中格子用 `p.hover`，与鼠标悬停同一效果，并只在选中那一帧 `ScrollIntoView`。
- 悬停条目等同于选中：`hoverSlot` / `hoverSlotTab` 只在指针移到另一个条目时更新选中项，
  指针停着时方向键仍可离开该条目。鼠标停在页面胶囊上时用键盘切页，会把该胶囊记进
  `hoverIgnoredTab`，和滚轮切页一样避免悬停延迟把它切回去。
- `keyboardNav` 在方向键、字母和翻页键导航时置位：鼠标悬停不再画高亮、不再选择条目，
  悬停才出现的运行时长也一起藏起；`pageInput` 收到指针移动或按下时调用 `pointerBack`
  清除它，鼠标重新接管。
- `keyboardFocus` 为真时（键盘导航中且没有浮层），选中格子里会放一个 0×0、无焦点环的
  `key-focus` 盒子并让它获得焦点。焦点因此从之前点过的按钮（如设置齿轮）移开，回车由
  窗口快捷键交给选中条目，而不是再次点击那个按钮。
- 回车等同于单击选中条目（启动、打开文件夹面板、运行中则询问停止）；文件夹面板以选中格子
  的中心（`keySelX/Y`）为锚点。
- Tab 下一页（循环）、PgUp/PgDn 上一页/下一页、Home/End 第一页/最后一页、1~9 第 n 页；
  切页后选中新页面的第一个条目。Esc 在没有浮层时按后台运行开关隐藏或最小化。
- 条目字母按**槽位下标**分配，空白也占字母：`itemKeyKeys` 依次是
  `QWERTYUIOPASDFGHJKLZXCVBNM`，对应 0~25。字母只选择条目，不启动；超过 26 个槽位没有字母。
- Tooltip：顶栏搜索/设置显示 `搜索 (Ctrl+F)`、`设置 (Ctrl+,)`，加速键是字面量，不进文案；
  条目 Tooltip 只显示名称，不显示字母。`Ctrl+F` 搜索、`Ctrl+,` 打开设置都由
  `OnWindowShortcut` 注册，不再只靠 `c.Shortcut` 查询（后者需要别处先注册）。
- 测试在 `src/view_keyboard_test.go`：方向键/翻页/字母选中、回车打开文件夹、搜索打开时按键
  不落到主界面，以及悬停后的 Tooltip 文本。

### 从运行中的窗口添加

- `src/running_windows.go` 的 `listRunningWindows` 枚举可见的顶层窗口：跳过工具窗口、有 owner
  的弹窗、本进程、`TextInputHost` 等 shell 宿主和 cloaked 窗口；按程序去重，保留最上层窗口的
  标题。窗口标题只用于识别，条目名取 exe 版本资源的 `FileDescription`，取不到才用文件名。
- UWP 的框架窗口属于 `ApplicationFrameHost`：从它的子窗口找到应用进程，取
  `GetApplicationUserModelId` 的 AUMID，存成 `shell:AppsFolder\<AUMID>`，名称用窗口标题（如
  「计算器」）。这类条目不能计时或停止；普通 exe 条目照常计时、可停止。
- 选中后存 exe 路径和所在目录（shell 目标不填工作目录）；图标走 `writeIcon`，对
  `shell:AppsFolder\…` 同样有效。
- 选择面板在 `src/view_window_picker.go`：`ui.Overlay` 的整窗遮罩加锚定在 FAB 上的面板（MyGo 的
  `ui.Menu` 只能放文本项），点外部或 Escape 关闭。面板打开时在后台读取程序图标，无窗口（测试）
  时不读；测试替换包级 `listWindows`。
- 非 Windows 的 `src/running_other.go` 返回空列表，保证交叉编译。

### 拖放

- 在单元格或页面上调用 `Drag(value)`，指针移动几个 DIP 后开始拖动；另一侧的 `ui.Drop[T]`
  和 `ui.DragOver[T]` 接收。
- **一个元素只接收一种拖动值**：`Drop`/`DragOver` 在元素状态上只保存一个 `accepts` 闭包，
  后一次调用覆盖前一次。因此页面内部的胶囊接收 `dragItem`（拖入条目），外层包装元素接收
  `dragTab`（页面重排）——不要把两种类型放在同一个元素上。
- 值按类型匹配：条目是 `dragItem`，页面是 `dragTab`（`src/items.go`）。页面内槽位重排走
  `reorderSlots`，以槽位索引为准，包含空白。
- **没有落点就放到末尾**：整个 `page` 也接受 `dragItem`，在单元和页面胶囊都没接住时把
  条目追加到当前页面末尾；拖到条目本来所在的页面胶囊上放下则什么都不做（按住 Ctrl/Cmd 仍复制）。
- **插入线统一放置**：`insertLine(e, vertical, after, p)` 在网格单元和页面胶囊的左右边画线，
  用 `DrawOver` 画 2 DIP 的强调色短线，不设 border：线不占内容空间、不随圆角弯曲，图标和
  文字位置保持不动；页面胶囊接收条目时也用 `DrawOver` 描边，而不是 `Border`；
  `dropAfter(e, vertical)` 按指针在目标中心前/后决定落点，再经 `dropTo(from, at, copy)`
  换算成 `moveSlot` 的索引。拖动时目标显示线而不是整块边框，**正在拖动的卡片和胶囊自身不画
  线**（预览画的是源元素，画了会带到跟着指针的副本上，落回原处也没有意义）；页面胶囊重排、
  网格条目换位都用
  它；`dropDragged` 按 `dragItem.tab` 区分同页换位（`reorderSlots`）与跨页移动
  （`moveItemToTabAt`）。
- **拖动期间所有页面都保留构建并叠放**：`body` 用 `Key(tab.GUID)` 包住每页；`v.itemDragging`
  为真时其他页面也构建，先画（绝对定位铺满），当前页面最后画并带 `p.bg` 背景盖住它们。这样
  指针停在另一个页面胶囊上（`dragTabHoverDelay` = 300ms）切换页面时，拖动源元素没有被卸载、MyGo
  跟着指针的预览也继续画，可以直接落到新页面的某个位置；否则 MyGo 会结束按下状态，释放时不
  产生 drop，还会留下一个陈旧拖动。`v.itemDragging` 在 `view()` 末尾把本帧所见交给下一帧。
- **从资源管理器拖入文件**：整个 `page` 仍是兜底放置目标（追加到当前页面），拖到页面胶囊上由
  `tabChip` 的 `DroppedFiles()` 接收，加入该页面；文件停在同一胶囊上 300ms 也会切换页面。
  每个条目把左右两半做成透明绝对定位的小盒子，调用 `fileDropHalf` 注册
  `DroppedFiles()` / `FileDragOver()`：原生文件拖动只告诉落在哪个元素，不知道元素内的位置，
  两半用来区分插在条目前还是后，并画出插入线。两半带 `Key`（`file-before` / `file-after`），
  网格单元的运行时字段也始终构建、只按悬停 `Hidden`：原生 drop 按元素 ID 找目标，卡片里子元素
  数量一变就会让两半的 ID 错位，文件要么落空、要么落到另一半。调用 `DroppedFiles()` /
  `FileDragOver()` 才会注册为放置目标；文件拖动不画整窗遮罩，只用条目上的插入线。没有条目
  接收的文件落到页面本身（追加到末尾），最后仍由 `main.go` 的 `OnFileDrop` 兜底。

### 图标

`mygo.Shell.FileIcon(path, px)` 把 PNG 写进图标缓存，`ui.DecodeImage` 再读成
`ui.ImageSource`；没有手工提取图标的路径。图标指向 `mygo-launcher-data/cached-icons/` 的条目
视为自动生成（`isAutoIcon`），「更新图标」和批量更新据此生效。添加文件时先试
`contentIcon`：图片（含 SVG）按 `normalizeIconImage` 缩小后、`.ico` 按
`extractIconResource` 取出，存进 cached-icons（因此算自动图标），让条目显示图片本身而不是
文件类型图标；取不到才退回 `writeIcon`。「更新图标」和批量更新也先试 `contentIcon`，图片和
`.ico` 会重新生成同样的内容图标，其它程序仍走 `writeIcon`。快捷方式的图标取自 `.lnk`
本身（Explorer 画的带应用徽标的图标），取不到才退回目标程序：有些目标（如 `chrome_proxy.exe`）
只有空白图标。条目的 `link` 字段记住这个 `.lnk`，单条和批量更新图标都用它；编辑条目改了目标
路径时清空。shell 目标（`::{CLSID}`、`shell:`）没有文件，更新图标直接用 `writeIcon` 从解析名
重画，不做存在性检查。
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
  删除条目/页面和更换图标后删掉两个目录里无人引用的文件。回收站这类状态图标不写图标缓存：
  `App.dynamicIcons` 按条目 GUID 保存解码后的图片，显示时优先于持久化图标；换了自定义图标时
  清掉，让用户的选择生效。

- 设置 → 外观底部的「程序图标」通过 `App.pickAppIcon` 和 `App.customIcon(path, 0)` 选图：
  图片、`.ico`、`.exe`、`.dll`，不含 SVG（窗口和托盘只收 PNG）；主窗口、托盘和「关于」用
  原图，设置窗口由 `gearIcon` 在右下角叠齿轮。清除后回到强调色合成的图标。
- 自定义背景由 `App.customBackground` 按显示器像素尺寸缩到最长边 1280~2560（无窗口时 1920）：
  不透明图存质量为 90 的 JPEG，带透明才存 PNG。加载时 `loadBackgroundBitmap` 再用同一上限缩一次，
  手工放进 `custom-icons` 的大图也不会多占内存。
  `drawBackground` 按**整窗** cover 计算源矩形，顶栏和 `body` 的页面 Box 各画落在自己矩形里的
  那一块，所以顶栏与内容区是一张连续的图；有背景时顶栏底色改用 `p.bg`，避免接缝。拖动时
  多页仍各自画背景，当前页最后画并盖住其他页；设置窗口不受影响。

### 特殊条目与 shell 操作

- `src/special_items_*.go` 用 `specialItem` 注册表描述“不只是启动”的条目：`Match` 按路径匹配，
  `Menu` 生成额外右键项，`RefreshIcon` 在 shell 图标变化时给出新的缓存图标。目前只有回收站。
- 回收站：`SHQueryRecycleBin` 取空/满状态，菜单在有内容时提供「清空回收站」，先走应用自己的
  确认框，再用 `SHEmptyRecycleBin`（不显示 shell 自己的确认/进度）在 goroutine 上清空，完成后
  提示并刷新图标。空/满图标直接取 shell 的回收站图标，放在内存里的 `App.dynamicIcons`，不写
  cached-icons：状态图标变化频繁，只改显示、不改 store。刷新时机：窗口可见且回收站所在的页面
  是当前页面时，窗口显示/获得焦点/切到该页面立即查一次，之后每 5 秒轮询；窗口隐藏、最小化或
  其他页面不查询（`startSpecialPoll` / `refreshVisibleSpecials`）。清空后立即刷新。数量查询
  缓存 2 秒，避免菜单每帧查询 shell。
- 条目菜单：shell 目标（`shell:`、`::{CLSID}`、网址、UWP）没有可显示的文件夹，菜单里的
  「打开所在文件夹」换成「打开」；「系统菜单」上方有分割线，它下面不再加线；「属性」固定放在
  最下方；「更新图标」「转为绝对/相对路径」移到条目详情弹窗（编辑已有条目时显示，窄窗口下
  按钮自动换行）。
- 「属性」和「系统菜单」对所有文件、文件夹和 shell 位置可用，UWP（`shell:AppsFolder\…`）和
  网址除外（`canUseShellActions`）。属性用 `ShellExecuteEx` 的 `properties` 动词 +
  `SEE_MASK_INVOKEIDLIST`；系统菜单用 `SHParseDisplayName` + `SHBindToParent` +
  `IShellFolder::GetUIObjectOf(IID_IContextMenu)`，显示 `TrackPopupMenuEx` 并把选中的命令交给
  `IContextMenu::InvokeCommand`。它不在应用内绘制，依赖 `IContextMenu` 的默认菜单项；
  `IContextMenu2/3` 的 owner-draw 消息没有处理，个别扩展项的显示可能与资源管理器不同。
  系统菜单会阻塞主线程直到关闭，属于预期的模态操作；属性页由 shell 自己的窗口显示。

### 应用与托盘图标

- 运行中的窗口、任务栏按钮和托盘图标跟随强调色与明暗主题：`src/app_icon.go` 用
  `resources/icon-base.png`（浅色底与描边）、`icon-base-dark.png`（暗色底与描边）、
  `icon-accent.png`（强调色图形的 alpha 遮罩，箭头保留 16% 透明度）和
  `icon-gear.png`（设置窗口齿轮的白色遮罩）合成；遮罩按
  `accentColor(prefs.Accent, mygo.Theme.IsDark())` 上色，齿轮描边浅色底用深色、暗色底用
  浅色，不把半透明箭头压成实色。
- 窗口图标 256px、托盘 64px；设置窗口的图标在右下角叠齿轮并裁到圆角底色内。
  `updateAppIcons` 同时设置主窗口、设置窗口和托盘；同一张无齿轮图标只合成一次再缩放。
- 更新时机：启动后 `WhenReady`；强调色预设点击或取色器变化（180ms 去抖
  `scheduleAppIconUpdate`）；主题切换；`auto` 时系统外观变化（`mygo.Theme.OnUpdated`）；
  打开设置窗口时补上齿轮。exe、桌面/开始菜单快捷方式的图标仍是构建时的 `icon.png`。
- 图标资源由 `docs/generate-icon-assets.ps1` 从 `icon.svg` 生成（遮罩保留 fill-opacity），
  它在透明页面上用无头浏览器截图，截图的 alpha 直接使用；改了 SVG 后和
  `generate-icon.ps1` 一起跑。

### 绘制与内存

- `main()` 在创建窗口前读 `ui-state.json` 的 `gpu`（设置里的「GPU 绘制」，默认关闭）：关闭时
  把 `MYGO_GPU` 设为 `0`，让 MyGo 在内存里绘制，而不是建 GPU 设备、图集和纹理；开启时设为
  `1`。渲染器在窗口创建时选定，所以设置里切换后需重启才生效。显式设置 `MYGO_GPU` 环境变量时
  以环境变量为准。这个窗口小、变化少，软件绘制的开销可以接受，实测可见窗口的常驻内存约 40MB、
  隐藏到托盘后约 36MB；GPU 绘制约 86MB。
- 窗口离开屏幕时（关闭到托盘、自动隐藏触发的 `minimize`）调用 `releaseMemory`
  （`debug.FreeOSMemory`），把上一帧留下的堆尽快还给系统。

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
- **语义相同的文案共用一个键**，不要为不同位置各加一条：设置「数据目录」的按钮和条目菜单都用
  `revealInFolder`（「打开所在文件夹」），管理员图标的提示和选项都用 `runAsAdministrator`。
  只有中文恰好同字、英文语义不同的键才分开（`path` / `paths`、`icon` / `icons`、
  `updateIcon` / `updateIcons`、`pickFiles` / `selectFileTitle`）。没人使用的键直接删掉。

## 设置窗口

- `src/view_settings.go`：`openSettings` 首次创建设置窗口，之后只显示并聚焦；关闭时清空指针。
  设置窗口是一个独立的 `mygo.NewWindow`，560×720（最小 480×420），`StateKey: "settings"`，
  不提供最大/最小化，背景 `light-dark(#f4f4f6, #1c1c1f)`。
- `app.win` 为 nil 时 `openSettings` 直接返回，所以测试用
  `ui.NewTester(v.settingsView, 560, 720)` 单独驱动设置内容。
- `App.refresh` 同时刷新主窗口和设置窗口；设置里的改动立刻反映到启动器。
- 设置分组：外观（语言下拉框、主题分段控件、强调色色板（悬停提示成员名字）+ 打开取色器的
  显示当前颜色的圆形编辑按钮，色板下的
  `MyGo!!!!!` 是字面文字、不进文案、图标大小分段控件、程序图标、背景与背景透明度）、
  行为（全局快捷键、计时器、自动隐藏、
  窗口置顶、滚轮切换页面、GPU 绘制、关闭后在后台运行）、路径（新条目绝对路径、批量转换）、
  数据（刷新、打开数据目录（hint 显示实际路径）、更新图标、从 CLaunch 迁移）、关于（上色后的
  程序图标、名称与版本、GitHub、更新）。GPU 开关改动后提示重启生效。

## 启动参数

- `src/launch_args.go` 解析 `os.Args`：`--silent` 静默启动，`--at-cursor` 在光标处显示；
  两者同时出现时以 `--silent` 为准，其它参数忽略。
- 冷启动时 `WindowOptions.Hidden` 为真，`--at-cursor` 在 `OnReadyToShow` 里调用
  `showMainAtCursor()`，`--silent` 则不显示。静默启动时后台运行开启仍有托盘和全局快捷键；
  关闭后台运行就没有托盘，只能靠全局快捷键唤回。
- 第二实例仍由 `singleton_windows.go` 的按安装目录互斥认出：静默启动只退出、不打扰已有窗口；
  `--at-cursor` 用 `GetWindowPlacement` 取恢复尺寸，按 `GetCursorPos` / 显示器工作区
  调 `SetWindowPos` 移动已有窗口，不需要进程间通信。非 Windows 的 `activateExistingInstance`
  是空操作。
- 光标与工作区的进程内定位在 `src/window_place.go`：`placeWindowAtCursor` 是纯函数
  （测试在 `src/launch_args_test.go`），窗口比工作区大时贴工作区左上角。

## 全局快捷键

- `ui-state.json` 的 `shortcut` 保存 accelerator（如 `Ctrl+Alt+Space`），由
  `src/shortcut.go` 在 `WhenReady` 里用 `mygo.GlobalShortcut.Register` 注册；快捷键只在应用
  运行时有效。
- 改变时先解绑旧值再注册新值；注册失败（被占用、系统保留）保留旧快捷键，并用 toast 提示。
- 录制用聚焦的只读 `TextInput` + `OnTextKey` 取 `Key`/`Mods`：普通键必须包含 Ctrl、Alt 或
  Super，F1–F12 可以单独使用，Escape 取消，Backspace/Delete 清除；键名映射见
  `shortcutKeyNames`。
- 按下时主窗口可见且聚焦就隐藏（后台运行开启则隐藏到托盘，否则最小化），其余情况
  `showMainAtCursor()`：窗口按普通尺寸居中于光标，并夹在光标所在显示器的工作区内，随后
  显示并聚焦。托盘「打开」项仍用 `showMain()`，不改变窗口位置。
- 测试把 `registerShortcut` / `unregisterShortcut` 换成假函数：真正的注册需要应用的 message
  loop，无头测试里不能用。

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
  `MYGO_UPDATER_PRIVATE_KEY` 执行
  `go tool mygo build -platform windows/amd64,windows/arm64 -upload`，并为两种架构生成更新包。
  `CHANGELOG.md` 必须有 `## <version>` 小节，发布 draft 时勾选 latest。细节见
  `docs/UPDATES.md`。

## 托盘与后台运行

- 开关是 store `Settings` 的 `background_on_close`，默认开启；关掉后不创建托盘，关闭窗口
  就是退出。
- `src/tray.go`：`syncTray` 在开关打开时创建 `mygo.NewTray`（稳定 ID `main`，图标由
  `app_icon.go` 按当前强调色合成成 64px），关闭时 `Destroy`；语言变化或开关变化时重设菜单和
  悬停提示。托盘只在开关打开时存在，且一直显示。
- 托盘左键和右键菜单的「打开」恢复主窗口（`showMain`，最小化时先 `Restore` 再 `Show`，
  因为 `Show` 不会取消最小化），并显示当前全局快捷键；菜单还有「设置」（`openSettings`）和
  「退出」（`App.Quit`）。
- 主窗口标题只有 `appName`；安装目录放在托盘悬停提示 `instanceTitle(installDir())` 里。因为
  标题不再唯一，`singleton_windows.go` 的 `findMainWindow` 按「标题 = appName + 进程 exe 所在
  目录 = 本安装目录」枚举窗口，避免聚焦到另一个目录的副本。
- 主窗口的 `OnClose` 交给 `view.handleClose`：开关打开且不在退出流程（`App.quitting`）时
  `PreventDefault` 并 `Hide`；否则关闭就是退出，同时销毁可能残留的托盘、关闭设置窗口，保证
  没有窗口让进程留在后台。`OnBeforeQuit` 在退出开始时置位 `quitting`，所以托盘的「退出」不会
  被拦下；`OnQuit` 销毁托盘。
- 关掉开关时若主窗口正隐藏，先用 `showMain` 显示再移除托盘，否则窗口没有入口。
- 测试里 `syncTray` 因为没有窗口（`app.win == nil`）直接返回：`mygo.NewTray` 要求应用
  就绪；测试覆盖设置持久化和 `handleClose` 的判定。

## 数据

- 所有数据都在 `mygo-launcher-data/`，文件夹由 `src/data_root.go` 的 `dataBase()` 决定：
  - 安装版放在 `%LOCALAPPDATA%\MyGo Launcher\`：启动时读卸载项
    （`HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\com.canwdev.golauncher`）的
    `InstallLocation`，等于 exe 所在目录才算安装版；旧版本留在安装目录里的数据由
    `migrateDataDir` 首次启动时迁过来，绝对图标路径一并改写。
  - 便携版和开发构建放在 exe 旁（开发时仍是工作目录）。`absBase` 仍是条目相对路径的基准，
    和 `dataBase()` 分开。
  - 卸载器只删 `$INSTDIR`；`installer/installer.nsh` 的 `customUnInstall` 先把旧版残留数据
    搬到用户目录，再询问是否删除 `%LOCALAPPDATA%\MyGo Launcher`，默认保留，`/S` 静默卸载
    也保留。
- 目录里是：
  - `go-launcher-data.json` — store：`apps`、`categories`（页面，`slots` 存条目 guid，`null`
    表示空白）和 `settings`（`auto_time` / `auto_hide` / `always_on_top` /
    `absolute_paths` / `background_on_close`）
  - `ui-state.json` — 窗口自身偏好：语言、主题、强调色、当前页面、GPU 绘制、滚轮切换页面、
    全局快捷键、自定义程序图标、背景图与背景透明度
  - `window-state.json` — 两个窗口的几何信息，通过 MyGo 的 `WindowStateStore`
  - `cached-icons/` — 自动生成的图标（`writeIcon`，以及 `contentIcon` 从图片 / `.ico` 内容
    生成的图标）
  - `custom-icons/` — 用户在条目编辑器里选择的图标、自定义程序图标和启动器背景，按
    `<名称>-<时间戳>` 保存；`pruneIcons` 除了条目图标，还要保护 `ui-state.json` 里的这两个
    路径
- store 只通过 `App.save` → `writeStoreAtomic` 写入，并记录文件指纹（mtime、大小、内容哈希），
  让监听器（`src/store_watch.go`，`mygo.WatchFiles`）区分应用自己的写入和手工编辑。手工改到
  一半（非法 JSON）的文件会被忽略，不会被当作空 store。
- 条目路径相对于 `absBase`（开发时是工作目录，构建版是 exe 所在目录），除非「新条目使用绝对
  路径」；`convertItemPaths` 只转换条目路径、工作目录和快捷方式路径，**不转换图标路径**：图标
  始终相对于数据文件夹，迁移后仍能显示。
- 可执行程序只在计时器（`auto_time`）开启时被跟踪（`startTracked`）：退出时把时长加到
  `runtimeStats`，运行中也能停止。关闭计时器时 `App.stopTiming` 立即清空 `running`（不结束
  进程）、把已跟踪进程标成不再计时、停止时钟重绘，并关闭运行时长对话框；网格里的时长整个
  不画，程序可以重复启动，不再弹停止确认。重新开启只跟踪之后启动的程序。
  运行时对话框的 `-1h` / `-30m` / `+30m` / `+1h` 只改待保存的分钟数（`adjustRuntime`），
  点保存后才通过 `setRuntimeMs` 写入。提示是「部分程序无法自动计时」。
- **只有程序计时**：`timeableItem`（`src/utils.go`）按扩展名判断 `Path` 是否为 `.exe` / `.bat` /
  `.cmd` / `.com` 且不是 shell 目标，不读磁盘（文件暂时不存在时仍显示时长）。文件夹、文件、
  网址和 shell 目标打开后没有可跟踪的进程，格子里不画计时字段，`openRuntime` 也直接返回；
  它们原有的 `runtime_ms` 保留在数据里，只是不显示。

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
- **全局快捷键在测试里要替换**：`registerShortcut` / `unregisterShortcut` 换成假函数；录制用
  `tt.Click("Not set")` 后 `tt.Key(ui.Ctrl|ui.Alt, ui.KeySpace)`，设置行的按钮会显示新值。
- 测试不能碰真实数据目录：`newTestApp` 把 `absBase` 指向临时目录。
- **文件夹面板在临时目录里测**（`src/view_folder_test.go`）：在 `absBase` 下建目录和文件，
  点条目展开、点子目录进入、点「上级文件夹」返回；无窗口时目录同步读取、不取 shell 图标。
  点面板里的文件会调用 `openEntryFile`，测试替换它，不真的启动外部程序。
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
| `src/view.go` | 帧入口：调色板、页面结构、网格主体、添加 FAB、忙碌条、`later`/`refresh` |
| `src/view_header.go` | 顶栏：页面胶囊、新建页面、搜索、设置 |
| `src/view_tabs.go` | 页面胶囊的行为：右键菜单、条目拖入、页面重排 |
| `src/view_settings.go` | 设置窗口及其内容（齿轮打开） |
| `src/view_grid.go` / `src/view_item.go` | 网格单元格与拖放；条目图标、时长等共用部分 |
| `src/view_keyboard.go` | 主界面键盘：选中、翻页、回车、Esc、字母选中与窗口快捷键注册 |
| `src/background.go` / `src/view_background.go` | 自定义背景归一化、解码缓存与整窗绘制 |
| `src/view_appearance.go` | 设置里的程序图标和背景选择/清除 |
| `src/view_window_picker.go` | 添加 FAB 的选择窗口：运行中的程序、系统位置与两个添加入口 |
| `src/system_locations_*.go` | 系统位置列表：回收站、此电脑、控制面板、用户文件夹、网络、启动、最近使用和常用文件夹的 shell 路径 |
| `src/special_items_*.go` | 特殊条目注册表：回收站等条目的额外菜单和状态图标 |
| `src/recycle_bin_windows.go` | 回收站查询、清空和空/满图标刷新 |
| `src/shell_actions_*.go` | 文件与 shell 位置的「属性」和原生系统菜单 |
| `src/running_*.go` | 枚举运行中的窗口、取程序路径与显示名（Windows）；其他平台为空 |
| `src/view_dialogs.go` | 条目、运行时、重命名和确认对话框 |
| `src/view_search.go` | 搜索浮层：过滤、键盘操作、在页面中定位 |
| `src/view_folder.go` | 文件夹条目单击展开的面板：列出目录、进入子目录、打开文件 |
| `src/items.go` / `src/commands.go` | 页面与条目操作；菜单和按钮实际执行的动作 |
| `src/icons.go` / `src/icons_svg.go` | 条目图标与窗口绘制的 SVG |
| `src/icon_extract_*.go` | 从 `.ico` / `.exe` / `.dll` 的图标资源提取 PNG |
| `src/tray.go` | 托盘图标、菜单，以及关闭窗口时隐藏到托盘的拦截 |
| `src/ui_state.go` / `src/window_state_store.go` | `ui-state.json` 偏好；`window-state.json` |
| `src/launch_*.go` / `src/lnk_*.go` / `src/singleton_*.go` | 进程启动与提权、`.lnk` 解析（COM）、按安装目录的单实例 |
| `src/launch_args.go` / `src/window_place.go` | 启动参数（`--silent` / `--at-cursor`）与窗口在光标处的定位 |
| `src/open_dialog_*.go` | 添加文件的打开对话框；Windows 上保留 `.lnk`，其他平台用 MyGo 的 |
| `src/claunch.go` | CLaunch.ini 解析与迁移：解码、分节、Type 映射、导入到 store |
| `src/utils.go` | `formatRuntime`、路径规范化、`splitArgs`、`isExecutable`、`isShellTarget` |
| `src/version.go` | `appVersion`，与 `mygo.json` 保持同步 |
| `src/updates.go` | updater 插件、更新偏好函数与测试替换点 |
| `src/shortcut.go` | 全局快捷键的注册、录制、键名映射与窗口显隐 |
| `src/data_root*.go` | 数据目录选择（安装版 `%LOCALAPPDATA%`、便携版 exe 旁）、旧数据迁移 |
| `installer/installer.nsh` | 安装器钩子：搬走旧版残留数据，卸载时询问是否删除数据 |
| `locales/` | 界面文案 `en.json` / `zh-Hans.json` 与生成的 `catalog_gen.go` |
| `docs/` | `ICON.md`（图标）、`UPDATES.md`（更新与发布）、`MIGRATION.md`（CLaunch 迁移）、`TODO.md`（待办）、`screenshot.webp`（界面截图）、`generate-icon.ps1` |
| `resources/` | 应用图标源文件；`resources.go` 把图标嵌入二进制，供托盘使用，图标文件由 `mygo.json` 的 `resourcesExclude` 排除，不随包安装 |

没有前端。新增一个操作意味着在 `view` 或 `App` 上添加方法，并从需要它的菜单或按钮调用。
