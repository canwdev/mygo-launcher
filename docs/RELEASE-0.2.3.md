# 0.2.3 工作状态

工作状态文档：记录目标、设计决定、步骤与验证结果，交给后续接手的人。

## 目标

- 计时器关闭后立即停止所有计时相关事务，不再跟踪程序。
- 全局快捷键唤出的窗口以光标为中心，并限制在当前显示器工作区；新增 `--at-cursor`。
- 新增 `--silent` 静默启动。
- 主界面完整键盘操作，条目按槽位显示 QWERTY 字母。
- 图标按钮 Tooltip 显示名称与快捷键；条目 Tooltip 显示名称与字母。
- 运行时长提示改为「计时器」。

## 外观自定义

- 设置 → 外观底部增加「程序图标」「背景」「背景透明度」。
- 程序图标复用 `App.customIcon`（图片、`.ico`、`.exe`、`.dll`，取第一个图标，不含 SVG），
  存进 `custom-icons`；主窗口、托盘、「关于」用原图，设置窗口始终叠齿轮。
- 背景图片归一化后也存进 `custom-icons`（png/jpg/jpeg/gif/bmp/webp；按显示器像素缩到最长边
  1280~2560，不透明转 JPEG 质量 90，带透明保留 PNG），加载时再受同一上限保护；主窗口（含顶栏）
  按整窗 cover 和透明度连续绘制，设置窗口保持原底色。透明度默认 0.4，范围 0.05~0.9。
- 两项都不加 Tooltip；背景没有已选提示。`pruneIcons` 要把 `ui-state.json` 里的两个路径
  算作引用，避免启动时被清理。

## 设计决定

- **计时器**：`planLaunch` 只在 `AutoTime` 开启时检查重复运行并设置 `track`；关闭开关时
  `App.stopTiming` 清空 `running`、把旧进程标成 `timed=false`、停止时钟重绘，不结束进程。
  `finishProcess` 只在 `AutoTime && p.timed` 时累计，避免关掉再开启时给旧进程补计时。
- **窗口定位**：`placeWindowAtCursor` 是纯函数，按光标居中并夹在 `Display.WorkArea` 内。
  进程内用 `mygo.Screen` 的光标和显示器；第二实例用 Win32 的 `GetWindowPlacement` /
  `GetCursorPos` / `MonitorFromPoint` / `SetWindowPos` 直接移动已有窗口，不做 IPC。
- **启动参数**：`--silent`、`--at-cursor`，同时出现时静默优先；冷启动用
  `WindowOptions.Hidden`，`--at-cursor` 在 `OnReadyToShow` 定位后显示。
- **键盘**：`Context.OnWindowShortcut` 注册窗口快捷键，不依赖焦点，浮层与对话框优先；
  `overlayOpen()` 兜底。选中状态是槽位下标（`keySel/keySelTab`），空白不参与方向键选中；
  字母按槽位分配（空白也占位）但只选中、不启动。悬停条目等同于选中，`hoverSlot` 只在指针
  换到另一个条目时更新，方向键可以离开悬停项；悬停高亮与选中都用 `p.hover`。指针停在页面
  胶囊上时用键盘切页，会把该胶囊记进 `hoverIgnoredTab`，避免悬停延迟切回。键盘导航期间
  `keyboardNav` 暂停悬停高亮和悬停时长，指针移动后由 `pointerBack` 交还鼠标；同时选中格子里
  一个无焦点环的 0×0 `key-focus` 盒子获得焦点，回车不会落到之前点击过的按钮上。
- **Tooltip**：加速键 `Ctrl+F` / `Ctrl+,` 是字面量，不进 `locales`；`Ctrl+F` 原来只在
  构建时查询、没有注册，这次与 `Ctrl+,` 一起改为 `OnWindowShortcut`。条目 Tooltip 只显示
  名称；时长字段未运行时的提示复用 `autoTiming`。

## 步骤

- [x] 计时器关闭行为与测试
- [x] 光标定位纯函数与测试
- [x] 启动参数解析与测试
- [x] 冷启动 / 第二实例 `--at-cursor`、`--silent`
- [x] 主界面键盘、选中高亮、滚动可见
- [x] 条目字母选中、Tooltip
- [x] 文案生成、README/AGENTS、版本号与 CHANGELOG
- [x] 自定义程序图标：偏好、选择/清除、四处展示、齿轮角标
- [x] 自定义背景：图片归一化、整窗 cover 绘制、透明度
- [x] 外观设置行、文案、测试与文档

## 验证

- `go vet ./...`、`go test ./...`、Windows 构建、Linux 交叉编译
- 外观自定义的测试在 `src/appearance_test.go`：背景归一化与清理、程序图标补方形与齿轮、
  整窗背景绘制、偏好默认值、设置行。
- 待手动验证：真实窗口的快捷键定位、第二实例定位、静默启动、托盘、多显示器与高 DPI；
  系统对话框选择程序图标/背景，以及窗口缩放、拖动时背景的显示。

## 实现时采用的默认

- 静默启动且后台运行关闭：窗口隐藏且不创建托盘；如果设置了全局快捷键仍可唤出。
- `--at-cursor` 在窗口已在前台时：只置前并移动，不像快捷键那样切换隐藏。
- 回车「打开子菜单」：按单击处理，文件夹打开内容面板。

## 文件

- `src/app.go`：计时跟踪与 `stopTiming`
- `src/view_settings.go`：关闭计时器的清理
- `src/launch_args.go` / `src/window_place.go`：启动参数、光标定位
- `src/singleton_windows.go` / `src/singleton_other.go`：第二实例
- `src/view_keyboard.go` / `src/view_grid.go` / `src/view.go`：键盘、选中、Tooltip
- `src/view_header.go`：图标按钮 Tooltip
- `locales/*.json`：计时器提示
- 测试：`src/timer_test.go`、`src/launch_args_test.go`、`src/view_keyboard_test.go`
