package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
#include <stdlib.h>
#include "platform_darwin.h"
*/
import "C"

//go:embed webui/app.html
var appPage string

const (
	mainWidth, mainHeight     = 520, 860
	pickerWidth, pickerHeight = 1100, 740
)

// App is the macOS shell. The UI is a local web page in a native window, driven
// through a token-guarded API; the overlays are the same pages OBS shows.
type App struct {
	Engine

	uiMu   sync.Mutex
	server *OverlayServer
	token  string

	profiles []*Profile
	tmplImgs []image.Image
	ocrLangs []OcrLang
	fonts    []string

	status, sim, hp, ocr string
	trSrc, trDst, trCost string
	hotkeyErrs           []string

	sound     *exec.Cmd
	lastFront bool

	overlay, subtitle C.int
	overlayEdit       bool
	subtitleState     subtitleView
	subtitleVersion   int64
	uninstalling      bool
	gotoTranslate     bool // the page should open the Translate tab once
	pick              *pickState
}

type pickState struct {
	What, Title, Instr string
	Init               ROI
	shot               *image.RGBA
	wasOn              bool
}

type subtitleView struct {
	Text    string `json:"text"`
	Note    string `json:"note"`
	Dim     bool   `json:"dim"`
	Size    int    `json:"size"`
	Anchor  string `json:"anchor"`
	Version int64  `json:"v"`
}

var theApp *App

func dataDirMac() string {
	base, err := os.UserConfigDir() // ~/Library/Application Support
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "Byrgenwerth")
}

func main() {
	dir := dataDirMac()
	if err := CheckWritable(dir); err != nil {
		fmt.Fprintln(os.Stderr, "Byrgenwerth:", err)
		os.Exit(1)
	}
	a := newApp(dir)
	theApp = a

	srv, err := StartOverlayServer(a.cfg.ServerPort)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Byrgenwerth:", err)
		os.Exit(1)
	}
	a.server = srv
	a.registerRoutes(srv)
	a.loadProfiles()

	url := fmt.Sprintf("%sapp?t=%s", srv.URL(), a.token)
	go a.afterLaunch()
	cstr(url, func(u *C.char) { C.bw_run(u, mainWidth, mainHeight) })
}

// newApp builds the app without touching any native window, so tests can drive it.
func newApp(dir string) *App {
	tok := make([]byte, 24)
	rand.Read(tok)
	a := &App{Engine: Engine{store: &Store{Dir: dir}, quit: make(chan struct{}), ocrAvailable: true},
		token: hex.EncodeToString(tok), overlay: -1, subtitle: -1}
	a.ui = a
	a.cfg = a.store.LoadConfig()
	a.presets = LoadPresets()
	return a
}

// afterLaunch runs once AppKit is up: the native calls wait on the main thread.
func (a *App) afterLaunch() {
	mons := ListMonitors()
	fonts := fontFamilies()
	base := a.server.URL()
	var overlay, subtitle C.int
	cstr(base, func(u *C.char) { overlay = C.bw_overlay_create(u) })
	cstr(base+"subtitle", func(u *C.char) { subtitle = C.bw_overlay_create(u) })

	// May ask for the Keychain password after an update: not while holding the lock.
	apiKey := loadAPIKeyMac()

	a.sync(func() {
		a.monitors, a.fonts = mons, fonts
		if a.cfg.Monitor < 0 || a.cfg.Monitor >= len(a.monitors) {
			a.cfg.Monitor = 0
		}
		a.overlay, a.subtitle = overlay, subtitle
		a.translation.ctx, a.translation.cancel = context.WithCancel(context.Background())
		a.translation.apiKey = apiKey
		h := a.cfg.Hotkeys
		for act, spec := range map[HotkeyAction]string{ActAdd: h.Add, ActSub: h.Sub, ActCapture: h.Capture, ActToggle: h.Toggle, ActWindow: h.Window, ActTranslate: h.TranslateToggle, ActTranslateRegion: h.TranslateRegion} {
			if err := registerHotkey(act, spec); err != nil {
				a.hotkeyErrs = append(a.hotkeyErrs, err.Error())
			}
		}
		a.placeOverlays()
		a.applyTop()
		a.refreshCount()
		a.updateLoop()
		switch {
		case C.bw_screen_access(1) == 0:
			a.status = "Cần quyền Screen Recording: bật cho Byrgenwerth trong System Settings → Privacy & Security, rồi mở lại app."
		case len(a.hotkeyErrs) > 0:
			a.status = "Cảnh báo phím tắt: " + strings.Join(a.hotkeyErrs, "; ")
		default:
			a.status = "Sẵn sàng. Khi chết mà app chưa tự nhận ra, bấm " + h.Add + "."
		}
	})
	a.wg.Add(3)
	go a.captureLoop()
	go a.translateLoop()
	go a.tickLoop()
}

// tickLoop gets the main window out of the way when a full-screen game comes to
// the front, once per switch, so F7 can still bring it back.
func (a *App) tickLoop() {
	defer a.wg.Done()
	for {
		select {
		case <-a.quit:
			return
		case <-time.After(time.Second):
		}
		full := C.bw_front_fullscreen(0) != 0
		if full && !a.lastFront && C.bw_main_visible() != 0 {
			C.bw_main_hide()
		}
		a.lastFront = full
	}
}

func appWillQuit() {
	a := theApp
	if a == nil {
		return
	}
	close(a.quit)
	if a.uninstalling {
		return // saving now would bring the data folder back
	}
	// The main thread must not wait on a lock whose holder may be waiting on the
	// main thread, so give up after a moment: only a last config save is at stake.
	for i := 0; i < 20; i++ {
		if a.uiMu.TryLock() {
			a.store.SaveConfig(a.cfg)
			a.uiMu.Unlock()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if a.sound != nil && a.sound.Process != nil {
		a.sound.Process.Kill()
	}
}

func hotkeyPressed(act HotkeyAction) {
	a := theApp
	a.sync(func() {
		switch act {
		case ActWindow:
			if C.bw_main_visible() != 0 {
				C.bw_main_hide()
			} else {
				C.bw_main_show()
			}
		case ActTranslate:
			if a.pick == nil {
				a.setTranslate(!a.translation.on)
			}
		case ActTranslateRegion:
			a.startPick("translate")
		case ActAdd:
			a.manualAdd()
		case ActSub:
			a.addDeath(-1, "hotkey")
		case ActCapture:
			a.captureTemplate(true)
		case ActToggle:
			if a.cur != nil {
				a.cur.AutoEnabled = !a.cur.AutoEnabled
				a.store.SaveProfile(a.cur)
				a.updateLoop()
			}
		}
	})
}

// ---------- EngineUI ----------

func (a *App) sync(f func()) {
	a.uiMu.Lock()
	defer a.uiMu.Unlock()
	f()
}

func (a *App) setStatus(s string)              { a.status = s }
func (a *App) simText(s string)                { a.sim = s }
func (a *App) hpText(s string)                 { a.hp = s }
func (a *App) ocrText(s string)                { a.ocr = s }
func (a *App) setOcrLanguages(langs []OcrLang) { a.ocrLangs = langs }
func (a *App) ownRects() []ROI                 { return nil } // capture already leaves the app out
func (a *App) inGame() bool                    { return C.bw_front_fullscreen(1) != 0 }
func (a *App) refreshROILabel()                {}
func (a *App) trSrcText(s string)              { a.trSrc = s }
func (a *App) trDstText(s string)              { a.trDst = s }
func (a *App) trCostText(s string)             { a.trCost = s }

func (a *App) playSound() {
	if a.cfg.SoundFile == "" {
		return
	}
	if a.sound != nil && a.sound.Process != nil {
		a.sound.Process.Kill()
	}
	a.sound = exec.Command("/usr/bin/afplay", "-v", strconv.FormatFloat(float64(a.cfg.SoundVolume)/100, 'f', 2, 64), a.cfg.SoundFile)
	if err := a.sound.Start(); err != nil {
		a.status = "Lỗi phát âm thanh: " + err.Error()
		return
	}
	go a.sound.Wait()
}

func (a *App) refreshCount() {
	if a.cur == nil || a.server == nil {
		return
	}
	if a.cfg.ObsEnabled {
		if err := a.store.WriteObs(a.cfg.ObsFormat, a.cur.Name, a.cur.Deaths); err != nil {
			a.status = "Không ghi được file OBS: " + err.Error()
		}
	}
	a.server.Set(StateFromConfig(a.cfg, FormatObs(a.cfg.ObsFormat, a.cur.Name, a.cur.Deaths)))
}

func (a *App) reloadTemplates() {
	a.tmplImgs, a.tmplFeats = nil, nil
	if a.cur != nil {
		for _, name := range a.cur.Templates {
			img, err := a.store.LoadTemplateImage(a.cur, name)
			if err != nil {
				continue
			}
			a.tmplImgs = append(a.tmplImgs, img)
			a.tmplFeats = append(a.tmplFeats, ExtractFeature(img, a.cur.ROI))
		}
	}
	a.updateLoop()
}

func (a *App) translateOcrFailed(msg string) {
	a.setTranslate(false)
	a.status = "Không đọc được chữ tiếng Anh: " + msg
	C.bw_main_show()
}

func (a *App) drawSubtitle(text, note string, dim bool) {
	t := &a.translation
	mon := a.currentMonitor().Rect
	size := a.cfg.TranslateFontSize
	w := min(max(t.loop.region.R-t.loop.region.L, int32(size*24)), mon.R-mon.L)
	h := int32(size * 7)
	x, y := PlaceSubtitle(t.loop.region, mon, w, h)
	anchor := "top"
	if y+h <= t.loop.region.T { // sits above the region, so hug its lower edge
		anchor = "bottom"
	}
	a.subtitleVersion++
	a.subtitleState = subtitleView{Text: text, Note: note, Dim: dim, Size: size, Anchor: anchor, Version: a.subtitleVersion}
	if a.subtitle >= 0 {
		C.bw_overlay_frame(a.subtitle, C.int(x), C.int(y), C.int(w), C.int(h))
		C.bw_overlay_show(a.subtitle, 1)
	}
}

func (a *App) eraseSubtitle() {
	a.subtitleVersion++
	a.subtitleState = subtitleView{Version: a.subtitleVersion}
	if a.subtitle >= 0 {
		C.bw_overlay_show(a.subtitle, 0)
	}
}

// ---------- behaviour ----------

func (a *App) loadProfiles() {
	var err error
	a.profiles, err = a.store.LoadProfiles()
	if err != nil {
		a.status = "Lỗi đọc dữ liệu: " + err.Error()
	}
	if len(a.profiles) == 0 {
		if p, err := a.store.CreateProfile(newProfileName); err == nil {
			a.profiles = []*Profile{p}
		}
	}
	for _, p := range a.profiles {
		if p.ID == a.cfg.CurrentProfile {
			a.loadProfile(p)
			return
		}
	}
	if len(a.profiles) > 0 {
		a.loadProfile(a.profiles[0])
	}
}

func (a *App) loadProfile(p *Profile) {
	if i := a.presetIndex(p.Preset); i >= 0 {
		if k := UpgradeKeywords(p.Keywords, a.presets[i].Keywords); len(k) != len(p.Keywords) {
			p.Keywords = k
			a.store.SaveProfile(p)
		}
	}
	a.cur = p
	a.det = NewDetector(p.Threshold, time.Duration(p.CooldownSec*float64(time.Second)))
	a.hpDet = nil
	if p.HPColor.Valid() {
		a.hpDet = NewHPDeath(p.HPFull)
	}
	a.cfg.CurrentProfile = p.ID
	a.store.SaveConfig(a.cfg)
	a.ocrProven = false
	a.refreshCount()
	a.reloadTemplates()
}

func (a *App) manualAdd() {
	if a.cur == nil {
		return
	}
	now := time.Now()
	if a.det.AutoCountedRecently(now) {
		a.status = "App vừa tự đếm lần chết này rồi, bỏ qua " + a.cfg.Hotkeys.Add + "."
		return
	}
	img, err := CaptureMonitorScaled(a.currentMonitor(), CaptureW, CaptureH)
	a.addDeath(1, "hotkey")
	a.store.AppendMissLog(a.cur.Name, a.recentOcr, now)
	a.det.MarkManual(now)
	if err != nil {
		a.status = "Đã +1, nhưng không chụp được màn hình: " + err.Error()
		return
	}
	sim, _ := BestSimilarity(ExtractFeature(img, a.cur.ROI), a.tmplFeats)
	if len(a.tmplFeats) > 0 && sim >= a.cur.Threshold {
		a.status = fmt.Sprintf("Đã +1. Màn hình này đã khớp mẫu (%d%%), không lưu thêm.", int(sim*100))
		return
	}
	a.saveTemplate(img, "Đã +1 và lưu ảnh mẫu mới")
}

func (a *App) captureTemplate(fromHotkey bool) {
	if a.cur == nil {
		return
	}
	img, err := CaptureMonitorScaled(a.currentMonitor(), CaptureW, CaptureH)
	if err != nil {
		a.status = "Không chụp được màn hình: " + err.Error()
		return
	}
	if fromHotkey {
		a.det.MarkManual(time.Now())
	}
	a.saveTemplate(img, "Đã lưu ảnh mẫu mới")
}

func (a *App) saveTemplate(img image.Image, msg string) {
	if _, err := a.store.AddTemplate(a.cur, img, time.Now()); err != nil {
		a.status = "Lỗi lưu ảnh mẫu: " + err.Error()
		return
	}
	a.reloadTemplates()
	a.status = fmt.Sprintf("%s (%d/%d).", msg, len(a.cur.Templates), MaxTemplates)
}

func (a *App) setTranslate(on bool) {
	t := &a.translation
	if on && t.apiKey == "" {
		a.status = "Nhập API key Anthropic trước khi bật dịch."
		a.gotoTranslate = true
		C.bw_main_show()
		return
	}
	if on && !a.hasTranslateRegion() {
		a.startPick("translate")
		if a.pick != nil {
			a.pick.wasOn = true // start translating as soon as the region is saved
		}
		return
	}
	t.on = on
	a.syncTranslateLoop()
	if on {
		a.status = "Đang dịch vùng đã khoanh."
		a.showTranslateHint()
	} else {
		t.latest++ // a reply still in flight belongs to a session that was turned off
		a.hideSubtitle()
		a.status = "Đã tắt dịch."
	}
}

func (a *App) showTranslateHint() {
	h := a.cfg.Hotkeys
	a.showSubtitle("", "Đang đọc vùng đã khoanh. "+h.TranslateRegion+" khoanh lại · "+h.TranslateToggle+" tắt", false)
}

// startPick grabs the screen and asks the page to let the user drag a rectangle.
func (a *App) startPick(what string) {
	if a.cur == nil || a.pick != nil {
		return
	}
	mon := a.currentMonitor()
	ow, oh := OcrSize(mon)
	shot, err := CaptureMonitorScaled(mon, ow, oh)
	if err != nil {
		a.status = "Không chụp được màn hình: " + err.Error()
		return
	}
	p := &pickState{What: what, shot: shot}
	switch what {
	case "translate":
		p.Title = "Khoanh vùng hội thoại"
		p.Instr = "Kéo chuột để khoanh SÁT khung hội thoại trong game. Khoanh càng hẹp càng đỡ tốn phí và càng ít đọc nhầm."
		p.Init = a.cfg.TranslateROI
		t := &a.translation
		p.wasOn, t.on = t.on, false
		t.latest++ // a reply still in flight must not reappear
		a.syncTranslateLoop()
		a.hideSubtitle()
	case "roi":
		p.Title = "Khoanh vùng nhận diện - " + a.cur.Name
		p.Instr = "Kéo chuột để khoanh SÁT dòng chữ hoặc biểu tượng chỉ hiện khi chết (vd: YOU DIED). Tránh lấy phần cảnh game phía sau."
		p.Init = a.cur.ROI
	case "hp":
		p.Title = "Khoanh thanh máu - " + a.cur.Name
		p.Instr = "Kéo chuột khoanh THANH MÁU nhân vật, lúc đang đầy máu."
		p.Init = a.cur.HPZone
		if p.Init.W <= 0 {
			p.Init = ROI{0.05, 0.05, 0.4, 0.15}
		}
	default:
		return
	}
	a.pick = p
	C.bw_main_resize(pickerWidth, pickerHeight)
	C.bw_main_show() // the picker is in the main window, which may be hidden for a game
}

// endPick closes the picker; roi is nil when cancelled.
func (a *App) endPick(roi *ROI) {
	p := a.pick
	if p == nil {
		return
	}
	a.pick = nil
	C.bw_main_resize(mainWidth, mainHeight)
	if roi != nil {
		r := roi.Normalized()
		switch p.What {
		case "translate":
			a.cfg.TranslateROI = r
			a.store.SaveConfig(a.cfg)
			a.status = "Đã lưu vùng hội thoại."
		case "roi":
			a.cur.ROI = r
			a.store.SaveProfile(a.cur)
			a.det.Reset()
			a.reloadTemplates()
			a.status = "Đã lưu vùng nhận diện."
		case "hp":
			c, ok := LearnHPColor(p.shot, r)
			if !ok {
				a.status = "Không tìm thấy màu nổi bật trong vùng này. Hãy khoanh lúc đầy máu và khoanh sát thanh máu hơn."
				break
			}
			a.cur.HPZone, a.cur.HPColor = r, c
			a.cur.HPFull = MeasureHP(p.shot, r, c)
			a.cur.HPEnabled = true
			a.store.SaveProfile(a.cur)
			a.hpDet = NewHPDeath(a.cur.HPFull)
			a.updateLoop()
			a.status = "Đã học thanh máu."
		}
	}
	if p.What == "translate" {
		on := p.wasOn && a.hasTranslateRegion()
		a.translation.on = on
		a.syncTranslateLoop()
		if on {
			a.showTranslateHint()
		}
	}
}

func (a *App) applyTop() {
	on := C.int(0)
	if a.cfg.AlwaysOnTop {
		on = 1
	}
	C.bw_main_top(on)
}

// placeOverlays sizes the overlay windows to their text and shows the enabled ones.
func (a *App) placeOverlays() {
	if a.overlay < 0 || len(a.monitors) == 0 {
		return
	}
	var rects [][4]int32
	for _, m := range a.monitors {
		rects = append(rects, [4]int32{m.Rect.L, m.Rect.T, m.Rect.R, m.Rect.B})
	}
	x, y, ok := ClampToMonitors(a.cfg.OverlayX, a.cfg.OverlayY, rects)
	if !a.cfg.OverlayPosSet || !ok {
		x, y, _ = ClampToMonitors(-1<<30, -1<<30, rects)
	}
	h := a.cfg.OverlayFontSize*8/5 + 2*a.cfg.OutlineWidth + 24
	C.bw_overlay_frame(a.overlay, C.int(x), C.int(y), 1200, C.int(h))
	C.bw_overlay_show(a.overlay, boolC(a.cfg.OverlayEnabled || a.overlayEdit))
}

func boolC(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// toggleEdit switches an overlay's drag mode; leaving it saves where it ended up.
func (a *App) toggleEdit(id C.int, editing *bool, save func(x, y int32)) {
	if id < 0 {
		return
	}
	*editing = !*editing
	C.bw_overlay_edit(id, boolC(*editing))
	if *editing {
		C.bw_overlay_show(id, 1)
		a.status = "Kéo khung viền vàng tới vị trí bất kỳ, rồi bấm \"Xong, lưu vị trí\"."
		return
	}
	var x, y C.int
	C.bw_overlay_origin(id, &x, &y)
	save(int32(x), int32(y))
	a.store.SaveConfig(a.cfg)
	a.placeOverlays()
	a.status = "Đã lưu vị trí."
}

// uninstall removes the data, the key and the app bundle, then quits.
func (a *App) uninstall() {
	a.uninstalling = true
	saveAPIKeyMac("")
	os.RemoveAll(a.store.Dir)
	exe, err := os.Executable()
	if err == nil {
		if i := strings.Index(exe, ".app/"); i > 0 {
			os.RemoveAll(exe[:i+len(".app")])
		}
	}
	C.bw_quit()
}

// ---------- web API ----------

func (a *App) registerRoutes(srv *OverlayServer) {
	srv.mux.HandleFunc("/subtitle", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, subtitlePage)
	})
	srv.mux.HandleFunc("/subtitle/state", func(w http.ResponseWriter, r *http.Request) {
		var st subtitleView
		a.sync(func() { st = a.subtitleState })
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(st)
	})
	srv.mux.HandleFunc("/app", RequireToken(a.token, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, appPage)
	}))
	srv.mux.HandleFunc("/api/state", RequireToken(a.token, func(w http.ResponseWriter, r *http.Request) {
		var st map[string]any
		a.sync(func() { st = a.stateJSON() })
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(st)
	}))
	srv.mux.HandleFunc("/api/do", RequireToken(a.token, a.handleDo))
	srv.mux.HandleFunc("/api/shot", RequireToken(a.token, func(w http.ResponseWriter, r *http.Request) {
		var img image.Image
		a.sync(func() {
			if a.pick != nil {
				img = a.pick.shot
			}
		})
		writePNG(w, img)
	}))
	srv.mux.HandleFunc("/api/template", RequireToken(a.token, func(w http.ResponseWriter, r *http.Request) {
		i, _ := strconv.Atoi(r.URL.Query().Get("i"))
		var img image.Image
		a.sync(func() {
			if i >= 0 && i < len(a.tmplImgs) {
				img = a.tmplImgs[i]
			}
		})
		writePNG(w, img)
	}))
}

func writePNG(w http.ResponseWriter, img image.Image) {
	if img == nil {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	png.Encode(w, img)
}

func roiLabel(r ROI) string {
	return fmt.Sprintf("x %d%%, y %d%%, %d×%d%%", int(r.X*100), int(r.Y*100), int(r.W*100), int(r.H*100))
}

// stateJSON is everything the page shows. It never contains the API key.
func (a *App) stateJSON() map[string]any {
	c := a.cfg
	mons := make([]string, len(a.monitors))
	for i, m := range a.monitors {
		mons[i] = m.Label(i)
	}
	profiles := make([]map[string]string, len(a.profiles))
	for i, p := range a.profiles {
		profiles[i] = map[string]string{"id": p.ID, "name": p.Name}
	}
	presets := make([]string, len(a.presets))
	for i, p := range a.presets {
		presets[i] = p.Game
	}
	t := &a.translation
	goTr := a.gotoTranslate
	a.gotoTranslate = false
	st := map[string]any{
		"gotoTranslate": goTr, "version": version, "status": a.status, "profiles": profiles, "monitors": mons, "monitor": c.Monitor,
		"sim": a.sim, "hp": a.hp, "ocr": a.ocr, "langs": a.ocrLangs, "presets": presets, "fonts": a.fonts,
		"top": c.AlwaysOnTop, "dataDir": a.store.Dir, "obsPath": a.store.ObsPath(),
		"format": c.ObsFormat, "font": c.FontFamily, "size": c.OverlayFontSize, "bold": c.FontBold,
		"color": c.TextColor, "outline": c.Outline, "outlineWidth": c.OutlineWidth, "outlineColor": c.OutlineColor,
		"overlay": c.OverlayEnabled, "overlayEdit": a.overlayEdit, "obs": c.ObsEnabled,
		"sound": c.Sound, "volume": c.SoundVolume, "soundFile": filepath.Base(c.SoundFile), "hasSoundFile": c.SoundFile != "",
		"hotkeys": c.Hotkeys, "screenAccess": screenAccessGranted(),
		"translate": map[string]any{"on": t.on, "hasKey": t.apiKey != "", "hasRegion": a.hasTranslateRegion(),
			"font": c.TranslateFontSize, "src": a.trSrc, "dst": a.trDst, "cost": a.trCost, "region": func() string {
				if !a.hasTranslateRegion() || len(a.monitors) == 0 {
					return "Chưa khoanh"
				}
				r := c.TranslateROI.ToPixels(a.currentMonitor().Rect)
				return fmt.Sprintf("%d×%d điểm", r.R-r.L, r.B-r.T)
			}()},
	}
	if a.server != nil {
		st["url"] = a.server.URL()
	}
	if p := a.pick; p != nil {
		st["pick"] = map[string]any{"what": p.What, "title": p.Title, "instr": p.Instr, "init": p.Init, "w": p.shot.Bounds().Dx(), "h": p.shot.Bounds().Dy()}
	}
	if p := a.cur; p != nil {
		tmpl := make([]string, len(p.Templates))
		for i, n := range p.Templates {
			tmpl[i] = templateLabel(i, n)
		}
		roi := "Toàn màn hình"
		if !p.ROI.IsFull() {
			roi = roiLabel(p.ROI)
		}
		hint := "Cách nhau bằng dấu phẩy. Thêm * ở cuối để khớp phần đầu câu."
		if i := a.presetIndex(p.Preset); i >= 0 {
			hint = a.presets[i].HintText()
		}
		hpZone := "Chưa khoanh"
		if p.HPColor.Valid() {
			hpZone = fmt.Sprintf("%s · màu %.0f°", roiLabel(p.HPZone), p.HPColor.Hue)
		}
		st["cur"] = map[string]any{"id": p.ID, "name": p.Name, "deaths": p.Deaths, "auto": p.AutoEnabled, "ocr": p.OcrEnabled,
			"lang": p.OcrLang, "preset": p.Preset, "presetHint": hint, "keywords": strings.Join(EffectiveKeywords(p, c.Keywords), ", "),
			"threshold": int(p.Threshold*100 + 0.5), "cooldown": p.CooldownSec, "roi": roi, "templates": tmpl,
			"hpOn": p.HPEnabled, "hpZone": hpZone}
	}
	return st
}

type doRequest struct {
	Do    string  `json:"do"`
	S     string  `json:"s"`
	S2    string  `json:"s2"`
	N     float64 `json:"n"`
	B     bool    `json:"b"`
	ROI   *ROI    `json:"roi"`
	Style *struct {
		Font, Color, OutlineColor string
		Size, OutlineWidth        int
		Bold, Outline             bool
	} `json:"style"`
}

func (a *App) handleDo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req doRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var file string
	if req.Do == "sound-choose" { // a modal panel: the lock must stay free meanwhile
		file = goString(C.bw_choose_file())
	}
	a.sync(func() {
		if file != "" {
			a.cfg.SoundFile = file
			a.store.SaveConfig(a.cfg)
			a.status = "Đã chọn âm thanh."
			return
		}
		a.do(req)
	})
	w.WriteHeader(http.StatusNoContent)
}

// do applies one action from the page, on the same rules as the Windows handlers.
func (a *App) do(q doRequest) {
	c := &a.cfg
	save := func() { a.store.SaveConfig(*c) }
	// Actions that need no game selected.
	switch q.Do {
	case "translate":
		a.setTranslate(q.B)
		return
	case "api-key":
		key := strings.TrimSpace(q.S)
		if err := saveAPIKeyMac(key); err != nil {
			a.status = "Không lưu được API key: " + err.Error()
			return
		}
		a.translation.apiKey, a.translation.translator = key, nil
		a.status = "Đã lưu API key."
		return
	case "sub-font":
		c.TranslateFontSize = max(10, min(int(q.N), 120))
		save()
		if a.translation.on && a.subtitleState.Note+a.subtitleState.Text != "" {
			a.drawSubtitle(a.subtitleState.Text, a.subtitleState.Note, a.subtitleState.Dim)
		}
		return
	case "top":
		c.AlwaysOnTop = q.B
		save()
		a.applyTop()
		return
	case "copy":
		cstr(q.S, func(s *C.char) { C.bw_copy(s) })
		a.status = "Đã copy."
		return
	case "open-data":
		exec.Command("/usr/bin/open", a.store.Dir).Start()
		return
	case "open-privacy":
		exec.Command("/usr/bin/open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture").Start()
		return
	case "uninstall":
		a.uninstall()
		return
	case "pick-save":
		a.endPick(q.ROI)
		return
	case "pick-cancel":
		a.endPick(nil)
		return
	case "monitor":
		if i := int(q.N); i >= 0 && i < len(a.monitors) {
			c.Monitor = i
			save()
			a.updateLoop()
			a.syncTranslateLoop()
		}
		return
	case "format":
		c.ObsFormat = q.S
		save()
		a.refreshCount()
		return
	case "style":
		if s := q.Style; s != nil {
			if f := strings.TrimSpace(s.Font); f != "" {
				c.FontFamily = f
			}
			c.OverlayFontSize = max(10, min(s.Size, 300))
			c.FontBold, c.Outline = s.Bold, s.Outline
			c.OutlineWidth = max(1, min(s.OutlineWidth, 30))
			if _, err := ParseHex(s.Color); err == nil {
				c.TextColor = s.Color
			}
			if _, err := ParseHex(s.OutlineColor); err == nil {
				c.OutlineColor = s.OutlineColor
			}
			save()
			a.refreshCount()
			a.placeOverlays()
		}
		return
	case "overlay":
		c.OverlayEnabled = q.B
		save()
		a.placeOverlays()
		return
	case "overlay-edit":
		a.toggleEdit(a.overlay, &a.overlayEdit, func(x, y int32) { c.OverlayX, c.OverlayY, c.OverlayPosSet = x, y, true })
		return
	case "overlay-reset":
		c.OverlayPosSet = false
		save()
		a.placeOverlays()
		return
	case "obs":
		c.ObsEnabled = q.B
		save()
		a.refreshCount()
		return
	case "sound":
		c.Sound = q.B
		save()
		return
	case "volume":
		c.SoundVolume = max(0, min(int(q.N), 100))
		save()
		return
	case "sound-test":
		if c.SoundFile == "" {
			a.status = "Chưa chọn file âm thanh: bấm \"Chọn file...\"."
		}
		a.playSound()
		return
	case "sound-clear":
		c.SoundFile = ""
		save()
		return
	case "profile-select":
		for _, p := range a.profiles {
			if p.ID == q.S {
				a.loadProfile(p)
			}
		}
		return
	case "profile-add":
		a.addProfile(q.S)
		return
	}

	p := a.cur
	if p == nil {
		return
	}
	switch q.Do {
	case "add":
		a.addDeath(1, "button")
	case "sub":
		a.addDeath(-1, "button")
	case "reset":
		old := p.Deaths
		p.Deaths = 0
		a.store.SaveProfile(p)
		a.store.AppendHistory(time.Now(), p.Name, -old, 0, "reset")
		a.refreshCount()
	case "profile-rename":
		if name := strings.TrimSpace(q.S); name != "" {
			p.Name = name
			a.store.SaveProfile(p)
			a.refreshCount()
		}
	case "profile-delete":
		if len(a.profiles) <= 1 {
			a.status = "Phải còn ít nhất một game."
			return
		}
		a.store.DeleteProfile(p)
		for i, x := range a.profiles {
			if x == p {
				a.profiles = append(a.profiles[:i], a.profiles[i+1:]...)
				break
			}
		}
		a.loadProfile(a.profiles[0])
	case "auto":
		p.AutoEnabled = q.B
		a.store.SaveProfile(p)
		a.det.Reset()
		a.updateLoop()
	case "ocr":
		p.OcrEnabled = q.B
		a.store.SaveProfile(p)
		a.updateLoop()
	case "lang":
		p.OcrLang = q.S
		a.store.SaveProfile(p)
		a.updateLoop()
		a.status = "Đã đổi ngôn ngữ OCR cho game này."
	case "preset":
		if i := a.presetIndex(q.S); i >= 0 {
			p.Preset = a.presets[i].Game
			p.Keywords = append([]string(nil), a.presets[i].Keywords...)
		} else {
			p.Preset, p.Keywords = "", nil
		}
		a.store.SaveProfile(p)
		a.det.Reset()
		a.status = "Đã áp dụng mẫu theo game."
	case "keywords":
		kw := ParseKeywords(q.S)
		if len(kw) == 0 || strings.Join(kw, ",") == strings.Join(c.Keywords, ",") {
			p.Keywords = nil // back to the generic list
		} else {
			p.Keywords = kw
		}
		a.store.SaveProfile(p)
		a.status = fmt.Sprintf("Đã lưu %d từ khóa cho \"%s\".", len(EffectiveKeywords(p, c.Keywords)), p.Name)
	case "threshold":
		p.Threshold = max(0.5, min(q.N/100, 0.99))
		a.det.Threshold = p.Threshold
		a.store.SaveProfile(p)
	case "cooldown":
		p.CooldownSec = max(1, min(q.N, 600))
		a.det.Cooldown = time.Duration(p.CooldownSec * float64(time.Second))
		a.store.SaveProfile(p)
	case "pick":
		a.startPick(q.S)
	case "template-capture":
		a.captureTemplate(false)
	case "template-remove":
		if i := int(q.N); i >= 0 && i < len(p.Templates) {
			a.store.RemoveTemplate(p, i)
			a.reloadTemplates()
			a.status = "Đã xóa mẫu."
		}
	case "hp":
		p.HPEnabled = q.B
		a.store.SaveProfile(p)
		if p.HPEnabled && !p.HPColor.Valid() {
			a.status = "Bấm \"Khoanh thanh máu...\" lúc đang đầy máu để app học màu thanh máu."
		}
		a.updateLoop()
	}
}

func (a *App) addProfile(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	p, err := a.store.CreateProfile(name)
	if err != nil {
		a.status = "Lỗi tạo game: " + err.Error()
		return
	}
	if i := FindPreset(a.presets, name); i >= 0 {
		p.Preset = a.presets[i].Game
		p.Keywords = append([]string(nil), a.presets[i].Keywords...)
		a.store.SaveProfile(p)
		a.status = "Đã nạp mẫu cho " + p.Preset + "."
	}
	a.profiles = append(a.profiles, p)
	a.loadProfile(p)
}

// subtitlePage is the translation box, hugging the edge next to the region.
const subtitlePage = `<!doctype html>
<html lang="vi"><head><meta charset="utf-8"><title>Byrgenwerth</title>
<style>
html,body{height:100%;margin:0;background:transparent;overflow:hidden}
body{display:flex;flex-direction:column;align-items:center;justify-content:flex-start;padding:8px;box-sizing:border-box;
 font-family:-apple-system,"Helvetica Neue",sans-serif;font-weight:500;color:#eeeef0;line-height:1.4;cursor:default;user-select:none}
body[data-anchor="bottom"]{justify-content:flex-end}
#b{max-width:100%;padding:.35em .7em;border-radius:8px;background:rgb(17 17 19 / .86);text-align:center}
#b[hidden]{display:none}
#m{white-space:pre-line}#m:empty{display:none}
#b.dim #m{opacity:.55}
#n{font-size:.6em;color:#b5b2bc}#n:empty{display:none}
</style></head>
<body><div id="b" role="status" aria-live="polite" hidden><div id="m"></div><div id="n"></div></div>
<script>
var v=-1,b=document.getElementById('b'),m=document.getElementById('m'),n=document.getElementById('n');
function tick(){fetch('/subtitle/state',{cache:'no-store'}).then(function(r){return r.json()}).then(function(s){
 if(s.v===v)return;v=s.v;
 document.body.dataset.anchor=s.anchor||'top';
 if(s.size)document.body.style.fontSize=s.size+'px';
 m.textContent=s.text||'';n.textContent=s.note||'';
 b.classList.toggle('dim',!!s.dim);
 b.hidden=!(s.text||s.note);
}).catch(function(){}).then(function(){setTimeout(tick,120)})}
tick();
</script></body></html>`
