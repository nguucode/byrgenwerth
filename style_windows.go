//go:build windows

package main

import (
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

var (
	gdi32                   = syscall.NewLazyDLL("gdi32.dll")
	procEnumFontFamiliesExW = gdi32.NewProc("EnumFontFamiliesExW")

	fontNames map[string]bool
	fontCb    = syscall.NewCallback(func(lf, tm, fontType, lparam uintptr) uintptr {
		l := (*win.LOGFONT)(unsafe.Pointer(lf))
		name := syscall.UTF16ToString(l.LfFaceName[:])
		if name != "" && !strings.HasPrefix(name, "@") { // @ = vertical variants
			fontNames[name] = true
		}
		return 1
	})
)

// InstalledFonts lists font family names, sorted. UI thread only.
func InstalledFonts() []string {
	fontNames = map[string]bool{}
	dc := win.GetDC(0)
	defer win.ReleaseDC(0, dc)
	var lf win.LOGFONT
	lf.LfCharSet = 1 // DEFAULT_CHARSET: all families
	procEnumFontFamiliesExW.Call(uintptr(dc), uintptr(unsafe.Pointer(&lf)), fontCb, 0, 0)
	out := make([]string, 0, len(fontNames))
	for n := range fontNames {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	fontNames = nil
	return out
}

var customColors [16]win.COLORREF

// PickColor opens the Windows color dialog.
func PickColor(owner win.HWND, initial RGB) (RGB, bool) {
	var cc win.CHOOSECOLOR
	cc.LStructSize = uint32(unsafe.Sizeof(cc))
	cc.HwndOwner = owner
	cc.RgbResult = win.RGB(initial.R, initial.G, initial.B)
	cc.LpCustColors = &customColors
	cc.Flags = win.CC_RGBINIT | win.CC_FULLOPEN
	if !win.ChooseColor(&cc) {
		return initial, false
	}
	c := uint32(cc.RgbResult)
	return RGB{uint8(c), uint8(c >> 8), uint8(c >> 16)}, true
}
