//go:build windows

package main

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

var (
	procUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	overlays                = map[win.HWND]*Overlay{}
	overlayClassName, _     = syscall.UTF16PtrFromString("ByrgenwerthOverlay")
	overlayClassRegistered  bool
	overlayWndProc          = syscall.NewCallback(func(hwndp, msgp, wp, lp uintptr) uintptr {
		hwnd, msg := win.HWND(hwndp), uint32(msgp)
		o := overlays[hwnd]
		switch msg {
		case win.WM_NCHITTEST:
			if o != nil && o.editing {
				return win.HTCAPTION // drag anywhere to move
			}
		case win.WM_MOUSEACTIVATE:
			return 3 // MA_NOACTIVATE: never steal focus from the game
		case win.WM_EXITSIZEMOVE:
			if o != nil {
				var r win.RECT
				win.GetWindowRect(hwnd, &r)
				o.X, o.Y = r.Left, r.Top
				if o.OnMoved != nil {
					o.OnMoved(o.X, o.Y)
				}
			}
			return 0
		}
		return win.DefWindowProc(hwnd, msg, wp, lp)
	})
)

// Overlay is a borderless, always-on-top, click-through layered window
// that shows text with per-pixel alpha. In edit mode it becomes draggable.
type Overlay struct {
	hwnd         win.HWND
	X, Y         int32
	Text         string
	FontSize     int
	FontFamily   string
	Bold         bool
	Color        RGB
	OutlineWidth int // 0 = no outline
	OutlineColor RGB
	Subtitle     *SubtitleState // when set, draws the translation box instead of Text
	editing      bool
	OnMoved      func(x, y int32)
}

const (
	exBase    = win.WS_EX_LAYERED | win.WS_EX_TOPMOST | win.WS_EX_TOOLWINDOW | win.WS_EX_NOACTIVATE
	exClickTh = win.WS_EX_TRANSPARENT
)

func NewOverlay(x, y int32) (*Overlay, error) {
	hinst := win.GetModuleHandle(nil)
	if !overlayClassRegistered {
		var wc win.WNDCLASSEX
		wc.CbSize = uint32(unsafe.Sizeof(wc))
		wc.LpfnWndProc = overlayWndProc
		wc.HInstance = hinst
		cur, _, _ := procLoadCursorW.Call(0, uintptr(win.IDC_SIZEALL))
		wc.HCursor = win.HCURSOR(cur)
		wc.LpszClassName = overlayClassName
		if win.RegisterClassEx(&wc) == 0 {
			return nil, syscall.GetLastError()
		}
		overlayClassRegistered = true
	}
	title, _ := syscall.UTF16PtrFromString("Byrgenwerth Overlay")
	o := &Overlay{X: x, Y: y, FontSize: 40, FontFamily: "Segoe UI", Bold: true, Color: RGB{255, 255, 255}, OutlineWidth: 3}
	o.hwnd = win.CreateWindowEx(exBase|exClickTh, overlayClassName, title, win.WS_POPUP,
		x, y, 10, 10, 0, 0, hinst, nil)
	if o.hwnd == 0 {
		return nil, syscall.GetLastError()
	}
	overlays[o.hwnd] = o
	return o, nil
}

func (o *Overlay) Show(show bool) {
	if o == nil {
		return
	}
	if show {
		win.ShowWindow(o.hwnd, win.SW_SHOWNOACTIVATE)
		o.KeepOnTop()
	} else {
		win.ShowWindow(o.hwnd, win.SW_HIDE)
	}
}

// KeepOnTop re-asserts topmost; borderless games sometimes push themselves above.
func (o *Overlay) KeepOnTop() {
	if o == nil {
		return
	}
	win.SetWindowPos(o.hwnd, win.HWND_TOPMOST, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
}

func (o *Overlay) SetEditing(on bool) {
	if o == nil {
		return
	}
	o.editing = on
	ex := uint32(exBase)
	if !on {
		ex |= exClickTh
	}
	win.SetWindowLong(o.hwnd, win.GWL_EXSTYLE, int32(ex))
	o.Render()
}

func (o *Overlay) Editing() bool { return o != nil && o.editing }

func (o *Overlay) MoveTo(x, y int32) {
	if o == nil {
		return
	}
	o.X, o.Y = x, y
	o.Render()
}

func (o *Overlay) Destroy() {
	if o == nil {
		return
	}
	delete(overlays, o.hwnd)
	win.DestroyWindow(o.hwnd)
}

// Render draws the text into a 32-bit DIB and pushes it with UpdateLayeredWindow.
func (o *Overlay) Render() {
	if o == nil || o.hwnd == 0 {
		return
	}
	if o.Subtitle != nil {
		o.renderSubtitle()
		return
	}
	text := o.Text
	if text == "" {
		text = " "
	}
	screenDC := win.GetDC(0)
	defer win.ReleaseDC(0, screenDC)
	memDC := win.CreateCompatibleDC(screenDC)
	defer win.DeleteDC(memDC)

	var lf win.LOGFONT
	lf.LfHeight = -int32(o.FontSize)
	lf.LfWeight = 400
	if o.Bold {
		lf.LfWeight = win.FW_BOLD
	}
	lf.LfCharSet = 1 // DEFAULT_CHARSET
	lf.LfQuality = win.ANTIALIASED_QUALITY
	family := o.FontFamily
	if family == "" {
		family = "Segoe UI"
	}
	face, _ := syscall.UTF16FromString(family)
	copy(lf.LfFaceName[:len(lf.LfFaceName)-1], face)
	font := win.CreateFontIndirect(&lf)
	defer win.DeleteObject(win.HGDIOBJ(font))
	oldFont := win.SelectObject(memDC, win.HGDIOBJ(font))
	defer win.SelectObject(memDC, oldFont)

	t16, _ := syscall.UTF16FromString(text)
	n := int32(len(t16) - 1)
	var sz win.SIZE
	win.GetTextExtentPoint32(memDC, &t16[0], n, &sz)

	outline := o.OutlineWidth
	if outline < 0 {
		outline = 0
	}
	pad := int32(outline + 4)
	if o.editing {
		pad += 8
	}
	w := sz.CX + 2*pad
	h := sz.CY + 2*pad

	var bih win.BITMAPINFOHEADER
	bih.BiSize = uint32(unsafe.Sizeof(bih))
	bih.BiWidth = w
	bih.BiHeight = -h
	bih.BiPlanes = 1
	bih.BiBitCount = 32
	bih.BiCompression = win.BI_RGB
	var bits unsafe.Pointer
	dib := win.CreateDIBSection(memDC, &bih, win.DIB_RGB_COLORS, &bits, 0, 0)
	if dib == 0 || bits == nil {
		return
	}
	defer win.DeleteObject(win.HGDIOBJ(dib))
	oldBmp := win.SelectObject(memDC, win.HGDIOBJ(dib))
	defer win.SelectObject(memDC, oldBmp)

	pix := unsafe.Slice((*byte)(bits), int(w*h*4))
	win.SetBkMode(memDC, win.TRANSPARENT)
	win.SetTextColor(memDC, win.RGB(255, 255, 255))

	mask := func(offsets [][2]int) []uint8 {
		for i := range pix {
			pix[i] = 0
		}
		for _, d := range offsets {
			win.TextOut(memDC, pad+int32(d[0]), pad+int32(d[1]), &t16[0], n)
		}
		win.GdiFlush()
		m := make([]uint8, w*h)
		for i := range m {
			b, g, r := pix[i*4], pix[i*4+1], pix[i*4+2]
			mx := r
			if g > mx {
				mx = g
			}
			if b > mx {
				mx = b
			}
			m[i] = mx
		}
		return m
	}
	fill := mask([][2]int{{0, 0}})
	out := make([]uint8, len(fill))
	if outline > 0 {
		out = mask(OutlineOffsets(outline))
	}
	copy(pix, ComposeOverlay(fill, out, int(w), int(h), o.Color, o.OutlineColor, o.editing))

	dst := win.POINT{X: o.X, Y: o.Y}
	size := win.SIZE{CX: w, CY: h}
	src := win.POINT{}
	blend := win.BLENDFUNCTION{BlendOp: 0, SourceConstantAlpha: 255, AlphaFormat: win.AC_SRC_ALPHA}
	procUpdateLayeredWindow.Call(uintptr(o.hwnd), uintptr(screenDC),
		uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&size)),
		uintptr(memDC), uintptr(unsafe.Pointer(&src)), 0,
		uintptr(unsafe.Pointer(&blend)), 2 /* ULW_ALPHA */)
}

// gdiFont creates a font; caller deletes it.
func gdiFont(family string, px int, bold bool) win.HFONT {
	var lf win.LOGFONT
	lf.LfHeight = -int32(px)
	lf.LfWeight = 400
	if bold {
		lf.LfWeight = win.FW_BOLD
	}
	lf.LfCharSet = 1
	lf.LfQuality = win.ANTIALIASED_QUALITY
	if family == "" {
		family = "Segoe UI"
	}
	face, _ := syscall.UTF16FromString(family)
	copy(lf.LfFaceName[:len(lf.LfFaceName)-1], face)
	return win.CreateFontIndirect(&lf)
}
