# 从 CLaunch 迁移数据

CLaunch 是一款 Windows 桌面启动器，把页面（Page）里的按钮（Btn）排成网格，配置全部写在
`Data\CLaunch.ini`。MyGo Launcher 在「设置 → 数据」最下方提供一项迁移：选定那个 ini，把每个
CLaunch 页面转成一个新页面。本文先记计划，再记实现与兼容性清单。

## 计划

1. 摸清 `CLaunch.ini` 的真实结构：编码、页面与按钮的分节方式、按钮类型（`Type`）与字段含义、
   子菜单与相对路径的基准。以本机 `D:\Programs\CLaunch\Data\CLaunch.ini` 与 CLaunch 自带
   文档（`Docs\CLaunch_en.chm`）为准。
2. 写一个只读的 INI 解析器：容忍重复分节名（每个页面都重复 `[Btn000]` 起的分节）、UTF-16LE
   编码和空行/注释；不做写回。
3. 建立映射：CLaunch 页面 → 页面；按钮 → `AppItem`；`Type` → 路径、参数、管理员权限；
   子菜单原地展开；相对路径按 `CLaunch.exe` 所在目录解析，`%VAR%` 环境变量展开。
4. 设置界面：在「数据」分组最后一行加「从 CLaunch 迁移」，按钮选定 `CLaunch.ini`。系统对话框
   会阻塞，读盘与提取图标也要时间，所以都在 goroutine 上跑，结果回到主线程再写 store。
5. 兼容启动：迁移会带进 `ms-settings:`、`shell:AppsFolder\<AUMID>` 这类 shell 目标，需要在
   启动、图标和路径规范化里识别它们，避免被当成相对路径拼到数据目录下。
6. 补文案（`locales/en.json`、`locales/zh-Hans.json`）、测试与文档；`CHANGELOG.md` 记一条
   用户可见的变化。

## CLaunch.ini 的结构（实测）

- 文件是 UTF-16LE，带 `FF FE` BOM；手工改过的文件可能是 UTF-8，读取时两种都接受。
- 分节名会重复：`[Page000]` 后紧跟 `[Btn000]`、`[Btn001]`……，下一个 `[Page001]` 又重新
  从 `[Btn000]` 开始，因此解析必须按顺序保留，不能用 `map[string]section`。
- `[Pages]` 只有 `Count`；页面名、按钮数在各自的 `[Page###]` 里（`Name`、`Count`），按钮顺序
  以 `Btn` 分节的出现顺序为准，`Position` 只是它在 CLaunch 网格里的格位。
- 子菜单：`[SubMenus] Count=n`，每个 `[SubMenu###]` 用 `Page=0002`、`Button=9` 指向所属页面
  的第 9 个按钮（从 0 数），后面再跟自己的 `[Btn###]` 分节。
- 按钮字段：`Type`（十六进制类型 + 选项位）、`Name`、`File`、`Parameter`（启动参数）、
  `Directory`（工作目录）、`IconFile`/`IconIndex`、`Tip`、`Keyboard`、`Flag`。

### Type

低 16 位是类型，高位是选项。实测与本机数据、CLaunch 文档对应的值：

| Type | 含义 | 迁移方式 |
| --- | --- | --- |
| `00000001` | 文件、程序、文档 | `File` → 路径 |
| `00000002` / `00000003` | 文件夹或旧版文件项 | `File` → 路径 |
| `00000004` | URL / shell 目标（`ms-settings:`、`shell:`） | `File` 原样作为 shell 目标 |
| `00000006` | 特殊文件夹，`File` 是二进制的 shell ID list | 无法解析，跳过 |
| `00000007` | Windows Store 应用，`File` 是 AppUserModelID | `shell:AppsFolder\<AUMID>` |
| `00000010` | 子菜单 | 用它的子项原地替换父项 |
| `00002000`–`00002fff` | 特殊文件夹，低字节是 CSIDL | 见下表 |
| `00001000` | 关机 | `shutdown.exe /s /t 0` |
| `00001001` | 重启 | `shutdown.exe /r /t 0` |
| `00004003` | 退出 CLaunch | 跳过 |
| `00400001` | 文件，选项位 `00400000` 为「以管理员运行」 | 路径 + `RunAsAdmin` |

CSIDL 是 Windows 的已知文件夹编号，CLaunch 直接存进类型里。可迁移的：

| CSIDL | 文件夹 | 目标 |
| --- | --- | --- |
| `0x00` | 桌面 | `%USERPROFILE%\Desktop` |
| `0x02` | 开始菜单程序 | `%APPDATA%\Microsoft\Windows\Start Menu\Programs` |
| `0x05` | 文档 | `%USERPROFILE%\Documents` |
| `0x06` | 收藏夹 | `%USERPROFILE%\Favorites` |
| `0x07` | 启动 | `%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup` |
| `0x0a` | 回收站 | `::{645FF040-5081-101B-9F08-00AA002F954E}` |
| `0x0b` | 开始菜单 | `%APPDATA%\Microsoft\Windows\Start Menu` |
| `0x11` | 此电脑 | `::{20D04FE0-3AEA-1069-A2D8-08002B30309D}` |
| `0x12` / `0x13` | 网络 | `::{208D2C60-3AEA-1069-A2D7-08002B30309D}` |
| `0x24` / `0x25` | Windows / System32 | `%SystemRoot%` / `%SystemRoot%\System32` |
| `0x26` / `0x2a` | Program Files（64/32 位） | `%ProgramFiles%` / `%ProgramFiles(x86)%` |
| `0x27` / `0x28` | 图片 / 用户目录 | `%USERPROFILE%\Pictures` / `%USERPROFILE%` |
| 其它 | — | 跳过 |

### 路径与图标

- CLaunch 的相对路径以 `CLaunch.exe` 所在目录为基准（文档 `extConvertPath.htm` 明确说明），
  不是以 ini 所在目录为基准。常见布局是 ini 在 `...\CLaunch\Data\`，基准就是它的上一级。
  迁移时按顺序找：ini 目录或其上一级里的 `CLaunch.exe`；找不到就看 ini 是否在名为 `Data` 的
  目录，是就用上一级；否则用 ini 所在目录。
- 路径里的 `%VAR%`、`$VAR` 会在迁移时展开。
- 图标优先取目标文件/文件夹/壳项本身，其次取 `IconFile`（是图片时）。
- `File` 是 `.lnk` 时按「添加文件」的方式解析出目标、参数与起始目录，快捷方式本身记在条目的
  `Link` 上作为图标来源，因此迁移来的快捷方式同样可以计时和停止。`IconFile` +
  `IconIndex` 指定的 DLL 内第 N 个图标无法用 MyGo 的 `Shell.FileIcon` 精确提取，会退化成目标
  自身的图标，但「更新图标」仍可重取。

### 子菜单与无法迁移的项

- 子菜单在父按钮的位置展开成同一页面里的条目，不另建页面；父按钮本身不保留。
- 二进制 ID list（`00000006`）、退出 CLaunch、未知 CSIDL 会跳过，迁移结果里会报出数量。
- 同一个 ini 重复导入会再追加一组同名页面，不做去重：这是用户明确的重复操作。

## 实现

### 代码

| 位置 | 内容 |
| --- | --- |
| `src/claunch.go` | 解码、INI 解析、页面/按钮/子菜单读取、Type 映射、导入结果结构与 `App.addCLaunchImport` |
| `src/view_settings.go` | 「数据」分组最后一行、`migrateCLaunch` 与 `applyCLaunchImport` |
| `src/utils.go` | `isShellTarget`，`normalizePath` 对 shell 目标不再改写斜杠 |
| `src/store.go` | `absPath` 对 shell 目标原样返回，避免把它拼进数据目录 |
| `src/app.go` | 启动计划对 shell 目标走 `OpenPath`/`LaunchApp`，不当作可执行文件跟踪 |
| `src/claunch_test.go` | 解析、映射、应用与设置界面的无头测试 |
| `locales/*.json` | 迁移文案；`go tool mygo generate` 生成访问函数 |

### 迁移流程

1. 设置里点「选择 CLaunch.ini…」：`migrateCLaunch` 置忙碌提示，goroutine 打开系统对话框
   （过滤 `*.ini`）。
2. `importCLaunchFile` 读文件 → `decodeCLaunch`（BOM/UTF-16 检测）→ `readCLaunchIni`
   （顺序解析，子菜单按 `Page`/`Button` 挂回父按钮）→ `buildCLaunchImport`（按当前「新条目使用
   绝对路径」设置生成 `AppItem`，带图标提取）。
3. `dispatch` 回主线程：`App.addCLaunchImport` 追加页面与条目、登记运行时长、保存 store，
   把第一个新页面设为当前页。
4. 主窗口 toast 与设置行说明报告「迁了几个页面、几个条目，跳过几个」。

### 启动兼容

`isShellTarget` 识别 `https://…`、`ms-settings:…`、`shell:…`、`::{GUID}` 这类不是文件路径的
目标（但排除 `C:\…` 盘符路径）。对它们：

- `normalizePath` 不改写，`absPath` 不拼接；
- `planLaunch` 原样交给 shell，不尝试启动进程、不跟踪运行时长；
- `shell:AppsFolder\<AUMID>` 用 MyGo 的 `Shell.LaunchApp`（默认动词），其它交给
  `mygo.Shell.OpenPath`。

### 测试

- UTF-16LE BOM 与重复分节名的解析；
- Type 映射：相对路径、URL、Store 应用、CSIDL、管理员位、二进制 ID list 跳过；
- 子菜单原地展开、空页面不建页面；
- `App.addCLaunchImport` 追加页面、条目和运行时长并写盘；
- `isShellTarget` 与 `planLaunch` 对 shell 目标的行为；
- 设置窗口里出现迁移那一行。

`go test ./...`、`go vet ./...` 与 `GOOS=linux go build -o build/launcher-linux ./src` 作为
回归检查；界面测试不触碰真实数据目录，也不打开系统对话框（无窗口时迁移入口直接返回）。

## 已知局限

- `Type=00000006` 的特殊文件夹（CLaunch 用二进制 PIDL 记录，例如本机数据里的「控制面板」）
  只能跳过；若 `File` 恰好是字面路径/shell 目标则照常迁移。
- `IconIndex` 无法精确还原。
- CLaunch 的快捷键、热键、鼠标直启、子菜单外观、插件项、电源项的确认选项等 UI 行为不迁移。
- 迁移只读 ini，不改动 CLaunch 的数据；重复导入会重复追加页面。
- 页面名与条目名原样保留；空页面名用「CLaunch N」，空条目名用「CLaunch 条目」。
