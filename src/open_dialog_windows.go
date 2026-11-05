//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
	"golang.org/x/sys/windows"
)

// The open dialog for adding files keeps shortcuts as shortcuts: MyGo's
// Dialog.Open dereferences .lnk items, so a picked shortcut would come back
// as its target, without the arguments that make it what it is.
//
// It is IFileOpenDialog, as MyGo's dialog is, with FOS_NODEREFERENCELINKS.

var (
	clsidFileOpenDialog = windows.GUID{
		Data1: 0xdc1c5a9c, Data2: 0xe88a, Data3: 0x4dde,
		Data4: [8]byte{0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7},
	}
	iidFileOpenDialog = windows.GUID{
		Data1: 0xd57c7288, Data2: 0xd4ad, Data3: 0x4768,
		Data4: [8]byte{0xbe, 0x02, 0x9d, 0x96, 0x95, 0x32, 0xd9, 0x60},
	}
)

// IFileOpenDialog vtable indices.
const (
	dialogShow       = 3
	dialogSetOptions = 9
	dialogGetOptions = 10
	dialogSetTitle   = 17
	dialogResults    = 27

	itemsCount  = 7
	itemsItemAt = 8

	itemFileSysPath = 5
)

const (
	sigdnFileSysPath      = 0x80058000
	fosAllowMultiselect   = 0x200
	fosForceFileSystem    = 0x40
	fosPathMustExist      = 0x800
	fosFileMustExist      = 0x1000
	fosNoDereferenceLinks = 0x100000

	hresultCancelled = 0x800704C7
)

// openFilesDialog asks for files, returning shortcuts as the .lnk paths the
// launcher parses itself.
func openFilesDialog(parent *mygo.Window, title string) ([]string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ok, uninit := coInitCOM()
	if !ok {
		return nil, fmt.Errorf("CoInitializeEx failed")
	}
	if uninit {
		defer procCoUninitialize.Call()
	}

	var dlg uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		uintptr(clctxInprocServer),
		uintptr(unsafe.Pointer(&iidFileOpenDialog)),
		uintptr(unsafe.Pointer(&dlg)),
	)
	if int32(hr) < 0 || dlg == 0 {
		return nil, fmt.Errorf("CoCreateInstance(FileOpenDialog) failed: 0x%08x", uint32(int32(hr)))
	}
	defer dialogRelease(dlg)

	var opts uint32
	dialogCall(dlg, dialogGetOptions, uintptr(unsafe.Pointer(&opts)))
	dialogCall(dlg, dialogSetOptions, uintptr(opts|
		fosAllowMultiselect|fosForceFileSystem|fosPathMustExist|fosFileMustExist|fosNoDereferenceLinks))
	if title != "" {
		if p, err := windows.UTF16PtrFromString(title); err == nil {
			dialogCall(dlg, dialogSetTitle, uintptr(unsafe.Pointer(p)))
		}
	}

	var owner uintptr
	if parent != nil {
		owner = parent.NativeHandle()
	}
	hr = dialogCall(dlg, dialogShow, owner)
	if uint32(hr) == hresultCancelled {
		return nil, nil
	}
	if int32(hr) < 0 {
		return nil, fmt.Errorf("the file dialog failed: 0x%08x", uint32(int32(hr)))
	}

	var items uintptr
	if hr = dialogCall(dlg, dialogResults, uintptr(unsafe.Pointer(&items))); int32(hr) < 0 || items == 0 {
		return nil, fmt.Errorf("the file dialog returned no results: 0x%08x", uint32(int32(hr)))
	}
	defer dialogRelease(items)

	var n uint32
	dialogCall(items, itemsCount, uintptr(unsafe.Pointer(&n)))
	paths := make([]string, 0, n)
	for i := uint32(0); i < n; i++ {
		var item uintptr
		if int32(dialogCall(items, itemsItemAt, uintptr(i), uintptr(unsafe.Pointer(&item)))) < 0 || item == 0 {
			continue
		}
		if p := dialogItemPath(item); p != "" {
			paths = append(paths, p)
		}
		dialogRelease(item)
	}
	return paths, nil
}

// dialogCall calls method index of the COM object obj.
func dialogCall(obj uintptr, index int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(nativePtr(obj))
	fn := *(*uintptr)(nativePtr(vtbl + uintptr(index)*unsafe.Sizeof(uintptr(0))))
	var a [8]uintptr
	a[0] = obj
	copy(a[1:], args)
	r, _, _ := syscall.SyscallN(fn, a[:len(args)+1]...)
	return r
}

func dialogRelease(obj uintptr) {
	if obj != 0 {
		dialogCall(obj, 2)
	}
}

// dialogItemPath is the filesystem path of an IShellItem, "" for one without.
func dialogItemPath(item uintptr) string {
	var p uintptr
	if int32(dialogCall(item, itemFileSysPath, sigdnFileSysPath, uintptr(unsafe.Pointer(&p)))) < 0 || p == 0 {
		return ""
	}
	defer procCoTaskMemFree.Call(p)
	return windows.UTF16PtrToString((*uint16)(nativePtr(p)))
}

// nativePtr turns an address a native call returned into a pointer.
func nativePtr(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}
