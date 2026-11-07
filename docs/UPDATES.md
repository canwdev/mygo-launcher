# 自动更新与发布

本文档说明 MyGo Launcher 的自动/手动更新、签名密钥和 GitHub 发布流程。方案与
MyGo Music 相同：MyGo 的 updater 插件负责检查、展示和安装，`mygo build -upload`
负责签名并上传更新包。

## 用户看到的行为

- 应用启动约 10 秒后自动检查一次 GitHub Releases，之后每天一次；发现新版本时弹出
  原生更新窗口，显示 CHANGELOG 中对应版本的说明，可以安装、稍后提醒或跳过该版本。
- 设置 → 关于 里可以：
  - 打开/关闭「自动检查更新」（默认开启）；
  - 打开/关闭「自动下载并安装」（仅在自动检查开启时可用）；
  - 查看当前版本、上次检查时间或发现的新版本，并手动「检查更新」。
- 「自动下载并安装」开启后，后台检查到的新版本会直接下载，下次启动时生效。
- 开发构建（`go tool mygo dev`）和无法写入安装目录的构建不会后台检查；手动检查时窗口会
  说明原因。

## 签名密钥

更新包用 Ed25519 密钥签名，公钥内置在应用里，私钥只用于构建和 CI。

- 公钥在 `mygo.json` 的 `updates.publicKey`。
- 私钥在 MyGo 默认的密钥目录：
  `%APPDATA%\mygo\update-keys\mygo-update.key`（本机已生成于
  `C:\Users\arch\AppData\Roaming\mygo\update-keys\mygo-update.key`）。
- **把私钥内容加入 GitHub 仓库 secret `MYGO_UPDATER_PRIVATE_KEY`**，然后妥善保存；
  丢失后已安装的旧版本无法再接受新更新，泄露则可以给用户推送任意代码。
- 不要重新运行 `mygo keygen -force`：新公钥会让旧安装无法验证新更新。

## 发布流程

1. 修改 `mygo.json` 的 `version` 与 `src/version.go` 的 `appVersion`，在
   `CHANGELOG.md` 顶部增加 `## <version>` 小节（MyGo 会把这一节放进更新窗口和
   Release notes；没有这一节 `mygo build` 会失败）。
2. 提交并推送，然后打 tag 并推送：

   ```sh
   git tag v0.3.0
   git push origin v0.3.0
   ```

3. `.github/workflows/release.yml` 会：
   - 校验 tag 与 `mygo.json` 的版本一致；
   - 从 CHANGELOG 取出该版本说明；
   - 创建 **draft** Release；
   - 在仓库旁检出固定提交的 MyGo，然后用 `MYGO_UPDATER_PRIVATE_KEY` 和 `GH_TOKEN` 执行
     `go tool mygo build -platform windows/amd64,windows/arm64 -upload`；
   - 为 x64 和 arm64 各生成安装包、应用归档、签名更新包和对应的
     `update-windows-<arch>.json`；
   - 把安装包、应用归档、`SHA256SUMS`、更新包和 `update-windows-*.json` 上传到 draft。
4. 在 GitHub 上检查 draft 的附件，然后手动发布，并确保勾选 **Set as the latest
   release**。安装后的应用通过 `releases/latest/download/` 读取清单。

## UPX（已弃用）

曾在提交 `db2c8ec6e160e2669d56889a7d34a766833d5215` 中实现：CI 安装 UPX 5.3.0，给固定提交的
MyGo 打补丁（`.github/patches/mygo-upx.patch`），以 `MYGO_UPX=upx` 在生成安装包、归档和签名
更新包之前执行 `upx --best --lzma`，exe 从约 17MB 降到约 5.1MB。

之后移除，原因是代价太大：UPX 用运行内存换体积，软件绘制下常驻内存约从 50MB 升到 90MB；
同时要给固定提交的 MyGo 维护补丁，增加构建复杂度。当前发布包不再压缩。

本地验证签名打包：

```sh
MYGO_UPDATER_PRIVATE_KEY="$(cat /path/to/mygo-update.key)" \
  go tool mygo build -skip-installer
```

不带密钥时仍能构建 exe，但不会生成 `update-windows-*.json` 和更新包。

## 代码位置

- `src/updates.go` — updater 插件、偏好和测试替换点。
- `src/view_settings.go` — 关于分组里的更新设置与手动检查。
- `src/main.go` — App.Run 前启用插件。
- `locales/*.json` — 更新相关文案，改完运行 `go tool mygo generate`。
- `.github/workflows/release.yml` — tag 触发的 draft + `-upload` 发布，同时构建 x64 与
  arm64；Release notes 末尾附带按架构下载的表格。

## 验证

- `go build -o build ./src`
- `go test ./...`
- `go tool mygo i18n check`
- 本地 `go tool mygo build -skip-installer` 只验证普通打包；签名更新包需要私钥。
- 用旧版本手动检查一次更新，确认能读取 Release 清单并显示更新窗口。

## 交接记录

- 2026-11-05 — 设计与实现完成：原生更新插件、关于分组、签名密钥、tag 发布流程。
