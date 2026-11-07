# MyGo Launcher 应用图标

本文档说明 MyGo Launcher 应用图标的文件、设计、使用方式与仓库提交约定。

## 图标文件

| 路径 | 说明 |
| --- | --- |
| `resources/icon.svg` | 图标矢量源（512×512）。修改设计改这个文件。 |
| `resources/icon.png` | 应用图标（512×512 透明 PNG），由 `icon.svg` 生成。MyGo 从这里生成 exe 图标与安装包图标。 |
| `resources/icon-base.png` | 运行时图标的浅色圆角底与描边（不含强调色图形）。 |
| `resources/icon-base-dark.png` | 暗色外观下的深色圆角底与描边。 |
| `resources/icon-accent.png` | 强调色图形的 alpha 遮罩，箭头保留 16% 透明度，运行时按主题色上色。 |
| `resources/icon-gear.png` | 设置窗口图标右下角齿轮的白色遮罩（透明背景），运行时按明暗上色。 |
| `docs/generate-icon.ps1` | 图标一键生成脚本（SVG → `icon.png`，可选重新打包）。 |
| `docs/generate-icon-assets.ps1` | 生成运行时用的浅色/暗色底、强调色遮罩与齿轮（`icon-base/base-dark/accent/gear.png`）。 |

`resources/` 是 MyGo 打包时复制到程序旁边的资源目录（`mygo.json` 的默认约定）。

## 使用方式

在项目根目录执行：

```powershell
# 只重新生成图标（resources/icon.png）
.\docs\generate-icon.ps1

# 重新生成图标并打包（go tool mygo build）
.\docs\generate-icon.ps1 -Build

# 改了图标设计后，同时重新生成运行时用的底色 / 强调色遮罩 / 齿轮
.\docs\generate-icon-assets.ps1
```

脚本自动完成：SVG 渲染为透明 PNG → 校验透明性 → （可选）重新打包。
MyGo 打包时自己把 PNG 转成多尺寸 ICO，仓库里不再保存 `icon.ico`。

## 运行时配色

运行中的窗口、任务栏按钮和托盘图标由 `src/app_icon.go` 现场合成：强调色遮罩
（`icon-accent.png`）按当前强调色上色后叠到当前外观的底图上——浅色用 `icon-base.png`，
暗色用 `icon-base-dark.png`——箭头因为遮罩保留了 `fill-opacity="0.16"` 而保持半透明。
设置窗口的图标再叠右下角的齿轮（`icon-gear.png` 是透明背景的白色遮罩），描边在浅色底上
用深色、暗色底上用浅色。强调色、明暗主题或系统外观变化后会立即刷新。exe 和快捷方式的
图标仍是构建时的 `icon.png`，不会随运行中的设置改变。

资源脚本用 Chrome/Edge 在透明页面上截图（`--default-background-color=00000000`），
截图自带 alpha，不需要再用洋红背景抠图。

## 注意事项

- 脚本依赖本机 Chrome/Edge 与 .NET System.Drawing（Windows 自带）。
- `generate-icon.ps1` 必须是 **UTF-8 with BOM** 编码（PowerShell 5.1 按 ANSI 解析无 BOM 文件会读乱中文注释）。
- 换了图标后若桌面/任务栏仍显示旧图标，刷新 Windows 图标缓存后重启资源管理器即可（见下）。

## 更换图标后刷新缓存

应用运行时内存图标缓存重启即清空；磁盘缓存 `mygo-launcher-data/cached-icons/` 不入库。若桌面/任务栏仍显示旧图标，运行以下脚本刷新 Windows 图标缓存：

```powershell
Remove-Item "$env:LOCALAPPDATA\IconCache.db" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\Microsoft\Windows\Explorer\iconcache_*.db" -Force -ErrorAction SilentlyContinue
Stop-Process -Name explorer -Force
Start-Process explorer
```

## 仓库提交约定

- **提交**：`resources/icon.svg`、`resources/icon.png`、`resources/icon-base.png`、
  `resources/icon-base-dark.png`、`resources/icon-accent.png`、`resources/icon-gear.png`、
  `docs/generate-icon.ps1`、`docs/generate-icon-assets.ps1`、`docs/ICON.md`。
- **不提交**：`build/`（打包产物）、`resources/.render/`（渲染临时文件）、`mygo-launcher-data/`（运行时数据）——见 `.gitignore`。
