//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runningWindow is a program that has a window on screen, as the window
// picker lists it: the program it starts and the window's title, which tells
// two windows of one program apart.
type runningWindow struct {
	Name  string // the program's display name
	Title string // the window's title
	Path  string // the program's executable
	PID   uint32
}

var (
	runningUser32  = syscall.NewLazyDLL("user32.dll")
	runningDwm     = syscall.NewLazyDLL("dwmapi.dll")
	runningVersion = syscall.NewLazyDLL("version.dll")

	procIsWindowVisible         = runningUser32.NewProc("IsWindowVisible")
	procGetWindowLongW          = runningUser32.NewProc("GetWindowLongW")
	procGetWindowOwner          = runningUser32.NewProc("GetWindow")
	procGetClassNameW           = runningUser32.NewProc("GetClassNameW")
	procDwmGetWindowAttr        = runningDwm.NewProc("DwmGetWindowAttribute")
	procGetFileVersionInfoSizeW = runningVersion.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW     = runningVersion.NewProc("GetFileVersionInfoW")
	procVerQueryValueW          = runningVersion.NewProc("VerQueryValueW")
	procGetAppUserModelID       = syscall.NewLazyDLL("kernel32.dll").NewProc("GetApplicationUserModelId")
)

const (
	gwlExStyle     = ^uintptr(19) // GWL_EXSTYLE (-20)
	wsExToolWindow = 0x80
	gwOwner        = 4
	dwmwaCloaked   = 14

	appsFolderPrefix = `shell:AppsFolder\`
)

// listRunningWindows lists the programs with a visible top-level window, one
// per executable: further windows of the same program are left out, as the
// picker adds the program, not a window.
func listRunningWindows() []runningWindow {
	self := uint32(os.Getpid())
	seen := map[string]bool{}
	var out []runningWindow
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if visible, _, _ := procIsWindowVisible.Call(hwnd); visible == 0 {
			return 1
		}
		if style, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyle); uint32(style)&wsExToolWindow != 0 {
			return 1
		}
		// Owned windows are dialogs and popups of another window; the picker
		// adds their program through the owner.
		if owner, _, _ := procGetWindowOwner.Call(hwnd, gwOwner); owner != 0 {
			return 1
		}
		title := strings.TrimSpace(windowTitle(hwnd))
		if title == "" || title == "Program Manager" {
			return 1
		}
		// A cloaked window is one the shell keeps for a packaged app that is
		// not on screen, as its start tile's hidden copy.
		var cloaked uint32
		if r, _, _ := procDwmGetWindowAttr.Call(hwnd, dwmwaCloaked,
			uintptr(unsafe.Pointer(&cloaked)), unsafe.Sizeof(cloaked)); r == 0 && cloaked != 0 {
			return 1
		}
		var pid uint32
		procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == 0 || pid == self {
			return 1
		}
		path, ok := processImage(pid)
		if !ok || path == "" {
			return 1
		}
		// A packaged app's window is a frame of ApplicationFrameHost: the app
		// owns one of its child windows, and its AUMID is what starts it.
		if isFrameWindow(hwnd, path) {
			appID, appPID, ok := hostedApp(hwnd)
			if !ok {
				return 1
			}
			path, pid = appsFolderPrefix+appID, appPID
		} else {
			// The shell's own surfaces are not programs to add.
			switch strings.ToLower(filepath.Base(path)) {
			case "textinputhost.exe", "shellexperiencehost.exe",
				"startmenuexperiencehost.exe", "searchhost.exe", "lockapp.exe":
				return 1
			}
		}
		key := strings.ToLower(path)
		if seen[key] {
			return 1
		}
		seen[key] = true
		name := title
		if !isShellTarget(path) {
			name = appDisplayName(path)
			if name == "" || strings.EqualFold(name, filepath.Base(path)) {
				name = defaultTitle(path)
			}
		}
		out = append(out, runningWindow{Name: name, Title: title, Path: path, PID: pid})
		return 1
	})
	_ = windows.EnumWindows(cb, nil)
	return out
}

// isFrameWindow reports whether hwnd is the frame of a packaged app, which
// belongs to ApplicationFrameHost rather than to the app itself.
func isFrameWindow(hwnd uintptr, path string) bool {
	if strings.EqualFold(filepath.Base(path), "applicationframehost.exe") {
		return true
	}
	return strings.EqualFold(windowClass(hwnd), "ApplicationFrameWindow")
}

// hostedApp finds the package app in a frame window: the app owns a child
// window, while the frame belongs to the host, so its process has no AUMID.
func hostedApp(hwnd uintptr) (appID string, pid uint32, ok bool) {
	var framePID uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&framePID)))
	cb := syscall.NewCallback(func(child, _ uintptr) uintptr {
		var childPID uint32
		procGetWindowThreadProcess.Call(child, uintptr(unsafe.Pointer(&childPID)))
		if childPID == 0 || childPID == framePID {
			return 1
		}
		childPath, hasImage := processImage(childPID)
		if !hasImage || strings.EqualFold(filepath.Base(childPath), "applicationframehost.exe") {
			return 1
		}
		if id := appUserModelID(childPID); id != "" {
			appID, pid, ok = id, childPID, true
			return 0
		}
		return 1
	})
	windows.EnumChildWindows(windows.HWND(hwnd), cb, nil)
	return appID, pid, ok
}

// windowClass is the class name Windows registered hwnd under.
func windowClass(hwnd uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// appUserModelID is the application user model ID a process started with, ""
// when it has none.
func appUserModelID(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 512)
	size := uint32(len(buf))
	r, _, _ := procGetAppUserModelID.Call(
		uintptr(h), uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&buf[0])),
	)
	if int32(r) != 0 || size == 0 || size > uint32(len(buf)) {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}

// appDisplayName is the name the program gives itself in its version
// resource, as Explorer's Description column shows it: "Google Chrome", not
// "chrome.exe".
func appDisplayName(path string) string {
	if !fileExists(path) {
		return ""
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	size, _, _ := procGetFileVersionInfoSizeW.Call(uintptr(unsafe.Pointer(p)), 0)
	if size == 0 {
		return ""
	}
	data := make([]byte, size)
	if r, _, _ := procGetFileVersionInfoW.Call(
		uintptr(unsafe.Pointer(p)), 0, size, uintptr(unsafe.Pointer(&data[0])),
	); r == 0 {
		return ""
	}
	query := func(section string) (unsafe.Pointer, uint32, bool) {
		q, err := windows.UTF16PtrFromString(section)
		if err != nil {
			return nil, 0, false
		}
		var value unsafe.Pointer
		var length uint32
		r, _, _ := procVerQueryValueW.Call(
			uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(q)),
			uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&length)),
		)
		if r == 0 || value == nil || length == 0 {
			return nil, 0, false
		}
		return value, length, true
	}
	// The string block is named after a translation: its language and code
	// page, which the version resource lists; take the first with a name.
	if translations, length, ok := query(`\VarFileInfo\Translation`); ok {
		for _, pair := range unsafe.Slice((*uint32)(translations), int(length/4)) {
			key := fmt.Sprintf(`\StringFileInfo\%04x%04x\FileDescription`, uint16(pair), uint16(pair>>16))
			if value, _, ok := query(key); ok {
				if name := strings.TrimSpace(windows.UTF16PtrToString((*uint16)(value))); name != "" {
					return name
				}
			}
		}
	}
	return ""
}
