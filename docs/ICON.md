# MyGo Launcher 应用图标

本文档说明 MyGo Launcher 应用图标的文件、设计、使用方式与仓库提交约定。

## 图标文件

| 路径 | 说明 |
| --- | --- |
| `resources/icon.svg` | 图标矢量源（512×512）。修改设计改这个文件。 |
| `resources/icon.png` | 应用图标（512×512 透明 PNG），由 `icon.svg` 生成。MyGo 从这里生成 exe 图标、窗口图标与安装包图标。 |
| `docs/generate-icon.ps1` | 图标一键生成脚本（SVG → PNG，可选重新打包）。 |

`resources/` 是 MyGo 打包时复制到程序旁边的资源目录（`mygo.json` 的默认约定）。

## 使用方式

在项目根目录执行：

```powershell
# 只重新生成图标（resources/icon.png）
.\docs\generate-icon.ps1

# 重新生成图标并打包（go tool mygo build）
.\docs\generate-icon.ps1 -Build
```

脚本自动完成：SVG 渲染为透明 PNG → 校验透明性 → （可选）重新打包。
MyGo 打包时自己把 PNG 转成多尺寸 ICO，仓库里不再保存 `icon.ico`。

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

- **提交**：`resources/icon.svg`、`resources/icon.png`、`docs/generate-icon.ps1`、`docs/ICON.md`。
- **不提交**：`build/`（打包产物）、`resources/.render/`（渲染临时文件）、`mygo-launcher-data/`（运行时数据）——见 `.gitignore`。
