# 界面重做：计划与交接

**状态：** 已实现并验证 · **最后更新：** 2026-11-05

本文档是 `docs/TODO.md` 里界面重做的实施计划与交接说明。动手前先读 `AGENTS.md`（帧模型、
数据布局和测试规则仍然适用）。完成或暂停某一步时，更新这里的勾选状态，并在「交接记录」里补一条
说明，方便后续接手。

## 1. 目标

来自 `docs/TODO.md` 的两项：

1. **参照 `../mygo-music` 重写界面。** 借鉴它的配色、控件风格、分组设置卡片和整体质感，而不是
   在旧 Wails 界面上逐处打补丁。导航最终采用**顶部标签栏**，不使用它的左侧栏。
2. **全局菜单改为设置。** 原来的右上角 ⋯ 菜单改成齿轮入口；主题、强调色、自动计时、自动隐藏、
   始终置顶、路径转换、数据目录和图标维护都收进设置窗口，菜单不再承载操作项。

参考资料（位于同级检出 `../mygo-music`）：

| 文件 | 可借鉴的内容 |
| --- | --- |
| `internal/app/ui_kit.go` | `paletteFor`、`iconButton`/`roundButton`、`segmented` |
| `internal/app/ui_main.go` | 根布局、内容列、上下内边距 |
| `internal/app/ui_settings.go` | 分组设置卡片、`row` 辅助函数、开关/分段控件 |
| `internal/app/ui_pages.go` | 页面标题、卡片网格、带悬停态的列表行 |
| `internal/app/icons.go` | Lucide 风格描边图标与设置齿轮 |

## 2. 设计决定

以下决定已经落地；修改时请在本节补充原因。

- **调色板。** `src/theme.go` 的 `palette` 提供浅色/深色两套：
  `bg`、`sidebar`、`text`、`muted`、`faint`、`hairline`、`hover`、`press`、`accent`、
  `onAccent`、`shade`。每帧复制 `c.Theme()`，覆盖 `Accent`、`AccentText`、`Selection` 后
  `c.SetTheme`。浅色/深色本身仍由 MyGo 的 ThemeSource 决定，调色板只负责视图用色。
- **强调色是偏好。** 预设取自 MyGo 网站 `website/src/styles.css` 的 `--bang-1` 至
  `--bang-5`，即 MyGO!!!!! 五位成员 Tomori、Anon、Rāna、Soyo、Taki 的颜色；每个预设都有明暗
  两套，跟随主题自动切换，默认 `--bang-1`。`ui-state.json` 的 `accent` 保存预设 id
  （`bang-1`…`bang-5`）或自定义 `#rrggbb`。`accentText` 按亮度选择白字或深色字，浅色自定义色
  也能保证可读。
- **主布局。** 纵向：56 DIP 顶栏（`sidebar` 背景，可拖动窗口）+ 内容页（`bg` 背景）+ 底部忙碌条。
  - 顶栏从左到右：横向标签胶囊与「加标签」按钮、搜索、列表/网格切换、设置。没有品牌、分割线或
    页面大标题；标签多时横向滚动。
  - 内容区直接显示列表或网格；右下角叠加圆形添加 FAB。列表/网格底部留出空间，避免 FAB 遮住
    最后一行。
- **添加入口是 FAB。** 右下角圆形强调色 `+`，没有可见文字，打开「选择文件 / 手动创建」菜单。
- **标签留在顶栏。** 保留原有操作：点击切换、拖动排序、拖入条目（Ctrl 复制）、右键重命名/复制/
  删除。选中项用低调的 `press` 底色和加深文字；标签没有 ⋯ 按钮。
- **菜单只用右键。** 行、网格单元、空格子和标签都不再有 ⋯，各自在元素上挂弹出菜单。
- **设置是独立窗口**（与 MyGo Music 一致）：`mygo.NewWindow` +
  `Content: ui.View(v.settingsView)`，560×720（最小 480×420），`StateKey: "settings"`，
  禁止最大化/最小化，`AutoHideMenuBar`，背景 `light-dark(#f4f4f6, #1c1c1f)`。首次使用时创建，
  之后显示并聚焦，关闭时清空指针。`App.refresh` 同时刷新两个窗口，设置里的改动立即生效。
- **无头测试不打开设置窗口。** `app.win == nil`（测试）时 `openSettings` 直接返回；测试用
  `ui.NewTester(v.settingsView, 560, 720)` 单独驱动设置内容。
- **测试里不改主题。** 帧内调用 `mygo.Theme.SetSource` 会死锁无头 tester。主题切换由设置里的
  点击触发，先保存偏好，再通过 `app.dispatch` 在帧后执行；无窗口时不改主题。
- **保留原有命令。** 条目菜单、对话框、搜索、运行时编辑、拖拽/排序以及 store/监听逻辑的行为
  不变。这是界面重写，不是功能重写。

## 3. 实施步骤

### 第 1 步 — 调色板、控件与图标

- [x] `src/theme.go`：`palette`、`paletteFor(t, accent)`、`applyPalette(c, accent)`、
      `accentColor`、`accentText`、预设列表与自定义色解析。
- [x] `src/ui_kit.go`：`iconButton`（无边框、圆形、悬停圆圈）与 `segmented`。
- [x] `src/icons_svg.go`：设置齿轮、列表/网格切换、播放/停止、盒子等控件图标；24×24 描边、
      1.75 线宽，与 MyGo Music 一致。
- [x] 移除旧顶栏辅助函数（`iconButton`、`themeButton`、`viewModeButton`、`appMenuButton`）
      以及不再使用的 pill/dots。

### 第 2 步 — 主窗口外壳

- [x] `src/view.go`：每帧计算调色板；根列、顶栏、页面；添加 FAB；忙碌条。
- [x] `src/view_header.go`：横向标签与加标签按钮、搜索、布局切换、设置；没有品牌或大标题。
- [x] `src/view_tabs.go`：顶栏标签胶囊；保留拖拽/排序、条目拖入和右键菜单，去掉 ⋯。一个元素
      只接收一种拖动类型。

### 第 3 步 — 列表、网格、搜索与对话框

- [x] `src/view_list.go`：54 DIP 行、圆角 10、悬停填充、运行/定位时的强调色淡染；34 DIP 图标；
      名称 + 路径；时长；运行/停止；没有 ⋯，也没有手型光标（单击不会启动）。保留双击启动和
      拖放；右键打开条目菜单。
- [x] `src/view_grid.go`：116×128 卡片、浅色静止底色、54 DIP 图标与名称作为一组垂直居中、
      角上时长；虚线空格子；右键菜单；保留排序/插入/删除。
- [x] `src/view_search.go`：调色板配色、选中行强调色、准星定位、输入框不再套两层边框；列表
      给出明确高度（行数 × 40，最多 320），底部提示不会压住最后一行。
- [x] `src/view_dialogs.go`：17pt 标题、调色板弱化文字、强调色主按钮；测试依赖的标签
      （`Cancel`、`Save`、`OK`、`Edit item` 等）保持不变。
- [x] 空状态：居中图标、「No files added yet.」和提示文字。

### 第 4 步 — 设置窗口

- [x] `App`：增加 `settingsWin *mygo.Window`；`App.refresh` 也会刷新它。
- [x] `src/view_settings.go`：`openSettings`、`settingsView`、`changeTheme`、`changedSettings`。
- [x] 分组：
  - 外观：主题分段控件（System / Light / Dark）；强调色一行：五个 `--bang` 预设色块加一个
    `ui.ColorWell` 自定义取色器。布局切换在顶栏，不在设置里。
  - 行为：自动计时、启动后自动隐藏、始终置顶（开关）。
  - 路径：新条目使用绝对路径（开关）；批量转为绝对/相对路径（按钮）。
  - 数据：刷新 store、打开数据目录、批量更新图标。
  - 关于：版本与 GitHub 链接。
- [x] 所有改动通过 `App.save` / `setAlwaysOnTop` / `savePrefs` 保存；`App.refresh` 同时更新
      两个窗口。主题切换在帧后执行。
- [x] 删除 `appMenuButton` 以及顶栏原来的主题/视图切换按钮。

### 第 5 步 — 测试、文档与验证

- [x] `src/view_test.go`：开关使用 `Toggle …` 标签点击；布局测试改为点顶栏按钮；新增设置
      冒烟测试和强调色测试（默认 `--bang-1`、点击 `Anon` 后保存 `bang-2`、明暗颜色不同）。
- [x] `AGENTS.md`：调色板、右键菜单、顶部导航、设置窗口、测试规则和文件表。
- [x] `CHANGELOG.md`：界面、强调色、设置、右键菜单、FAB、布局切换等条目。
- [x] `docs/TODO.md`：两项均已勾选，并指向本文档。
- [x] `README.md`：简介、数据文件和目录结构按新界面更新。
- [x] 执行 `go build -o build ./src`、`go test ./...`、`go vet ./...`、
      `GOOS=linux go build -o build/launcher-linux ./src`，全部通过；`gofmt -l .` 无输出。

### 第 6 步 — 目录整理与文档中文化

- [x] 所有 Go 源码与测试移入 `src/`（含包内测试）；`mygo.json` 增加 `"main": "./src"`。
      已验证 `go tool mygo build -skip-installer` 可以正常打包。
- [x] `TODO.md` 移到 `docs/TODO.md`。
- [x] `AGENTS.md` 取消英文限制：仓库文档一律中文，代码注释不限语言。
- [x] `README.md`、`CHANGELOG.md`、`docs/UI-REDO.md` 改写为中文，并更新路径。

## 4. 结果与验证

最终界面：56 DIP 顶栏（标签胶囊 + 加标签、搜索、列表/网格切换、设置）覆盖内容区，右下角是
强调色圆形 `+`；没有品牌、分割线或大标题。列表、网格、搜索和对话框沿用 MyGo Music 的调色板与
控件。强调色来自 `ui-state.json`（默认 `--bang-1`，另有四个 `--bang` 预设与自定义颜色，明暗
两套随主题自动切换）。
⋯ 按钮全部取消，条目、标签和空格子都用右键菜单。齿轮打开独立 560×720 设置窗口，
`App.refresh` 同时刷新两个窗口。

开发期间用 `ui.Tester.Image()` 无头渲染并检查了列表、网格、条目对话框、搜索浮层和设置窗口
（浅色与深色）的截图；临时测试与 PNG 已删除。最终验证：

- `go build -o build ./src`、`go test ./...`、`go vet ./...`、
  `GOOS=linux go build -o build/launcher-linux ./src` 全部通过；
- `gofmt -l .` 无输出；
- `go tool mygo build -skip-installer` 成功产出 `build/windows-amd64/MyGo Launcher.exe`。

## 5. 注意事项（结合当前代码复核）

- **每一帧都从状态重建。** 回调在构建之外运行：改变窗口显示内容后要请求一帧（`v.refresh()`；
  集合变化用 `v.later(fn)`）。
- **不要在构建集合的循环里修改集合。** 增删、重排、打开对话框都走 `v.later`。
- **`mygo.Theme.SetSource` 不能在帧内运行**，否则无头 `ui.Tester` 会死锁；主题切换必须在帧后。
- **一个元素只接收一种拖动值。** 标签内部胶囊接收 `dragItem`，外层包装接收 `dragTab`。
- **tester 只能点击可见、可达的目标。** `List`/`GridView` 只构建滚动到视野内的行；测试数据要
  放在顶部附近。
- **测试不能碰真实数据目录。** `newTestApp` 把 `absBase` 指向 `t.TempDir()`。
- **测试里 `app.win` 为 nil。** 真实窗口调用（`settingsWin`、`SetAlwaysOnTop`、系统对话框）
  要按现有方式判空。
- **无头截图里动画颜色不会自动推进。** 添加 FAB 用 `AnimateColor` 动画填充；改强调色后要新建
  `ui.Tester`（或调用 `Advance`）才能截到新颜色。

## 6. 文件结构

| 路径 | 作用 |
| --- | --- |
| `mygo.json` | 应用名、版本、入口包 `./src`、输出目录 |
| `src/theme.go` | 浅色/深色调色板与强调色偏好 |
| `src/ui_kit.go` | 共享圆形图标按钮与分段控件 |
| `src/view.go` | 帧入口、调色板、页面结构、添加 FAB、忙碌条、`later`/`refresh` |
| `src/view_header.go` | 顶栏：标签、搜索、布局切换、设置 |
| `src/view_settings.go` | 设置窗口及其内容 |
| `src/view_tabs.go` | 标签胶囊：拖拽、拖入、右键菜单 |
| `src/view_list.go` / `src/view_grid.go` | 两种布局 |
| `src/view_dialogs.go` / `src/view_search.go` | 条目/运行时/重命名/确认对话框；搜索浮层 |
| `src/icons_svg.go` | 窗口与设置绘制的图标 |
| `src/app.go` | 增加 `settingsWin` 并随刷新一起重绘 |
| `docs/TODO.md` | 待办（界面重做两项已完成） |

设置窗口使用 `StateKey: "settings"`，因此 `window-state.json` 在首次打开后会多出一个
`settings` 条目，与 `main` 并列。

## 7. 交接记录

- 2026-11-05 — 写出计划，开始实现。
- 2026-11-05 — 第 1–5 步完成：测试、构建、`go vet` 和 Linux 交叉编译均通过，README、AGENTS、
  CHANGELOG、TODO 已更新；检查了列表、网格、对话框、搜索和设置窗口的无头截图。
- 2026-11-05 — 按用户要求把左侧栏改为顶栏，重新截图检查。
- 2026-11-05 — 再次调整：去掉品牌、分割线和大标题；顶栏为标签 + 搜索 + 布局切换 + 设置；添加
  操作变成右下角 FAB；取消所有 ⋯，统一右键菜单；列表行去掉手型光标；网格卡片改为 116×128、
  54 DIP 图标并与名称垂直居中。测试（含强调色测试）和全部验证命令通过。
- 2026-11-05 — 目录整理：Go 源码移入 `src/`，`mygo.json` 指定 `"main": "./src"`，已验证
  `go tool mygo build -skip-installer`；`TODO.md` 移到 `docs/`；全部仓库文档改为中文。
- 2026-11-05 — 强调色预设改为网站 `styles.css` 的 `--bang-1` 至 `--bang-5`（Tomori、Anon、
  Rāna、Soyo、Taki，明暗两套随主题切换），默认 `--bang-1`，保留成员配色注释；设置色板与测试
  同步更新，浅色/深色设置窗口截图已检查。

## 8. 后续事项

- **在真实窗口中检查。** 无头渲染能验证布局和颜色，但无法验证原生标题栏、窗口拖动、设置窗口的
  打开/聚焦，以及实时主题切换。请在 Windows 上运行 `go tool mygo dev`，检查浅色与深色。
- **确定设置窗口与主窗口的关系。** 目前它是独立顶层窗口、有自己记住的几何位置（与 MyGo Music
  一致）。如果需要始终盖在启动器之上或随主窗口关闭，需要明确选择（父/所有者窗口或关闭钩子）。
- **主题切换刻意未测试。** 如果以后需要测试，应确保 `SetSource` 通过 `Window.Update` 在帧后
  执行，不要在构建期间调用。
- **自定义取色器未做无头测试。** 预设色块有测试覆盖；`ui.ColorWell` 的往返需要在真实窗口确认。
- **网格可继续打磨。** 卡片目前使用浅色静止底色；想要更强层次感，可参考 MyGo Music 的悬停上浮
  与阴影（`coverCard`），而不是加边框。
- **标签很多时的顶栏。** 标签栏可以横向滚动；如果经常超过十几个标签，考虑增加滚动提示或
  「更多」菜单。
