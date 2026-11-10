//go:build windows

package main

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"go-launcher/locales"
	"golang.org/x/sys/windows"
)

const (
	seeMaskNoCloseProcess = 0x00000040
	swShowNormal          = 1
)

type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr
	hProcess       uintptr
}

var shellExecuteExProc = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteExW")

func shellExecuteEx(verb, path, params, dir string) (windows.Handle, error) {
	verbPtr, err := windows.UTF16PtrFromString(verb)
	if err != nil {
		return 0, err
	}
	filePtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var paramsPtr, dirPtr *uint16
	if params != "" {
		paramsPtr, err = windows.UTF16PtrFromString(params)
		if err != nil {
			return 0, err
		}
	}
	if dir != "" {
		dirPtr, err = windows.UTF16PtrFromString(dir)
		if err != nil {
			return 0, err
		}
	}
	info := &shellExecuteInfo{
		fMask:        seeMaskNoCloseProcess,
		lpVerb:       verbPtr,
		lpFile:       filePtr,
		lpParameters: paramsPtr,
		lpDirectory:  dirPtr,
		nShow:        swShowNormal,
	}
	info.cbSize = uint32(unsafe.Sizeof(*info))
	r1, _, callErr := shellExecuteExProc.Call(uintptr(unsafe.Pointer(info)))
	if r1 == 0 {
		return 0, callErr
	}
	return windows.Handle(info.hProcess), nil
}

func joinArgsForShell(args []string) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		if strings.ContainsAny(a, " \t") {
			parts = append(parts, `"`+a+`"`)
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " ")
}

// launchVerb 返回 ShellExecuteEx 使用的动词：item 勾选“Run as administrator”时
// 固定 "runas"（系统弹 UAC 提权），否则用默认的 "open"（仅在程序自身要求提权时
// 才回退到 runas，见 startProcess）。
func launchVerb(elevated bool) string {
	if elevated {
		return "runas"
	}
	return "open"
}

// startProcess launches path with args in workDir and returns the process
// handle; the caller decides how to wait/cleanup.
//
// An .exe starts through CreateProcess with an environment block built fresh
// from the registry, so item processes see the system's current variables
// rather than the copy the launcher inherited when it started. Other files
// (.bat/.cmd/.com) and an .exe CreateProcess cannot start still go through
// ShellExecuteEx, which takes no environment block.
//
// elevated=true（item 勾选了“Run as administrator”）时固定用 "runas" 动词，由系统
// 弹 UAC 提权；否则先用 "open"，只有在程序自身要求提权等情况下才回退到 "runas"。
func startProcess(path string, args []string, workDir string, elevated bool) (windows.Handle, error) {
	if elevated {
		h, err := shellExecuteEx(launchVerb(true), path, joinArgsForShell(args), workDir)
		if err != nil {
			// 1223 = ERROR_CANCELLED：用户在 UAC 弹窗点了“否”
			if errors.Is(err, syscall.Errno(1223)) {
				return 0, errors.New(locales.ElevationCancelled())
			}
			return 0, err
		}
		return h, nil
	}
	if strings.EqualFold(filepath.Ext(path), ".exe") {
		if h, err := createProcessFreshEnv(path, args, workDir); err == nil {
			return h, nil
		}
	}
	h, err := shellExecuteEx(launchVerb(false), path, joinArgsForShell(args), workDir)
	if err != nil {
		if errors.Is(err, syscall.Errno(1223)) || errors.Is(err, syscall.Errno(5)) {
			return 0, err
		}
		h, err = shellExecuteEx("runas", path, joinArgsForShell(args), workDir)
		if err != nil {
			return 0, err
		}
	}
	return h, nil
}

// createProcessFreshEnv starts path with an environment block read from the
// registry now instead of the launcher's own environment: variables the user
// changed after the launcher started apply to the new process.
func createProcessFreshEnv(path string, args []string, workDir string) (windows.Handle, error) {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return 0, err
	}
	defer token.Close()
	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, token, false); err != nil {
		return 0, err
	}
	defer func() { _ = windows.DestroyEnvironmentBlock(env) }()

	app, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	line := make([]string, 0, len(args)+1)
	line = append(line, syscall.EscapeArg(path))
	for _, a := range args {
		line = append(line, syscall.EscapeArg(a))
	}
	cmdLine, err := windows.UTF16PtrFromString(strings.Join(line, " "))
	if err != nil {
		return 0, err
	}
	var dir *uint16
	if workDir != "" {
		dir, err = windows.UTF16PtrFromString(workDir)
		if err != nil {
			return 0, err
		}
	}
	si := &windows.StartupInfo{}
	si.Cb = uint32(unsafe.Sizeof(*si))
	var pi windows.ProcessInformation
	if err := windows.CreateProcess(app, cmdLine, nil, nil, false,
		windows.CREATE_UNICODE_ENVIRONMENT, env, dir, si, &pi); err != nil {
		return 0, err
	}
	_ = windows.CloseHandle(pi.Thread)
	return pi.Process, nil
}

// startUntracked launches path without returning a process handle, so nothing
// can Stop or time it. The shell-executed process keeps running after the
// handle is released.
func startUntracked(path string, args []string, workDir string, elevated bool) error {
	h, err := startProcess(path, args, workDir, elevated)
	if err != nil {
		return err
	}
	_ = windows.CloseHandle(h)
	return nil
}

func startTracked(path string, args []string, workDir string, elevated bool, proc *runningProc) error {
	h, err := startProcess(path, args, workDir, elevated)
	if err != nil {
		return err
	}
	proc.start = time.Now()
	proc.wait = func() error {
		_, _ = windows.WaitForSingleObject(h, windows.INFINITE)
		return nil
	}
	proc.stop = func() error { return windows.TerminateProcess(h, 1) }
	proc.cleanup = func() { _ = windows.CloseHandle(h) }
	return nil
}
