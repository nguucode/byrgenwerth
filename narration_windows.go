//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// narration reads the Vietnamese translation aloud with Piper, on a keyboard
// key or a gamepad button combination. It belongs to the UI thread.
type narration struct {
	sound   Sound
	pads    *Gamepads
	cancel  context.CancelFunc
	req     int    // newest reading asked for; older results are dropped
	text    string // what is playing (or being prepared)
	wav     string // file being played, deleted when replaced
	getting bool   // the voice is downloading

	voiceCB   *walk.ComboBox
	rateNE    *walk.NumberEdit
	volNE     *walk.NumberEdit
	keyLE     *walk.LineEdit
	padLE     *walk.LineEdit
	getPB     *walk.PushButton
	statusLbl *walk.Label
}

// narrationSection is the part of the Translate page about narration.
func (a *App) narrationSection() []Widget {
	n := &a.narr
	names := make([]string, len(Voices))
	cur := 0
	for i, v := range Voices {
		names[i] = VoiceName(v)
		if v == a.cfg.NarrateVoice {
			cur = i
		}
	}
	return []Widget{
		title("Giọng đọc tiếng Việt"),
		form(
			Label{Text: "Giọng"}, row(
				ComboBox{AssignTo: &n.voiceCB, Model: names, CurrentIndex: cur, MinSize: inSize, MaxSize: inSize, OnCurrentIndexChanged: a.onNarrateVoiceChanged},
				PushButton{AssignTo: &n.getPB, Text: "Tải", OnClicked: a.downloadVoice, ToolTipText: "Tải Piper và giọng đã chọn (một lần)."},
			),
			Label{Text: "Tốc độ"}, row(
				NumberEdit{AssignTo: &n.rateNE, MinValue: -50, MaxValue: 100, Decimals: 0, Value: float64(a.cfg.NarrateRate), SpinButtonsVisible: true, MinSize: inSize, MaxSize: inSize, OnValueChanged: a.onNarrateSettingsChanged,
					ToolTipText: "0 = bình thường, +50 = nhanh hơn một nửa, -50 = chậm gấp đôi."},
				Label{Text: "%"},
			),
			Label{Text: "Âm lượng"}, row(
				NumberEdit{AssignTo: &n.volNE, MinValue: 0, MaxValue: 100, Decimals: 0, Value: float64(a.cfg.NarrateVolume), SpinButtonsVisible: true, MinSize: inSize, MaxSize: inSize, OnValueChanged: a.onNarrateSettingsChanged},
				Label{Text: "%"},
			),
		),
		row(
			PushButton{Text: "Nghe thử", OnClicked: func() { a.speak("Xin chào, đây là giọng thuyết minh của Byrgenwerth.") }},
			PushButton{Text: "Đọc dịch", OnClicked: a.onNarrate, ToolTipText: "Đọc bản dịch đang hiện"},
		),
		Label{AssignTo: &n.statusLbl, Text: "", TextColor: hintColor, EllipsisMode: EllipsisEnd},
		sep(),
		title("Bấm để đọc bản dịch"),
		form(
			Label{Text: "Bàn phím"}, row(
				LineEdit{AssignTo: &n.keyLE, ReadOnly: true, MinSize: inSize, MaxSize: inSize},
				PushButton{Text: "Gán...", OnClicked: func() { a.bindNarrate(false) }},
			),
			Label{Text: "Tay cầm"}, row(
				LineEdit{AssignTo: &n.padLE, ReadOnly: true, MinSize: inSize, MaxSize: inSize},
				PushButton{Text: "Gán...", OnClicked: func() { a.bindNarrate(true) }},
			),
		),
		hint("Bấm lần nữa để dừng. Tay Xbox hoặc giả lập Xbox (Steam Input, DS4Windows); game cũng nhận nút này, nên chọn tổ hợp game không dùng, ví dụ LB+RB+Y."),
		hint("Giọng đọc chạy ngay trên máy (Piper): không cần mạng, không gửi chữ đi đâu."),
	}
}

func (a *App) initNarration() {
	n := &a.narr
	n.sound.Alias = "dctts"
	n.pads = StartGamepads(func() { a.mw.Synchronize(a.onNarrate) })
	mask, _ := ParsePadCombo(a.cfg.NarratePad)
	n.pads.SetCombo(mask)
	a.refreshNarrateBindings()
	a.refreshVoiceStatus()
}

func (a *App) stopNarration() {
	n := &a.narr
	if n.cancel != nil {
		n.cancel()
	}
	n.sound.Close()
	if n.wav != "" {
		os.Remove(n.wav)
	}
	n.pads.Stop()
}

func (a *App) refreshNarrateBindings() {
	n := &a.narr
	k := a.cfg.Hotkeys.Narrate
	if k == "" {
		k = "Chưa gán"
	}
	n.keyLE.SetText(k)
	p := a.cfg.NarratePad
	if p == "" {
		p = "Chưa gán"
	}
	n.padLE.SetText(p)
}

func (a *App) narrateStatus(s string) {
	if a.narr.statusLbl != nil {
		a.narr.statusLbl.SetText(s)
	}
}

// refreshVoiceStatus says whether the chosen voice still has to be downloaded.
func (a *App) refreshVoiceStatus() {
	n := &a.narr
	if n.getting {
		return
	}
	dir := PiperDir(a.store.Dir)
	if PiperReady(dir, a.cfg.NarrateVoice) {
		n.getPB.SetEnabled(false)
		a.narrateStatus("Giọng " + VoiceName(a.cfg.NarrateVoice) + " đã sẵn sàng.")
		return
	}
	n.getPB.SetEnabled(true)
	a.narrateStatus(fmt.Sprintf("Chưa tải giọng này: bấm \"Tải\" (khoảng %d MB).", max(PiperDownloadSize(dir, a.cfg.NarrateVoice)>>20, 1)))
}

func (a *App) onNarrateVoiceChanged() {
	n := &a.narr
	if n.voiceCB == nil {
		return
	}
	if i := n.voiceCB.CurrentIndex(); i >= 0 && i < len(Voices) {
		a.cfg.NarrateVoice = Voices[i]
		a.saveConfig()
		a.refreshVoiceStatus()
	}
}

func (a *App) onNarrateSettingsChanged() {
	n := &a.narr
	if n.rateNE == nil || n.volNE == nil {
		return
	}
	a.cfg.NarrateRate = int(n.rateNE.Value())
	a.cfg.NarrateVolume = int(n.volNE.Value())
	a.saveConfig()
}

// downloadVoice fetches Piper and the chosen voice in the background.
func (a *App) downloadVoice() {
	n := &a.narr
	if n.getting {
		return
	}
	n.getting = true
	n.getPB.SetEnabled(false)
	dir, voice := PiperDir(a.store.Dir), a.cfg.NarrateVoice
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			select {
			case <-a.quit:
				cancel()
			case <-ctx.Done():
			}
		}()
		err := EnsurePiper(ctx, dir, voice, func(s string) {
			a.mw.Synchronize(func() { a.narrateStatus(s) })
		})
		a.mw.Synchronize(func() {
			n.getting = false
			if err != nil {
				a.narrateStatus("Tải giọng không được: " + err.Error())
				n.getPB.SetEnabled(true)
				return
			}
			a.refreshVoiceStatus()
			a.setStatus("Đã tải xong giọng " + VoiceName(voice) + ".")
		})
	}()
}

// onNarrate reads the translation on screen, or the last one; pressed again
// while that text is playing, it stops.
func (a *App) onNarrate() {
	text := a.translation.shown
	if text == "" {
		text = a.translation.last
	}
	n := &a.narr
	if text != "" && text == n.text && (n.sound.Playing() || n.cancel != nil) {
		a.stopSpeaking()
		return
	}
	if text == "" {
		a.narrateStatus("Chưa có bản dịch nào để đọc. Bật dịch ở trên trước.")
		a.setStatus("Chưa có bản dịch nào để đọc.")
		return
	}
	a.speak(text)
}

func (a *App) stopSpeaking() {
	n := &a.narr
	n.req++
	if n.cancel != nil {
		n.cancel()
		n.cancel = nil
	}
	n.sound.Close()
	n.text = ""
	a.narrateStatus("Đã dừng đọc.")
}

// speak turns text into speech with Piper in the background and plays it.
func (a *App) speak(text string) {
	n := &a.narr
	dir, voice := PiperDir(a.store.Dir), a.cfg.NarrateVoice
	if !PiperReady(dir, voice) {
		msg := "Chưa tải giọng đọc: mở tab Translate, bấm \"Tải\"."
		a.narrateStatus(msg)
		a.setStatus(msg)
		return
	}
	a.stopSpeaking()
	n.req++
	req := n.req
	n.text = text
	ctx, cancel := context.WithCancel(context.Background())
	n.cancel = cancel
	out := filepath.Join(os.TempDir(), fmt.Sprintf("byrgenwerth-tts-%d-%d.wav", os.Getpid(), req))
	rate, vol := a.cfg.NarrateRate, a.cfg.NarrateVolume
	a.narrateStatus("Đang chuẩn bị giọng đọc…")
	started := time.Now()
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		err := PiperSpeak(ctx, dir, voice, text, rate, vol, out)
		a.mw.Synchronize(func() {
			if req != n.req { // stopped or replaced meanwhile
				os.Remove(out)
				return
			}
			cancel()
			n.cancel = nil
			if err != nil {
				n.text = ""
				a.narrateStatus("Không đọc được: " + err.Error())
				return
			}
			old := n.wav
			n.wav = out
			if err := n.sound.Load(out, 100); err != nil {
				a.narrateStatus("Không phát được: " + err.Error())
				return
			}
			if old != "" && old != out {
				os.Remove(old)
			}
			if err := n.sound.Play(); err != nil {
				a.narrateStatus("Không phát được: " + err.Error())
				return
			}
			a.narrateStatus(fmt.Sprintf("Đang đọc (chuẩn bị mất %.1f giây).", time.Since(started).Seconds()))
		})
	}()
}

// ---------- binding a key or a gamepad combination ----------

var procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")

func keyDown(vk uint32) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return r&0x8000 != 0
}

// pressedHotkey returns the key just pressed with its modifiers, read straight
// from the keyboard so Alt combinations and F10 work too.
func pressedHotkey(prev map[uint32]bool) string {
	found := ""
	for vk := uint32(0x08); vk <= 0xFE; vk++ {
		down := keyDown(vk)
		was := prev[vk]
		prev[vk] = down
		if !down || was || found != "" {
			continue
		}
		switch vk {
		case 0x10, 0x11, 0x12, 0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5, 0x5B, 0x5C, 0x1B, 0x01, 0x02, 0x04: // modifiers, Esc, mouse
			continue
		}
		found = HotkeyText(keyDown(0x11), keyDown(0x12), keyDown(0x10), vk)
	}
	return found
}

// bindNarrate opens a small dialog that waits for a key (pad=false) or a
// gamepad combination (pad=true) and saves it.
func (a *App) bindNarrate(pad bool) {
	var dlg *walk.Dialog
	var le *walk.LineEdit
	var okPB, cancelPB *walk.PushButton
	result := ""
	instr := "Bấm phím muốn dùng, có thể kèm Ctrl, Alt, Shift (ví dụ Ctrl+Alt+V)."
	what := "Phím"
	if pad {
		instr = "Giữ cùng lúc các nút muốn dùng trên tay cầm rồi thả ra (ví dụ LB+RB+Y)."
		what = "Nút tay cầm"
	}
	// Registered hotkeys would swallow the key being bound.
	a.hk.Stop()
	defer a.startHotkeys()

	stop := make(chan struct{})
	err := Dialog{
		AssignTo:      &dlg,
		Title:         "Gán " + what + " để đọc bản dịch",
		DefaultButton: &okPB,
		CancelButton:  &cancelPB,
		MinSize:       Size{Width: 420, Height: 150},
		Layout:        VBox{Alignment: AlignHNearVNear},
		Children: []Widget{
			Label{Text: instr},
			row(
				Label{Text: what},
				LineEdit{AssignTo: &le, ReadOnly: true, Text: "Đang chờ…", MinSize: inSize, MaxSize: inSize},
			),
			row(
				PushButton{AssignTo: &okPB, Text: "Lưu", OnClicked: func() { dlg.Accept() }},
				PushButton{Text: "Bỏ gán", OnClicked: func() { result = ""; dlg.Accept() }},
				PushButton{AssignTo: &cancelPB, Text: "Hủy", OnClicked: func() { dlg.Cancel() }},
			),
		},
	}.Create(a.mw)
	if err != nil {
		a.setStatus("Lỗi mở hộp thoại: " + err.Error())
		return
	}
	set := func(s string) {
		if s == "" {
			return
		}
		result = s
		le.SetText(s)
	}
	if pad {
		a.narr.pads.Capture(func(mask uint32) {
			name := PadComboName(mask)
			dlg.Synchronize(func() { set(name) })
		})
		defer a.narr.pads.Capture(nil)
	} else {
		go func() {
			prev := map[uint32]bool{}
			pressedHotkey(prev) // keys already held do not count
			for {
				select {
				case <-stop:
					return
				case <-time.After(20 * time.Millisecond):
				}
				if k := pressedHotkey(prev); k != "" {
					dlg.Synchronize(func() { set(k) })
				}
			}
		}()
	}
	res := dlg.Run()
	close(stop)
	if res != walk.DlgCmdOK {
		return
	}
	if pad {
		a.cfg.NarratePad = result
		mask, _ := ParsePadCombo(result)
		a.narr.pads.SetCombo(mask)
	} else {
		a.cfg.Hotkeys.Narrate = result
	}
	a.saveConfig()
	a.refreshNarrateBindings()
	if result == "" {
		a.setStatus("Đã bỏ gán " + what + ".")
	} else {
		a.setStatus("Đã gán " + result + " để đọc bản dịch.")
	}
}

// startHotkeys (re)registers every global hotkey from the config.
func (a *App) startHotkeys() {
	h := a.cfg.Hotkeys
	keys := map[HotkeyAction]string{ActAdd: h.Add, ActSub: h.Sub, ActCapture: h.Capture, ActToggle: h.Toggle, ActWindow: h.Window,
		ActTranslate: h.TranslateToggle, ActTranslateRegion: h.TranslateRegion}
	if h.Narrate != "" {
		keys[ActNarrate] = h.Narrate
	}
	a.hk, a.hkErrors = StartHotkeys(keys, func(act HotkeyAction) {
		a.mw.Synchronize(func() { a.onHotkey(act) })
	})
	if len(a.hkErrors) > 0 && a.statusLbl != nil {
		a.setStatus("Cảnh báo phím tắt: " + fmt.Sprint(a.hkErrors))
	}
}
