# MyGo Launcher

用 Go 和 [MyGo](https://github.com/egoist/mygo) 写的 Windows 便携式桌面启动器，原生界面，无 WebView。

**简体中文** | [English](README-en.md)

<p align="center">
  <img src="docs/screenshot.webp" alt="MyGo Launcher 主窗口与设置窗口" width="960" />
</p>

## 安装

目前支持 **Windows 10/11 的 x64 与 arm64**；不支持 32 位 Windows 和 Windows 7/8。

从 [Releases](https://github.com/canwdev/mygo-launcher/releases/latest) 下载对应架构的安装包或便携版。

## 功能

- 页面化整理常用程序、文件和文件夹，页面可新建、重命名、复制、移除和拖动排序
- 网格视图，可以放「空白」控制排列，图标大小分小、中、大三挡
- 单击网格条目启动，运行中的程序可以停止；文件夹条目单击则展开内容，可逐层进入子目录，面板里的条目可以右键打开系统菜单
- 计时器开启时自动累计运行时长，点击时长可按 ±1 小时、±30 分钟或分钟修正；关闭后不显示时长也不计时；文件夹、文件和网址不计时
- 条目可以「以管理员身份运行」，启动时弹出 Windows 的 UAC
- 自动提取程序、文件或快捷方式的图标；拖入图片或 `.ico` 时直接显示图片内容，也可以指定图片或 `.ico` / `.exe` / `.dll` 里的某个图标
- `.lnk` 快捷方式按其中保存的目标和参数启动，并显示快捷方式自己的图标
- 从资源管理器拖入文件即可添加，拖到条目之间会插在那个位置，拖到页面胶囊上则加入该页面
- 右下角的 `+` 可以选择运行中的程序、文件或手动创建条目，也可以直接添加回收站、此电脑、控制面板、用户文件夹、网络等常用位置
- 文件和文件夹的右键菜单提供「属性」和系统原生菜单；回收站可以直接清空，图标随空/满更新
- 条目可以在网格中拖动换位，拖到页面胶囊上稍停会切换页面，再继续放到想要的位置（按住 Ctrl/Cmd 为复制）
- `Ctrl/Cmd+F` 搜索条目，回车启动，准星跳转到条目所在的页面
- 可录制全局快捷键（功能键可单独使用），在任意窗口显示或隐藏启动器（应用运行时生效）
- 关闭窗口后留在系统托盘继续运行，可在设置里改为关闭即退出
- 自动隐藏、窗口置顶、浅色 / 深色 / 跟随系统
- 滚轮默认切换页面，按住 Shift 滚动网格；可在设置里改为滚轮滚动、Shift+滚轮切页
- 运行中的窗口与托盘图标跟随强调色和明暗主题，设置窗口图标带齿轮
- 默认在内存中绘制以降低内存占用；可在设置里开启 GPU 绘制，重启后生效
- 简体中文与 English，首次启动按 Windows 的显示语言选定
- 从 `CLaunch.ini` 导入页面，尽量沿用按钮的名称、路径、参数和图标

便携版的数据保存在可执行文件旁的 `mygo-launcher-data/`；安装版保存在
`%LOCALAPPDATA%\MyGo Launcher\mygo-launcher-data\`，升级时旧数据会自动迁移，卸载时会询问是否
一并删除。手工修改 `go-launcher-data.json` 后会立即生效。

## 构建

需要 Go 1.27+。`go.mod` 用 `replace` 指向本机的 `../mygo` 检出，也可以删掉 `replace` 改用发布版模块。

```bash
go tool mygo dev     # 开发模式运行，改代码自动重启
go tool mygo build   # 打包到 build/，含 NSIS 安装包
go test ./...        # 运行测试（含无头界面测试）
```

发版、更新签名与安装包细节见 [docs/UPDATES.md](docs/UPDATES.md)。

## 致谢

- [MyGo](https://github.com/egoist/mygo)：原生 UI 与应用框架
- [MyGo Music](https://github.com/egoist/mygo-cloudmusic)：界面设计与整体风格的参考
- [MyGO!!!!!](https://bang-dream.com/artist/mygo/)：本项目与官方无关
- [CLaunch](https://ss1.xrea.com/pyonkichi.g1.xrea.com/en/)：风格参考

## 许可证

[MIT](LICENSE)

开源自由软件，不做任何担保。
