//go:build windows

package main

import (
	"runtime"
	"unsafe"

	"github.com/egoist/mygo"
	"golang.org/x/sys/windows"
)

// The "Change Icon" dialog Explorer shows under Change Icon: a file field with
// a Browse button and the icons of that file. PickIconDlg is exported by name
// but not documented, and has kept its signature since Windows 2000.

var procPickIconDlg = iconShell32.NewProc("PickIconDlg")

// pickIconFromShell asks for an icon file and an index with the shell's own
// picker, starting at path and index. ok is false when the dialog was closed.
func pickIconFromShell(parent *mygo.Window, path string, index int) (string, int, bool) {
	if procPickIconDlg.Find() != nil {
		return "", 0, false
	}

	// The dialog runs its own message loop, which needs the thread that made it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ok, uninit := coInitCOM()
	if !ok {
		return "", 0, false
	}
	if uninit {
		defer procCoUninitialize.Call()
	}

	buf := make([]uint16, windows.MAX_PATH)
	start := windows.StringToUTF16(path)
	copy(buf, start[:min(len(start), len(buf)-1)])
	i := int32(index)

	var owner uintptr
	if parent != nil {
		owner = parent.NativeHandle()
	}
	r, _, _ := procPickIconDlg.Call(owner, uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)), uintptr(unsafe.Pointer(&i)))
	if r == 0 {
		return "", 0, false
	}
	return windows.UTF16ToString(buf), int(i), true
}
