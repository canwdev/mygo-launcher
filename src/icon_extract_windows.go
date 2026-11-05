//go:build windows

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"syscall"
	"unsafe"
)

// Icons inside .ico files and the icon resources of .exe and .dll files: the
// shell's file icon shows a generic document for a DLL, so its resources are
// read directly.

var (
	iconShell32 = syscall.NewLazyDLL("shell32.dll")
	iconUser32  = syscall.NewLazyDLL("user32.dll")
	iconGdi32   = syscall.NewLazyDLL("gdi32.dll")

	procExtractIconEx    = iconShell32.NewProc("ExtractIconExW")
	procSHDefExtractIcon = iconShell32.NewProc("SHDefExtractIconW")
	procGetIconInfo      = iconUser32.NewProc("GetIconInfo")
	procDestroyIcon      = iconUser32.NewProc("DestroyIcon")
	procGetDC            = iconUser32.NewProc("GetDC")
	procReleaseDC        = iconUser32.NewProc("ReleaseDC")
	procGetObjectW       = iconGdi32.NewProc("GetObjectW")
	procGetDIBits        = iconGdi32.NewProc("GetDIBits")
	procDeleteObject     = iconGdi32.NewProc("DeleteObject")
)

type iconInfo struct {
	icon     uint32
	xHotspot uint32
	yHotspot uint32
	mask     uintptr
	color    uintptr
}

type gdiBitmap struct {
	kind       int32
	width      int32
	height     int32
	widthBytes int32
	planes     uint16
	bitsPixel  uint16
	bits       uintptr
}

type dibHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

// extractIconResource draws icon index of path, an .ico or the icon resources
// of an .exe or .dll, as PNG data at px pixels. It returns nil when the file
// holds no icon at that index.
func extractIconResource(path string, px, index int) []byte {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil
	}
	var large, small uintptr
	// The low word is the large size and the high word the small one.
	size := uintptr(px) | uintptr(16)<<16
	procSHDefExtractIcon.Call(
		uintptr(unsafe.Pointer(p)), uintptr(index), 0,
		uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), size)
	if small != 0 {
		procDestroyIcon.Call(small)
	}
	if large != 0 {
		return hiconPNG(large)
	}
	// The system's own sizes, for a file SHDefExtractIcon will not scale.
	n, _, _ := procExtractIconEx.Call(
		uintptr(unsafe.Pointer(p)), uintptr(index),
		uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1)
	if small != 0 {
		procDestroyIcon.Call(small)
	}
	if int(n) > 0 && large != 0 {
		return hiconPNG(large)
	}
	return nil
}

// hiconPNG draws an icon as a PNG, with the alpha channel of a 32-bit icon,
// or made from its mask.
func hiconPNG(h uintptr) []byte {
	if h == 0 {
		return nil
	}
	defer procDestroyIcon.Call(h)

	var ii iconInfo
	if r, _, _ := procGetIconInfo.Call(h, uintptr(unsafe.Pointer(&ii))); r == 0 {
		return nil
	}
	defer func() {
		if ii.color != 0 {
			procDeleteObject.Call(ii.color)
		}
		if ii.mask != 0 {
			procDeleteObject.Call(ii.mask)
		}
	}()

	mono := ii.color == 0
	bmp := ii.color
	if mono {
		bmp = ii.mask
	}
	var bm gdiBitmap
	if r, _, _ := procGetObjectW.Call(bmp, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm))); r == 0 {
		return nil
	}
	w, hgt := int(bm.width), int(bm.height)
	if mono {
		hgt /= 2 // the mask holds the AND and the XOR masks
	}
	if w <= 0 || hgt <= 0 || w > 1024 || hgt > 1024 {
		return nil
	}

	header := dibHeader{
		size: uint32(unsafe.Sizeof(dibHeader{})), width: int32(w), height: int32(-hgt),
		planes: 1, bitCount: 32,
	}
	pixels := make([]byte, w*hgt*4)
	dc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, dc)
	if !mono {
		if r, _, _ := procGetDIBits.Call(dc, ii.color, 0, uintptr(hgt),
			uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&header)), 0); r == 0 {
			return nil
		}
	}

	// Old icons keep their transparency in the mask; 32-bit ones have it in
	// the color bitmap's fourth byte.
	opaque := false
	for i := 3; i < len(pixels); i += 4 {
		if pixels[i] != 0 {
			opaque = true
			break
		}
	}
	if (mono || !opaque) && ii.mask != 0 {
		rowBytes := ((w + 31) / 32) * 4
		mask := make([]byte, rowBytes*hgt)
		maskHeader := dibHeader{
			size: uint32(unsafe.Sizeof(dibHeader{})), width: int32(w), height: int32(-hgt),
			planes: 1, bitCount: 1,
		}
		procGetDIBits.Call(dc, ii.mask, 0, uintptr(hgt),
			uintptr(unsafe.Pointer(&mask[0])), uintptr(unsafe.Pointer(&maskHeader)), 0)
		for y := 0; y < hgt; y++ {
			for x := 0; x < w; x++ {
				bit := (mask[y*rowBytes+x/8] >> (7 - uint(x%8))) & 1
				i := (y*w + x) * 4
				if bit == 0 {
					pixels[i+3] = 255
					if mono {
						pixels[i], pixels[i+1], pixels[i+2] = 255, 255, 255
					}
				} else {
					pixels[i+3] = 0
					if mono {
						pixels[i], pixels[i+1], pixels[i+2] = 0, 0, 0
					}
				}
			}
		}
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, hgt))
	for y := 0; y < hgt; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			img.SetNRGBA(x, y, color.NRGBA{R: pixels[i+2], G: pixels[i+1], B: pixels[i], A: pixels[i+3]})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil
	}
	return b.Bytes()
}
