//go:build windows

package main

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

// translatePage has two tabs: the translation itself and its narration.
func (a *App) translatePage() Widget {
	hk := a.cfg.Hotkeys
	translateTab := TabPage{
		Title:  "Dịch",
		Layout: pageLayout(),
		Children: []Widget{
			row(CheckBox{AssignTo: &a.trOnCB, Text: "Bật dịch Anh → Việt", OnCheckedChanged: a.onTranslateToggled,
				ToolTipText: hk.TranslateToggle + " bật/tắt dịch"}),
			form(
				Label{Text: "Vùng"}, row(
					Label{AssignTo: &a.trRegionLbl, Text: "Chưa khoanh"},
					PushButton{Text: "Khoanh...", OnClicked: a.editTranslateRegion, ToolTipText: hk.TranslateRegion + " khoanh lại vùng hội thoại"},
				),
				Label{Text: "API key"}, LineEdit{AssignTo: &a.trKeyLE, PasswordMode: true, MinSize: inSize, MaxSize: inSize, OnEditingFinished: a.onAPIKeyChanged,
					ToolTipText: "API key Anthropic của bạn (console.anthropic.com), trả phí theo lượng dùng. Key được mã hóa theo tài khoản Windows."},
				Label{Text: "Cỡ phụ đề"}, row(
					NumberEdit{AssignTo: &a.trFontNE, MinValue: 10, MaxValue: 120, Decimals: 0, Value: float64(a.cfg.TranslateFontSize), SpinButtonsVisible: true, MinSize: inSize, MaxSize: inSize, OnValueChanged: a.onSubtitleFontChanged},
					Label{Text: "px"},
				),
			),
			hint("Chỉ chữ trong vùng khoanh, vài câu trước và tên game được gửi tới Anthropic để dịch. Game cần chạy Windowed hoặc Borderless."),
			sep(),
			title("Câu vừa đọc"),
			TextLabel{AssignTo: &a.trSrcLbl, Text: "--", MinSize: Size{Width: 100}},
			title("Bản dịch"),
			TextLabel{AssignTo: &a.trDstLbl, Text: "--", MinSize: Size{Width: 100}},
			Label{AssignTo: &a.trCostLbl, Text: "", TextColor: hintColor},
			VSpacer{},
		},
	}
	narrateTab := TabPage{
		Title:    "Thuyết minh",
		Layout:   pageLayout(),
		Children: append(a.narrationSection(), VSpacer{}),
	}
	return Composite{
		AssignTo: &a.transPage,
		Visible:  false,
		Layout:   VBox{MarginsZero: true, Alignment: AlignHNearVNear},
		Children: []Widget{TabWidget{Pages: []TabPage{translateTab, narrateTab}}},
	}
}

// showPage switches between the two top-level tabs.
func (a *App) showPage(translate bool) {
	a.deathPage.SetVisible(!translate)
	a.transPage.SetVisible(translate)
	active, other := a.navDeath, a.navTranslate
	if translate {
		active, other = other, active
	}
	active.SetFont(a.navBold)
	other.SetFont(a.navNormal)
}

func (a *App) initTranslation() {
	a.navNormal, _ = walk.NewFont("Segoe UI", 9, 0)
	a.navBold, _ = walk.NewFont("Segoe UI", 9, walk.FontBold)
	a.showPage(false)

	t := &a.translation
	t.ctx, t.cancel = context.WithCancel(context.Background())
	t.apiKey = LoadAPIKey(a.store.Dir)
	a.trKeyLE.SetText(t.apiKey)
	a.refreshTranslateRegionLabel()

	o, err := NewOverlay(0, 0)
	if err != nil {
		a.setStatus("Không tạo được phụ đề: " + err.Error())
		return
	}
	o.FontSize = a.cfg.TranslateFontSize
	o.FontFamily = "Segoe UI"
	o.Subtitle = &SubtitleState{}
	a.subtitle = o
}

func (a *App) onAPIKeyChanged() {
	key := strings.TrimSpace(a.trKeyLE.Text())
	t := &a.translation
	if key == t.apiKey {
		return
	}
	if err := SaveAPIKey(a.store.Dir, key); err != nil {
		a.setStatus("Không lưu được API key: " + err.Error())
		return
	}
	t.apiKey = key
	t.translator = nil
	a.setStatus("Đã lưu API key.")
}

func (a *App) onSubtitleFontChanged() {
	if a.trFontNE == nil {
		return
	}
	a.cfg.TranslateFontSize = int(a.trFontNE.Value())
	a.saveConfig()
	if s := a.subtitle; s != nil {
		s.FontSize = a.cfg.TranslateFontSize
		if win.IsWindowVisible(s.hwnd) {
			a.renderSubtitle()
		}
	}
}

func (a *App) onTranslateToggled() {
	t := &a.translation
	on := a.trOnCB.Checked()
	if on && a.subtitle == nil {
		a.trOnCB.SetChecked(false)
		a.setStatus("Không tạo được khung phụ đề, không bật dịch được.")
		return
	}
	if on {
		a.onAPIKeyChanged() // the key field may still have focus
		if t.apiKey == "" {
			a.trOnCB.SetChecked(false)
			a.showPage(true)
			a.bringToFront() // the hotkey may have been pressed inside a game
			a.setStatus("Nhập API key Anthropic trước khi bật dịch.")
			a.trKeyLE.SetFocus()
			return
		}
		if !a.hasTranslateRegion() {
			a.trOnCB.SetChecked(false)
			a.editTranslateRegion()
			if a.hasTranslateRegion() {
				a.trOnCB.SetChecked(true)
			}
			return
		}
	}
	t.on = on
	if !on {
		t.latest++ // a reply still in flight belongs to a session that was turned off
		a.hideSubtitle()
		a.setStatus("Đã tắt dịch.")
	}
	a.syncTranslateLoop()
	if on {
		a.setStatus("Đang dịch vùng đã khoanh.")
		a.showTranslateHint()
	}
}

// showTranslateHint confirms in the game that the region is being read.
func (a *App) showTranslateHint() {
	h := a.cfg.Hotkeys
	a.showSubtitle("", "Đang đọc vùng đã khoanh. "+h.TranslateRegion+" khoanh lại · "+h.TranslateToggle+" tắt", false)
}

// editTranslateRegion takes a screenshot of the game without the app or the
// subtitle on it, then lets the user drag over the dialogue.
func (a *App) editTranslateRegion() {
	t := &a.translation
	if a.cur == nil || t.picking {
		return
	}
	t.picking = true
	defer func() { t.picking = false }()
	wasOn := t.on
	t.on = false
	t.latest++            // a reply still in flight must not reappear over the dialog
	a.syncTranslateLoop() // or OCR would read and translate the region dialog
	a.hideSubtitle()
	a.mw.SetVisible(false)
	time.Sleep(200 * time.Millisecond) // let the windows leave the screen
	img, err := CaptureMonitor(a.currentMonitor())
	a.bringToFront() // a dialog owned by a minimized window stays hidden
	if err != nil {
		a.setStatus("Không chụp được màn hình: " + err.Error())
	} else {
		a.editRegionOn(img, "Khoanh vùng hội thoại",
			"Kéo chuột để khoanh SÁT khung hội thoại trong game.\nKhoanh càng hẹp càng đỡ tốn phí và càng ít đọc nhầm.",
			a.cfg.TranslateROI, false, func(roi ROI) {
				a.cfg.TranslateROI = roi
				a.saveConfig()
				a.refreshTranslateRegionLabel()
				a.setStatus("Đã lưu vùng hội thoại.")
			})
	}
	t.on = wasOn
	a.syncTranslateLoop()
	if wasOn {
		a.showTranslateHint()
	}
}

func (a *App) translateOcrFailed(msg string) {
	a.trOnCB.SetChecked(false)
	a.bringToFront()
	a.setStatus("Không đọc được chữ tiếng Anh: " + msg + " Thêm English trong Windows Settings → Time & language → Language & region.")
}

func (a *App) drawSubtitle(text, note string, dim bool) {
	s := a.subtitle
	s.Subtitle.Text, s.Subtitle.Note, s.Subtitle.Dim = text, note, dim
	a.renderSubtitle()
	s.Show(true)
}

func (a *App) renderSubtitle() {
	s := a.subtitle
	s.Subtitle.Region = a.translation.loop.region
	s.Subtitle.Monitor = a.currentMonitor().Px()
	s.Render()
}

func (a *App) eraseSubtitle() {
	if s := a.subtitle; s != nil {
		s.Subtitle.Text, s.Subtitle.Note = "", ""
		s.Show(false)
	}
}

// SubtitleState is what the translation box shows and where it anchors.
type SubtitleState struct {
	Text, Note      string
	Dim             bool
	Region, Monitor PxRect
}

// renderSubtitle draws wrapped text in a dark box placed against the region.
func (o *Overlay) renderSubtitle() {
	st := o.Subtitle
	screenDC := win.GetDC(0)
	defer win.ReleaseDC(0, screenDC)
	memDC := win.CreateCompatibleDC(screenDC)
	defer win.DeleteDC(memDC)

	mainFont := gdiFont(o.FontFamily, o.FontSize, false)
	defer win.DeleteObject(win.HGDIOBJ(mainFont))
	noteFont := gdiFont(o.FontFamily, max(o.FontSize*6/10, 12), false)
	defer win.DeleteObject(win.HGDIOBJ(noteFont))
	oldFont := win.SelectObject(memDC, win.HGDIOBJ(mainFont))
	defer win.SelectObject(memDC, oldFont)

	const format = win.DT_CENTER | win.DT_WORDBREAK | win.DT_NOPREFIX
	pad := int32(max(o.FontSize/2, 8))
	monW := st.Monitor.R - st.Monitor.L
	maxW := min(max(st.Region.R-st.Region.L, int32(o.FontSize*24)), monW)
	draw := func(text string, font win.HFONT, rc *win.RECT, extra uint32) {
		if t16, err := syscall.UTF16FromString(text); err == nil && len(t16) > 1 {
			win.SelectObject(memDC, win.HGDIOBJ(font))
			win.DrawTextEx(memDC, &t16[0], int32(len(t16)-1), rc, format|extra, nil)
		}
	}
	measure := func(text string, font win.HFONT) (int32, int32) {
		if text == "" {
			return 0, 0
		}
		rc := win.RECT{Right: maxW - 2*pad}
		draw(text, font, &rc, win.DT_CALCRECT)
		return rc.Right, rc.Bottom
	}
	tw, th := measure(st.Text, mainFont)
	nw, nh := measure(st.Note, noteFont)
	w, h := max(tw, nw)+2*pad, th+nh+pad

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

	// mask draws one block of text and returns its coverage.
	mask := func(text string, font win.HFONT, top int32) []uint8 {
		for i := range pix {
			pix[i] = 0
		}
		rc := win.RECT{Left: pad, Top: top, Right: w - pad, Bottom: h}
		draw(text, font, &rc, 0)
		win.GdiFlush()
		m := make([]uint8, w*h)
		for i := range m {
			m[i] = max(pix[i*4], pix[i*4+1], pix[i*4+2])
		}
		return m
	}
	textMask := mask(st.Text, mainFont, pad/2)
	noteMask := mask(st.Note, noteFont, pad/2+th)
	copy(pix, ComposeSubtitle(textMask, noteMask, st.Dim))

	o.X, o.Y = PlaceSubtitle(st.Region, st.Monitor, w, h)
	dst := win.POINT{X: o.X, Y: o.Y}
	size := win.SIZE{CX: w, CY: h}
	src := win.POINT{}
	blend := win.BLENDFUNCTION{BlendOp: 0, SourceConstantAlpha: 255, AlphaFormat: win.AC_SRC_ALPHA}
	procUpdateLayeredWindow.Call(uintptr(o.hwnd), uintptr(screenDC),
		uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&size)),
		uintptr(memDC), uintptr(unsafe.Pointer(&src)), 0,
		uintptr(unsafe.Pointer(&blend)), 2 /* ULW_ALPHA */)
}

func (a *App) sync(f func())       { a.mw.Synchronize(f) }
func (a *App) simText(s string)    { a.simLbl.SetText(s) }
func (a *App) ocrText(s string)    { a.ocrLbl.SetText(s) }
func (a *App) inGame() bool        { return isOtherFullscreen(win.GetForegroundWindow()) }
func (a *App) trSrcText(s string)  { a.trSrcLbl.SetText(s) }
func (a *App) trDstText(s string)  { a.trDstLbl.SetText(s) }
func (a *App) trCostText(s string) { a.trCostLbl.SetText(s) }

func (a *App) refreshTranslateRegionLabel() {
	if !a.hasTranslateRegion() {
		a.trRegionLbl.SetText("Chưa khoanh")
		return
	}
	r := a.cfg.TranslateROI.ToPixels(a.currentMonitor().Px())
	a.trRegionLbl.SetText(fmt.Sprintf("%d×%d px", r.R-r.L, r.B-r.T))
}
