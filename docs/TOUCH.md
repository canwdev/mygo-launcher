# 触摸与滚动支持

本文记录 MyGo 与 MyGo Launcher 对触摸、触摸板和滚轮的输入支持，供以后决定是否补上触摸屏
滑动时参考。结论按 MyGo 当前检出的实现（`../mygo`，由 `go.mod` 的 `replace` 固定，CI 克隆
固定提交）整理。

## 结论

| 输入方式 | Windows | Linux |
| --- | --- | --- |
| 鼠标滚轮（竖直/横向） | 支持，按系统「每次滚动行数」换算，带平滑动画 | 支持 |
| 精确式触摸板双指滑动 | 支持，按像素跟手，松手有惯性 | 支持 |
| 触摸板双指捏合 | 支持，Windows 发送为 Ctrl+滚轮，转为 `ui.InputZoom` | 支持 |
| 触摸屏单指拖动滚动 | **不支持** | 支持 |
| 触摸屏双指捏合 / 双指滚动 | **不支持** | 支持 |

Windows 是本项目的发布平台，所以触摸屏滑动目前等于没有：界面不会跟手滚动。

## 代码依据（MyGo）

- Windows 后端的输入入口是 `internal/windows/surface.go`。它只处理
  `WM_MOUSEWHEEL`（`0x020A`）和 `WM_MOUSEHWHEEL`（`0x020E`）：`wmMouseWheel` 分支把
  滚轮增量换算成 DIP，`delta % 120 != 0` 时标记 `Precise`，按 `Ctrl+滚轮` 判断触摸板捏合并
  转成 `PointerZoom`。
- MyGo 全模块没有 `RegisterTouchWindow`、`WM_TOUCH`、`WM_POINTER`、`WM_GESTURE`、
  `GetTouchInputInfo` 之类的调用。未注册触摸的窗口由 Windows 把触屏「提升」为鼠标输入，
  只保留主指针的按下、移动和抬起。
- `ui/scroll.go` 的滚动容器只消费滚轮/`InputScroll`；没有「拖着内容滚」的实现。滚动条拖动
  走普通指针事件，触屏可以拖滚动条，但滑块很细，不适合当手势用。
- Linux 后端另有 `internal/linux/touch.go`：单指拖动转成 `Precise` 的滚动，双指转成
  `InputZoom`，说明「跟手滚动」在引擎层是现成的，缺的只是 Windows 的触摸消息。

## 在本启动器里的表现（Windows）

- 主界面 [wheelInput](../src/view_tabs.go) 会吃掉竖直滚动：滚轮和触摸板双指上下滑默认用于
  切换页面（`Precise` 事件的阈值是 120 DIP），按住 Shift 才滚动网格；横向 `DX` 不经过它。
- 设置窗口、文件夹面板等普通 `ui.Scroll` 列表用滚轮和触摸板正常滚动。
- 触屏上：轻点等于左键点击（点条目会启动），按住移动等于左键拖拽。MyGo 的拖动阈值只有
  4 DIP，所以在条目上滑动会把条目拖起来，而不是滚动；空白区域滑动没有滚动效果。

## 如果要支持 Windows 触摸屏

需要改 MyGo 的 Windows 后端，本仓库不用动界面代码：

1. 用 `RegisterTouchWindow`（或 `EnableMouseInPointer` + `WM_POINTER*`）接收触点；
2. 单指拖动转成 `Precise` 的 `InputScroll`（Linux `touch.go` 的做法）；
3. 双指的距离转成 `InputZoom`，中点移动转成滚动；
4. 轻点、长按右键等保持系统或自行模拟。

升级 MyGo 检出（或给固定提交打补丁）后重新构建即可。唯一的启动器侧顾虑是：触屏滑过条目时，
拖动会先于滚动被识别，可能需要按输入类型区分「条目拖动」和「内容滚动」。

本轮决定先不改 MyGo，也不在本仓库另做触摸层，只记录以上结论。
