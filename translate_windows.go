//go:build windows

package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

const translateInterval = 250 * time.Millisecond

// translationState belongs to the UI thread, the only writer. translateLoop
// reads loop; writes to it take a.mu.
type translationState struct {
	on         bool
	apiKey     string
	translator *Translator
	subtitle   *Overlay
	latest     int  // the newest line sent for translation; older replies are dropped
	picking    bool // the region dialog is open
	ctx        context.Context
	cancel     context.CancelFunc
	loop       translateLoopState
}

type translateLoopState struct {
	on     bool
	region PxRect
	ignore []ROI // the app's own windows, normalized to the region
	gen    int
}

func (a *App) translatePage() Widget {
	hk := a.cfg.Hotkeys
	return Composite{
		AssignTo: &a.transPage,
		Visible:  false,
		Border:   true,
		Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 6},
		Children: []Widget{
			title("Dịch hội thoại (Anh → Việt)"),
			CheckBox{AssignTo: &a.trOnCB, Text: "Bật dịch", OnCheckedChanged: a.onTranslateToggled},
			form(
				Label{Text: "Vùng hội thoại"}, row(
					Label{AssignTo: &a.trRegionLbl, Text: "Chưa khoanh", StretchFactor: 3},
					PushButton{Text: "Khoanh vùng...", OnClicked: a.editTranslateRegion},
				),
				Label{Text: "API key"}, LineEdit{AssignTo: &a.trKeyLE, PasswordMode: true, OnEditingFinished: a.onAPIKeyChanged},
				Label{Text: "Cỡ phụ đề"}, row(
					NumberEdit{AssignTo: &a.trFontNE, MinValue: 10, MaxValue: 120, Decimals: 0, Value: float64(a.cfg.TranslateFontSize), SpinButtonsVisible: true, MaxSize: Size{Width: 80}, OnValueChanged: a.onSubtitleFontChanged},
					Label{Text: "px"},
					HSpacer{},
				),
			),
			hint("Dùng API key Anthropic của bạn (console.anthropic.com), trả phí theo lượng dùng. Chữ trong vùng khoanh được gửi tới Anthropic để dịch. Key được mã hóa theo tài khoản Windows, không rời khỏi máy trừ khi gọi dịch."),
			hint("Game cần chạy Windowed hoặc Borderless để phụ đề hiện được phía trên. Tên game lấy từ tab Death count, giúp dịch đúng tên riêng."),
			hint(fmt.Sprintf("Phím tắt:  %s bật/tắt dịch  ·  %s khoanh lại vùng", hk.TranslateToggle, hk.TranslateRegion)),
			sep(),
			title("Câu vừa đọc"),
			TextLabel{AssignTo: &a.trSrcLbl, Text: "--", MinSize: Size{Width: 100}},
			title("Bản dịch"),
			TextLabel{AssignTo: &a.trDstLbl, Text: "--", MinSize: Size{Width: 100}},
			Label{AssignTo: &a.trCostLbl, Text: "", TextColor: hintColor},
			VSpacer{},
		},
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
	t.subtitle = o
}

func (a *App) hasTranslateRegion() bool { return a.cfg.TranslateROI.W > 0 }

func (a *App) refreshTranslateRegionLabel() {
	if !a.hasTranslateRegion() {
		a.trRegionLbl.SetText("Chưa khoanh")
		return
	}
	r := a.cfg.TranslateROI.ToPixels(a.currentMonitor().Px())
	a.trRegionLbl.SetText(fmt.Sprintf("%d×%d px", r.R-r.L, r.B-r.T))
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
	if s := a.translation.subtitle; s != nil {
		s.FontSize = a.cfg.TranslateFontSize
		if win.IsWindowVisible(s.hwnd) {
			a.renderSubtitle()
		}
	}
}

func (a *App) onTranslateToggled() {
	t := &a.translation
	on := a.trOnCB.Checked()
	if on && t.subtitle == nil {
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

// syncTranslateLoop hands the current settings to the OCR loop and restarts its
// line tracking.
func (a *App) syncTranslateLoop() {
	t := &a.translation
	a.mu.Lock()
	t.loop.on = t.on
	t.loop.region = a.cfg.TranslateROI.ToPixels(a.currentMonitor().Px())
	t.loop.gen++
	a.mu.Unlock()
	a.refreshTranslateIgnore()
}

// refreshTranslateIgnore keeps OCR from reading, and paying to translate, the
// subtitle or any other window of the app that sits inside the region.
func (a *App) refreshTranslateIgnore() {
	if !a.hasTranslateRegion() {
		return
	}
	var ignore []ROI
	for _, own := range a.ownRects() {
		ignore = append(ignore, own.RelativeTo(a.cfg.TranslateROI))
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.translation.loop.ignore = ignore
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

// translateLoop reads the region and reports settled lines to the UI thread.
func (a *App) translateLoop() {
	defer a.wg.Done()
	// WinRT OCR objects live on this thread for its whole life.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var ocr *OcrEngine
	defer func() { ocr.Close() }()

	var stab Stabilizer
	gen := 0
	for {
		select {
		case <-a.quit:
			return
		case <-time.After(translateInterval):
		}
		a.mu.Lock()
		st := a.translation.loop
		a.mu.Unlock()
		if !st.on {
			continue
		}
		if st.gen != gen {
			gen, stab = st.gen, Stabilizer{}
		}
		if ocr == nil {
			var err error
			if ocr, err = NewOcrEngine("en-US"); err != nil {
				ocr = nil
				msg := err.Error()
				a.mw.Synchronize(func() { a.onTranslateOcrFailed(msg) })
				continue
			}
		}
		img, err := CaptureRegion(st.region)
		if err != nil {
			continue
		}
		lines, err := ocr.Recognize(img)
		if err != nil {
			continue
		}
		var texts []string
		for _, ln := range DropLinesInside(lines, st.ignore) {
			texts = append(texts, ln.Text)
		}
		line, ev := stab.Feed(strings.Join(texts, "\n"), time.Now())
		if ev != StableNone {
			a.mw.Synchronize(func() { a.onStableLine(st.gen, line, ev) })
		}
	}
}

func (a *App) onTranslateOcrFailed(msg string) {
	a.trOnCB.SetChecked(false)
	a.bringToFront()
	a.setStatus("Không đọc được chữ tiếng Anh: " + msg + " Thêm English trong Windows Settings → Time & language → Language & region.")
}

func (a *App) onStableLine(gen int, line string, ev StableEvent) {
	t := &a.translation
	if !t.on || gen != t.loop.gen {
		return
	}
	if ev == StableClear {
		t.latest++ // a reply still in flight belongs to text that is gone
		a.hideSubtitle()
		return
	}
	a.translateLine(line)
}

func (a *App) translateLine(line string) {
	t := &a.translation
	t.latest++
	request := t.latest
	a.trSrcLbl.SetText(line)
	if len([]rune(line)) > MaxTranslateChars {
		a.showSubtitle("", "Vùng khoanh có quá nhiều chữ. Bấm "+a.cfg.Hotkeys.TranslateRegion+" để khoanh hẹp lại.", false)
		return
	}
	game := ""
	if a.cur != nil && a.cur.Name != newProfileName {
		game = a.cur.Name
	}
	if t.translator == nil || t.translator.game != game {
		t.translator = NewTranslator(game, NewAnthropicSend(t.apiKey))
	}
	tr := t.translator

	// Keep the previous line, dimmed, so the box does not jump.
	if s := t.subtitle.Subtitle; s.Text != "" {
		a.showSubtitle(s.Text, "", true)
	} else {
		a.showSubtitle("", "Đang dịch…", false)
	}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		res, err := tr.Translate(t.ctx, line)
		a.mw.Synchronize(func() { a.onTranslated(request, line, tr, res, err) })
	}()
}

func (a *App) onTranslated(request int, line string, tr *Translator, res TranslateResult, err error) {
	if request != a.translation.latest { // newer text is already on its way
		return
	}
	if err != nil {
		msg := DescribeTranslateError(err)
		a.trDstLbl.SetText(msg)
		// Still show the English: an unreadable second is worse than an untranslated one.
		a.showSubtitle(line, msg, false)
		return
	}
	a.trDstLbl.SetText(res.Text)
	a.showSubtitle(res.Text, "", false)
	took := "lấy từ cache"
	if !res.Cached {
		took = fmt.Sprintf("%d ms", res.Took.Milliseconds())
	}
	a.trCostLbl.SetText(fmt.Sprintf("Câu vừa rồi: %s  ·  Chi phí ước tính phiên này: $%.4f", took, tr.TotalUSD()))
}

func (a *App) showSubtitle(text, note string, dim bool) {
	s := a.translation.subtitle
	s.Subtitle.Text, s.Subtitle.Note, s.Subtitle.Dim = text, note, dim
	a.renderSubtitle()
	s.Show(true)
	a.refreshTranslateIgnore()
}

func (a *App) renderSubtitle() {
	s := a.translation.subtitle
	s.Subtitle.Region = a.translation.loop.region
	s.Subtitle.Monitor = a.currentMonitor().Px()
	s.Render()
}

func (a *App) hideSubtitle() {
	if s := a.translation.subtitle; s != nil {
		s.Subtitle.Text, s.Subtitle.Note = "", ""
		s.Show(false)
	}
	a.refreshTranslateIgnore()
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
