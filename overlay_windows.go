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
	overlayClassName, _     = syscall.UTF16PtrFromString("DeathCounterOverlay")
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
	Splits       *SplitsState // when set, draws the speedrun-style boss panel instead of Text
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
	title, _ := syscall.UTF16PtrFromString("Death Counter Overlay")
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
	if o.Splits != nil {
		o.renderPanel()
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

// renderPanel draws the boss panel laid out by LayoutSplits: text and rules in
// the overlay text color, with the same outline, on a transparent background.
func (o *Overlay) renderPanel() {
	screenDC := win.GetDC(0)
	defer win.ReleaseDC(0, screenDC)
	memDC := win.CreateCompatibleDC(screenDC)
	defer win.DeleteDC(memDC)

	type fkey struct {
		px   int
		bold bool
	}
	fonts := map[fkey]win.HFONT{}
	defer func() {
		for _, f := range fonts {
			win.DeleteObject(win.HGDIOBJ(f))
		}
	}()
	use := func(px int, bold bool) {
		k := fkey{px, bold}
		f, ok := fonts[k]
		if !ok {
			f = gdiFont(o.FontFamily, px, bold)
			fonts[k] = f
		}
		win.SelectObject(memDC, win.HGDIOBJ(f))
	}
	oldFont := win.SelectObject(memDC, win.GetStockObject(win.DEFAULT_GUI_FONT))
	defer win.SelectObject(memDC, oldFont)
	measure := func(text string, px int, bold bool) (int, int) {
		use(px, bold)
		t16, _ := syscall.UTF16FromString(text)
		var sz win.SIZE
		if len(t16) > 1 {
			win.GetTextExtentPoint32(memDC, &t16[0], int32(len(t16)-1), &sz)
		} else {
			win.GetTextExtentPoint32(memDC, &[]uint16{'A', 0}[0], 1, &sz)
			sz.CX = 0
		}
		return int(sz.CX), int(sz.CY)
	}

	W, H, rules, texts := LayoutSplits(*o.Splits, o.FontSize, measure)
	ow := max(o.OutlineWidth, 0)
	edge := ow + 4
	if o.editing {
		edge += 8
	}
	w, h := int32(W+2*edge), int32(H+2*edge)

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

	// Cut long texts to one line and resolve right alignment once.
	type placed struct {
		x, y, px int
		bold     bool
		t16      []uint16
	}
	var items []placed
	for _, tx := range texts {
		text := tx.Text
		tw, _ := measure(text, tx.Px, tx.Bold)
		if tx.MaxW > 0 && tw > tx.MaxW {
			r := []rune(text)
			for len(r) > 1 {
				r = r[:len(r)-1]
				if cw, _ := measure(string(r)+"…", tx.Px, tx.Bold); cw <= tx.MaxW {
					break
				}
			}
			text = string(r) + "…"
			tw, _ = measure(text, tx.Px, tx.Bold)
		}
		x := tx.X
		if tx.Right {
			x -= tw
		}
		t16, _ := syscall.UTF16FromString(text)
		items = append(items, placed{x + edge, tx.Y + edge, tx.Px, tx.Bold, t16})
	}

	win.SetBkMode(memDC, win.TRANSPARENT)
	win.SetTextColor(memDC, win.RGB(255, 255, 255))
	// mask stamps every text and rule at each offset and returns the coverage.
	mask := func(offsets [][2]int) []uint8 {
		for i := range pix {
			pix[i] = 0
		}
		for _, d := range offsets {
			for _, it := range items {
				if len(it.t16) > 1 {
					use(it.px, it.bold)
					win.TextOut(memDC, int32(it.x+d[0]), int32(it.y+d[1]), &it.t16[0], int32(len(it.t16)-1))
				}
			}
		}
		win.GdiFlush()
		m := make([]uint8, w*h)
		for i := range m {
			m[i] = max(pix[i*4], pix[i*4+1], pix[i*4+2])
		}
		grow := 0
		if len(offsets) > 1 {
			grow = ow
		}
		for _, rc := range rules {
			for y := max(rc.Y+edge-grow, 0); y < min(rc.Y+edge+rc.H+grow, int(h)); y++ {
				for x := max(rc.X+edge-grow, 0); x < min(rc.X+edge+rc.W+grow, int(w)); x++ {
					m[y*int(w)+x] = 255
				}
			}
		}
		return m
	}
	fill := mask([][2]int{{0, 0}})
	out := make([]uint8, len(fill))
	if ow > 0 {
		out = mask(OutlineOffsets(ow))
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
