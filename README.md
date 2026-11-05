# MyGo Launcher

用 Go 和 [MyGo](https://github.com/egoist/mygo) 写的 Windows 便携式桌面启动器，原生界面，无 WebView。

<p align="center">
  <img src="docs/screenshot.webp" alt="MyGo Launcher 主窗口与设置窗口" width="960" />
</p>

## 安装

从 [Releases](https://github.com/canwdev/mygo-launcher/releases/latest) 下载安装包
`mygo-launcher-setup-<版本>-amd64.exe`，运行后按向导安装，之后的新版本由应用自动更新。

也可以直接用便携版可执行文件：数据都保存在可执行文件旁的 `mygo-launcher-data/` 目录，整个目录可以随身携带。再次从同一目录启动会聚焦已打开的窗口。

## 功能

- 页面化整理常用程序、文件和文件夹，页面可新建、重命名、复制、移除和拖动排序
- 列表与网格两种视图，切换结果跨重启保留；网格里可以放「空白」控制排列
- 列表与网格的图标大小分小、中、大三挡
- 单击网格条目或双击列表条目启动，运行中的程序可以停止
- 自动累计运行时长，点击时长可按分钟或 ±1 小时修正；可关闭自动计时改为手工编辑
- 条目可以「以管理员身份运行」，启动时弹出 Windows 的 UAC
- 自动提取程序、文件或快捷方式的图标，也可以指定图片或 `.ico` / `.exe` / `.dll` 里的某个图标
- `.lnk` 快捷方式按其中保存的目标和参数启动，并显示快捷方式自己的图标
- 从资源管理器拖入文件即可添加，拖到条目之间会插在那个位置，拖到页面胶囊上则加入该页面
- 条目可以在列表和网格中拖动换位，拖到页面胶囊上稍停会切换页面，再继续放到想要的位置（按住 Ctrl/Cmd 为复制）
- `Ctrl/Cmd+F` 搜索条目，回车启动，准星跳转到条目所在的页面
- 关闭窗口后留在系统托盘继续运行，可在设置里改为关闭即退出
- 自动隐藏、始终置顶、浅色 / 深色 / 跟随系统
- 简体中文与 English，首次启动按 Windows 的显示语言选定
- 从 `CLaunch.ini` 导入页面，尽量沿用按钮的名称、路径、参数和图标

数据保存在可执行文件旁的 `mygo-launcher-data/go-launcher-data.json`，手工修改后会立即生效。

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
- [MyGo Music](https://github.com/zhangnjsd/mygo-music)：界面设计与整体风格的参考
- [MyGO!!!!!](https://bang-dream.com/artist/mygo/)：本项目与官方无关
- [CLaunch](https://ss1.xrea.com/pyonkichi.g1.xrea.com/en/)：风格参考

## 许可证

[MIT](LICENSE) 

开源自由软件，不做任何担保。
