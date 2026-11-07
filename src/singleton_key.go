package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// installDir 返回当前可执行文件所在的安装目录（绝对路径，尽量归一化）。
func installDir() string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	dir := filepath.Dir(exe)
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = filepath.Clean(dir)
	}
	return filepath.Clean(abs)
}

// installKey 返回当前安装目录对应的单例键。
//
// 单例按安装目录划分：每个安装目录只放一份 go-launcher.exe，目录不同则键不同，
// 不同目录的程序可以同时运行；同一目录再次启动会得到相同键，被判定为重复实例，
// 由调用方聚焦已有窗口后退出。
//
// Windows 大小写折叠保证同一目录永远得到同一个键；哈希输出定长十六进制，可安全
// 用于命名互斥体 / 锁文件名。
func installKey() string {
	dir := installDir()
	if runtime.GOOS == "windows" {
		dir = strings.ToLower(dir) // Windows 路径大小写不敏感
	}
	sum := sha256.Sum256([]byte(dir))
	return hex.EncodeToString(sum[:])
}

// instanceTitle 返回实例的说明：应用名 + 完整安装目录，用作托盘悬停提示。
// 窗口标题只有 appName；托盘提示保留安装位置，不同目录的副本也能分辨。
func instanceTitle(dir string) string {
	return appName + " - " + dir
}
