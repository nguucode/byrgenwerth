//go:build windows

package main

import (
	"errors"
	"fmt"
	"image"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procRegisterHotKey      = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey    = user32.NewProc("UnregisterHotKey")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
)

type Monitor struct {
	Rect    win.RECT
	Primary bool
}

func (m Monitor) Label(i int) string {
	w := m.Rect.Right - m.Rect.Left
	h := m.Rect.Bottom - m.Rect.Top
	s := fmt.Sprintf("Màn hình %d (%dx%d)", i+1, w, h)
	if m.Primary {
		s += " - chính"
	}
	return s
}

// enumCb is created once: Go limits how many syscall callbacks can exist.
var (
	enumMons []Monitor
	enumCb   = syscall.NewCallback(func(hMon, hdc, rect, lparam uintptr) uintptr {
		var mi win.MONITORINFO
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		if win.GetMonitorInfo(win.HMONITOR(hMon), &mi) {
			enumMons = append(enumMons, Monitor{Rect: mi.RcMonitor, Primary: mi.DwFlags&1 != 0})
		}
		return 1
	})
)

// ListMonitors returns all monitors, primary first. Call from the UI thread only.
func ListMonitors() []Monitor {
	enumMons = nil
	procEnumDisplayMonitors.Call(0, 0, enumCb, 0)
	mons := enumMons
	enumMons = nil
	// primary first, keep the rest in system order
	for i, m := range mons {
		if m.Primary && i != 0 {
			mons[0], mons[i] = mons[i], mons[0]
			break
		}
	}
	if len(mons) == 0 {
		mons = append(mons, Monitor{Rect: win.RECT{Right: win.GetSystemMetrics(win.SM_CXSCREEN), Bottom: win.GetSystemMetrics(win.SM_CYSCREEN)}, Primary: true})
	}
	return mons
}

// CaptureMonitor grabs the monitor scaled down to CaptureW x CaptureH.
func CaptureMonitor(m Monitor) (*image.RGBA, error) {
	return CaptureMonitorScaled(m, CaptureW, CaptureH)
}

// OcrSize keeps the monitor's aspect ratio at 1280 px wide (enough for banner text).
func OcrSize(m Monitor) (int32, int32) {
	w := m.Rect.Right - m.Rect.Left
	h := m.Rect.Bottom - m.Rect.Top
	if w <= 0 || h <= 0 {
		return 1280, 720
	}
	// 1920 keeps small labels (boss names on a 4K screen) readable for OCR.
	ow := int32(1920)
	if w < ow {
		ow = w
	}
	oh := ow * h / w
	return ow, oh &^ 1
}

// CaptureMonitorScaled grabs the monitor scaled to cw x ch.
// Scaling happens in GDI (HALFTONE), so we never copy a full 4K frame to Go.
func CaptureMonitorScaled(m Monitor, cw, ch int32) (*image.RGBA, error) {
	return captureRect(m.Px(), cw, ch)
}

// Px is the monitor's rectangle on the virtual screen.
func (m Monitor) Px() PxRect {
	return PxRect{m.Rect.Left, m.Rect.Top, m.Rect.Right, m.Rect.Bottom}
}

// CaptureRegion grabs a screen rectangle for OCR, no wider than 1920 px.
func CaptureRegion(r PxRect) (*image.RGBA, error) {
	w, h := r.R-r.L, r.B-r.T
	if w <= 0 || h <= 0 {
		return nil, errors.New("vùng khoanh rỗng")
	}
	cw := min(w, 1920)
	return captureRect(r, cw, max(h*cw/w, 1))
}

func captureRect(r PxRect, cw, ch int32) (*image.RGBA, error) {
	screenDC := win.GetDC(0)
	if screenDC == 0 {
		return nil, errors.New("GetDC failed")
	}
	defer win.ReleaseDC(0, screenDC)

	memDC := win.CreateCompatibleDC(screenDC)
	if memDC == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	defer win.DeleteDC(memDC)

	bmp := win.CreateCompatibleBitmap(screenDC, cw, ch)
	if bmp == 0 {
		return nil, errors.New("CreateCompatibleBitmap failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(bmp))

	old := win.SelectObject(memDC, win.HGDIOBJ(bmp))
	win.SetStretchBltMode(memDC, win.HALFTONE)
	win.SetBrushOrgEx(memDC, 0, 0, nil)
	ok := win.StretchBlt(memDC, 0, 0, cw, ch, screenDC, r.L, r.T, r.R-r.L, r.B-r.T, win.SRCCOPY|win.CAPTUREBLT)
	win.SelectObject(memDC, old)
	if !ok {
		return nil, errors.New("StretchBlt failed")
	}

	var bi win.BITMAPINFO
	bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bi.BmiHeader))
	bi.BmiHeader.BiWidth = cw
	bi.BmiHeader.BiHeight = -ch // top-down
	bi.BmiHeader.BiPlanes = 1
	bi.BmiHeader.BiBitCount = 32
	bi.BmiHeader.BiCompression = win.BI_RGB

	img := image.NewRGBA(image.Rect(0, 0, int(cw), int(ch)))
	if win.GetDIBits(memDC, bmp, 0, uint32(ch), &img.Pix[0], &bi, win.DIB_RGB_COLORS) == 0 {
		return nil, errors.New("GetDIBits failed")
	}
	// BGRA -> RGBA, force opaque
	p := img.Pix
	for i := 0; i < len(p); i += 4 {
		p[i], p[i+2] = p[i+2], p[i]
		p[i+3] = 255
	}
	return img, nil
}
