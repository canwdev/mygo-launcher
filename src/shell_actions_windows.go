//go:build windows

package main

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	seeMaskInvokeIDList = 0x0000000C
	cmfNormal           = 0x00000000
	tpmReturnCmd        = 0x0100
	tpmRightButton      = 0x0002
	cmicMaskAsyncOK     = 0x00000004
)

var (
	shellMenuUser32 = syscall.NewLazyDLL("user32.dll")

	procSHParseDisplayName = shell32.NewProc("SHParseDisplayName")
	procSHBindToParent     = shell32.NewProc("SHBindToParent")
	procCreatePopupMenu    = shellMenuUser32.NewProc("CreatePopupMenu")
	procDestroyMenu        = shellMenuUser32.NewProc("DestroyMenu")
	procTrackPopupMenuEx   = shellMenuUser32.NewProc("TrackPopupMenuEx")
	procGetCursorPos       = shellMenuUser32.NewProc("GetCursorPos")
	procGetForeground      = shellMenuUser32.NewProc("GetForegroundWindow")
	procGetMenuItemCount   = shellMenuUser32.NewProc("GetMenuItemCount")

	iidShellFolder = windows.GUID{
		Data1: 0x000214E6, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46},
	}
	iidContextMenu = windows.GUID{
		Data1: 0x000214E4, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46},
	}
)

type cursorPoint struct{ X, Y int32 }

// cmInvokeCommandInfo is CMINVOKECOMMANDINFO.
type cmInvokeCommandInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       uintptr // MAKEINTRESOURCE, not a pointer the GC knows
	lpParameters uintptr
	lpDirectory  uintptr
	nShow        int32
	dwHotKey     uint32
	hIcon        uintptr
}

// canUseShellActions reports whether the shell can show properties and its
// context menu for path: files, folders and shell folders, but not URLs or
// packaged apps, which have neither.
func canUseShellActions(path string) bool {
	if path == "" || isPackagedApp(path) {
		return false
	}
	if isShellNamespace(path) {
		return true
	}
	return !isShellTarget(path)
}

// menuOwner returns the window a shell menu or properties sheet belongs to:
// the foreground window, or the app's main window.
func menuOwner() uintptr {
	if h, _, _ := procGetForeground.Call(); h != 0 {
		return h
	}
	return findMainWindow()
}

// showItemProperties shows the shell's properties sheet for path.
func showItemProperties(path string) error {
	verb, err := windows.UTF16PtrFromString("properties")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	info := &shellExecuteInfo{
		fMask:  seeMaskInvokeIDList,
		hwnd:   menuOwner(),
		lpVerb: verb,
		lpFile: file,
		nShow:  swShowNormal,
	}
	info.cbSize = uint32(unsafe.Sizeof(*info))
	r, _, callErr := shellExecuteExProc.Call(uintptr(unsafe.Pointer(info)))
	if r == 0 {
		return fmt.Errorf("show properties of %q: %v", path, callErr)
	}
	return nil
}

// shellContextMenu builds the shell's context menu for path, and returns the
// menu and the IContextMenu behind it with a cleanup for both.
func shellContextMenu(path string) (hMenu, ctx uintptr, cleanup func(), err error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, nil, err
	}
	var pidl uintptr
	var attrs uint32
	r, _, _ := procSHParseDisplayName.Call(uintptr(unsafe.Pointer(name)), 0,
		uintptr(unsafe.Pointer(&pidl)), 0, uintptr(unsafe.Pointer(&attrs)))
	if int32(r) < 0 || pidl == 0 {
		return 0, 0, nil, fmt.Errorf("find %q: 0x%08x", path, uint32(int32(r)))
	}
	defer procCoTaskMemFree.Call(pidl)

	var parent, child uintptr
	r, _, _ = procSHBindToParent.Call(pidl, uintptr(unsafe.Pointer(&iidShellFolder)),
		uintptr(unsafe.Pointer(&parent)), uintptr(unsafe.Pointer(&child)))
	if int32(r) < 0 || parent == 0 {
		return 0, 0, nil, fmt.Errorf("open the folder of %q: 0x%08x", path, uint32(int32(r)))
	}
	defer comRelease(parent)

	item := [1]uintptr{child}
	r, _, _ = syscall.SyscallN(comMethod(parent, 10), parent, 0, 1,
		uintptr(unsafe.Pointer(&item[0])), uintptr(unsafe.Pointer(&iidContextMenu)), 0,
		uintptr(unsafe.Pointer(&ctx)))
	if int32(r) < 0 || ctx == 0 {
		return 0, 0, nil, fmt.Errorf("read the shell menu of %q: 0x%08x", path, uint32(int32(r)))
	}

	hMenu, _, _ = procCreatePopupMenu.Call()
	if hMenu == 0 {
		comRelease(ctx)
		return 0, 0, nil, errors.New("create the shell menu")
	}
	r, _, _ = syscall.SyscallN(comMethod(ctx, 3), ctx, hMenu, 0, 1, 0x7FFF, cmfNormal)
	if int32(r) < 0 {
		procDestroyMenu.Call(hMenu)
		comRelease(ctx)
		return 0, 0, nil, fmt.Errorf("fill the shell menu of %q: 0x%08x", path, uint32(int32(r)))
	}
	if count, _, _ := procGetMenuItemCount.Call(hMenu); count == 0 {
		procDestroyMenu.Call(hMenu)
		comRelease(ctx)
		return 0, 0, nil, fmt.Errorf("the shell has no menu for %q", path)
	}
	cleanup = func() {
		procDestroyMenu.Call(hMenu)
		comRelease(ctx)
	}
	return hMenu, ctx, cleanup, nil
}

// showShellMenu shows the shell's own context menu for path at the pointer,
// and runs the command chosen. It blocks until the menu closes.
func showShellMenu(path string) error {
	hMenu, ctx, cleanup, err := shellContextMenu(path)
	if err != nil {
		return err
	}
	defer cleanup()

	var pt cursorPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	cmd, _, _ := procTrackPopupMenuEx.Call(hMenu, tpmReturnCmd|tpmRightButton,
		uintptr(pt.X), uintptr(pt.Y), menuOwner(), 0)
	if cmd == 0 {
		return nil // dismissed
	}
	info := cmInvokeCommandInfo{
		fMask:  cmicMaskAsyncOK,
		lpVerb: cmd - 1, // MAKEINTRESOURCE(cmd - 1)
		nShow:  swShowNormal,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	r, _, _ := syscall.SyscallN(comMethod(ctx, 4), ctx, uintptr(unsafe.Pointer(&info)))
	if int32(r) < 0 {
		return fmt.Errorf("run the shell menu command: 0x%08x", uint32(int32(r)))
	}
	return nil
}

// nativeAddr turns an address of memory not managed by Go into a pointer.
func nativeAddr(addr uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&addr)) }

// comMethod returns the method at index of a COM object's vtable.
func comMethod(obj uintptr, index int) uintptr {
	vtable := *(*uintptr)(nativeAddr(obj))
	return *(*uintptr)(unsafe.Add(nativeAddr(vtable), uintptr(index)*unsafe.Sizeof(uintptr(0))))
}

// comRelease releases a COM object.
func comRelease(obj uintptr) {
	if obj != 0 {
		syscall.SyscallN(comMethod(obj, 2), obj)
	}
}
