package main

import (
	"context"
	"fmt"
	"image"
	"strings"
	"sync"
	"time"
)

type HotkeyAction int

const (
	ActAdd HotkeyAction = iota + 1
	ActSub
	ActCapture
	ActToggle
	ActWindow
	ActTranslate
	ActTranslateRegion
)

const newProfileName = "Game mới"

// OcrLang is one language the platform OCR can read.
type OcrLang struct {
	Tag  string
	Name string
}

// EngineUI is what the shared death count and translation logic asks of the
// platform: each app shows these its own way. All methods except sync run
// inside sync, on what the platform treats as its UI thread.
type EngineUI interface {
	sync(f func())
	setStatus(s string)
	simText(s string)
	hpText(s string)
	ocrText(s string)
	setOcrLanguages(langs []OcrLang)
	ownRects() []ROI
	inGame() bool
	playSound()
	refreshCount()
	refreshBosses()
	refreshROILabel()
	reloadTemplates()
	trSrcText(s string)
	trDstText(s string)
	trCostText(s string)
	translateOcrFailed(msg string)
	drawSubtitle(text, note string, dim bool)
	eraseSubtitle()
}

// Engine is the logic shared by the Windows and macOS apps.
type Engine struct {
	ui       EngineUI
	store    *Store
	cfg      Config
	cur      *Profile
	presets  []Preset
	monitors []Monitor

	tmplFeats []Feature
	det       *Detector
	gen       int

	mu   sync.Mutex
	loop loopState
	quit chan struct{}
	wg   sync.WaitGroup

	ocrAvailable bool
	ocrInitErr   string
	recentOcr    []OcrFrame // the last ~10 s of OCR, written to the miss log on a manual +1

	hpDet       *HPDeath
	tracker     *BossTracker
	curBoss     string
	lastBossHit string
	lastBossAt  time.Time
	// deathNoBoss is when a death was counted before its boss was confirmed. In
	// Souls games the name is often read cleanly only on the death screen.
	deathNoBoss time.Time
	// ocrProven: OCR has read this game's death text this session. From then on
	// a template match alone does not count: templates of a dark death screen
	// also match the dark respawn or a dimmed screenshot tool.
	ocrProven bool
	// loggedAway: the "game not in front" line was written to ocr-debug.log.
	loggedAway bool

	translation translationState
}

type loopState struct {
	enabled      bool
	hp           bool
	hpZone       ROI
	hpColor      HPColor
	ocr          bool
	ocrLang      string
	hasTemplates bool
	mon          Monitor
	roi          ROI
	gen          int
}

type frameResult struct {
	gen     int
	hpRaw   float64 // -1 when not measured
	black   bool
	langs   []OcrLang // set when the OCR engine was (re)created
	small   *image.RGBA
	feat    Feature
	hasFeat bool
	ranOcr  bool
	lines   []OcrLine
	ocrErr  error
}

// translationState belongs to the UI thread, the only writer. translateLoop
// reads loop; writes to it take a.mu.
type translationState struct {
	on         bool
	apiKey     string
	translator *Translator
	shown      string // the subtitle line on screen, "" when none
	latest     int    // the newest line sent for translation; older replies are dropped
	picking    bool   // the region dialog is open
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

func (a *Engine) onOcrUnavailable(msg string) {
	a.ocrAvailable = false
	a.ocrInitErr = msg
	a.ui.ocrText("Không dùng được: " + msg + ". Vẫn dùng được F9 + ảnh mẫu.")
	a.updateLoop()
}

// rememberOcr writes the frame to ocr-debug.log and keeps the last 10 seconds
// for the miss log.
func (a *Engine) rememberOcr(lines []OcrLine, now time.Time) {
	var texts []string
	for _, ln := range lines {
		texts = append(texts, DescribeLine(ln))
	}
	// Only log frames of the game: a browser in front fills the log in minutes.
	if inGame := a.ui.inGame(); inGame {
		a.store.AppendDebugLog("OCR  "+strings.Join(texts, " | "), now)
		a.loggedAway = false
	} else if !a.loggedAway {
		a.store.AppendDebugLog("OCR  (game not in front: not logged until it is)", now)
		a.loggedAway = true
	}
	a.recentOcr = append(a.recentOcr, OcrFrame{At: now, Lines: texts})
	for len(a.recentOcr) > 0 && now.Sub(a.recentOcr[0].At) > 10*time.Second {
		a.recentOcr = a.recentOcr[1:]
	}
}

func describeOcr(lines []OcrLine, m OcrMatch, hit bool) string {
	if hit {
		return fmt.Sprintf("Thấy \"%s\" (cao %.1f%%) → KHỚP", m.Line, m.Height*100)
	}
	best, bh := "", 0.0
	for _, ln := range lines {
		for _, w := range ln.Words {
			if w.H > bh {
				bh, best = w.H, ln.Text
			}
		}
	}
	if best == "" {
		return "Không thấy chữ"
	}
	r := []rune(best)
	if len(r) > 40 {
		best = string(r[:40]) + "…"
	}
	return fmt.Sprintf("Chữ lớn nhất \"%s\" (%.1f%%), không phải từ khóa", best, bh*100)
}

func (a *Engine) onFrame(fr frameResult) {
	if fr.gen != a.gen || a.cur == nil {
		return
	}
	now := time.Now()
	sim, idx := 0.0, -1
	if fr.hasFeat {
		sim, idx = BestSimilarity(fr.feat, a.tmplFeats)
		a.ui.simText(fmt.Sprintf("%d%%  (ngưỡng %d%%)", int(sim*100), int(a.cur.Threshold*100)))
	}
	if fr.langs != nil {
		a.ui.setOcrLanguages(fr.langs)
	}
	if fr.hpRaw >= 0 && a.hpDet != nil {
		ratio := a.hpDet.Ratio(fr.hpRaw)
		if a.hpDet.Full > a.cur.HPFull*1.05 { // calibration grew: remember it
			a.cur.HPFull = a.hpDet.Full
			a.store.SaveProfile(a.cur)
		}
		switch {
		case fr.black:
			a.ui.hpText("-- (màn hình đen)")
		case ratio < 0:
			a.ui.hpText("-- (chưa thấy thanh máu)")
		default:
			a.ui.hpText(fmt.Sprintf("%d%%", int(ratio*100+0.5)))
		}
		if a.hpDet.Feed(ratio, fr.black, now) && a.det.Trigger(now) {
			a.addDeath(1, "hp")
			a.ui.setStatus(fmt.Sprintf("Tự đếm lúc %s (máu về 0 rồi màn hình tối). Đếm sai thì bấm %s.", now.Format("15:04:05"), a.cfg.Hotkeys.Sub))
		}
	}
	var m OcrMatch
	hit := false
	if fr.ranOcr {
		fr.lines = DropLinesInside(fr.lines, a.ui.ownRects())
		if fr.ocrErr == nil {
			a.rememberOcr(fr.lines, now)
		}
		if fr.ocrErr != nil {
			a.ui.ocrText("Lỗi: " + fr.ocrErr.Error())
		} else {
			kw := EffectiveKeywords(a.cur, a.cfg.Keywords)
			if a.cur.OcrEnabled {
				m, hit = MatchDeath(fr.lines, kw, a.cfg.OcrMinHeight)
				a.ui.ocrText(describeOcr(fr.lines, m, hit))
			}
			// Boss names come from the game: skip frames where another window (a
			// browser, chat) is in front, or its text becomes the "boss".
			if a.cur.BossTrack && a.ui.inGame() {
				own := []string{a.cur.Name, "Deaths", FormatObs(a.cfg.ObsFormat, a.cur.Name, a.cur.Deaths)}
				a.tracker.Observe(WithoutNames(BossCandidates(fr.lines, a.bossZones(), kw), own), now)
				b, _ := a.tracker.Current(now)
				if b != a.curBoss {
					a.curBoss = b
					a.store.AppendDebugLog("BOSS "+b, now)
					if b != "" && !a.deathNoBoss.IsZero() && now.Sub(a.deathNoBoss) <= 15*time.Second {
						a.lastBossHit, a.lastBossAt = AddBossDeath(a.cur, b, 1), now
						a.deathNoBoss = time.Time{}
						a.store.SaveProfile(a.cur)
						a.store.AppendDebugLog("BOSS credited the death just before: "+b, now)
					}
					a.ui.refreshBosses()
				}
			}
		}
	}
	eff := sim
	if hit {
		eff = 1
		a.ocrProven = true
	} else if a.ocrProven {
		eff = 0
	}
	counted := false
	if hit && StrongKeyword(m.Keyword) {
		counted = a.det.FeedStrong(now)
	} else {
		counted = a.det.Feed(eff, now)
	}
	if !counted {
		return
	}
	a.addDeath(1, "auto")
	var why string
	if hit {
		why = fmt.Sprintf("đọc được chữ \"%s\"", m.Line)
	} else {
		why = fmt.Sprintf("giống mẫu #%d: %d%%", idx+1, int(sim*100))
	}
	msg := fmt.Sprintf("Tự đếm lúc %s (%s). Đếm sai thì bấm %s.", now.Format("15:04:05"), why, a.cfg.Hotkeys.Sub)
	a.store.AppendDebugLog("WHY  "+why+", boss: "+a.lastBossHit, now)
	if hit {
		msg += a.learnFromOcr(fr.small, m, sim)
	}
	a.ui.setStatus(msg)
}

// learnFromOcr turns an OCR-detected death into a template (and a tight ROI the
// first time), so stylized frames OCR misreads later are still caught.
func (a *Engine) learnFromOcr(small *image.RGBA, m OcrMatch, sim float64) string {
	if small == nil || (len(a.tmplFeats) > 0 && sim >= a.cur.Threshold) {
		return ""
	}
	note := ""
	if a.cur.ROI.IsFull() {
		a.cur.ROI = AutoROI(m.Box)
		a.store.SaveProfile(a.cur)
		a.ui.refreshROILabel()
		note = " Đã tự khoanh vùng quanh chữ."
	}
	if _, err := a.store.AddTemplate(a.cur, small, time.Now()); err != nil {
		return " (không lưu được ảnh mẫu: " + err.Error() + ")"
	}
	a.ui.reloadTemplates()
	return fmt.Sprintf(" Tự học: đã lưu ảnh mẫu %d/%d.%s", len(a.cur.Templates), MaxTemplates, note)
}

func (a *Engine) updateLoop() {
	a.gen++
	st := loopState{gen: a.gen}
	if a.cur != nil {
		st.hasTemplates = len(a.tmplFeats) > 0
		st.ocr = (a.cur.OcrEnabled || a.cur.BossTrack) && a.ocrAvailable
		st.ocrLang = a.cur.OcrLang
		st.hp = a.cur.HPEnabled && a.cur.HPColor.Valid()
		st.hpZone, st.hpColor = a.cur.HPZone, a.cur.HPColor
		st.enabled = a.cur.AutoEnabled && (st.hasTemplates || st.ocr || st.hp)
		st.roi = a.cur.ROI
	}
	if a.cfg.Monitor >= 0 && a.cfg.Monitor < len(a.monitors) {
		st.mon = a.monitors[a.cfg.Monitor]
	} else if len(a.monitors) > 0 {
		st.mon = a.monitors[0]
	}
	a.mu.Lock()
	a.loop = st
	a.mu.Unlock()
	if a.cur == nil {
		return
	}
	if !st.hasTemplates {
		a.ui.simText("-- (chưa có mẫu, sẽ tự học khi OCR bắt được)")
	}
	switch {
	case !a.cur.AutoEnabled:
		a.ui.simText("-- (tự nhận diện đang tắt)")
		a.ui.ocrText("--")
	case !a.ocrAvailable && a.ocrInitErr != "":
		// keep the error text set by onOcrUnavailable
	case !a.cur.OcrEnabled:
		a.ui.ocrText("Đang tắt")
	case !st.enabled:
		a.ui.ocrText("--")
	}
}

func (a *Engine) currentMonitor() Monitor {
	if a.cfg.Monitor >= 0 && a.cfg.Monitor < len(a.monitors) {
		return a.monitors[a.cfg.Monitor]
	}
	return a.monitors[0]
}

func (a *Engine) addDeath(delta int, source string) {
	if a.cur == nil {
		return
	}
	if a.cur.Deaths+delta < 0 {
		return
	}
	a.cur.Deaths += delta
	a.store.AppendDebugLog(fmt.Sprintf("COUNT %+d %s (total %d)", delta, source, a.cur.Deaths), time.Now())
	if delta > 0 && a.cfg.Sound {
		a.ui.playSound()
	}
	if a.cur.BossTrack && a.tracker != nil {
		now := time.Now()
		if delta > 0 {
			a.lastBossHit, a.deathNoBoss = "", now
			if b, ok := a.tracker.Current(now); ok {
				a.lastBossHit = AddBossDeath(a.cur, b, 1)
				a.lastBossAt = now
				a.deathNoBoss = time.Time{}
			}
		} else {
			a.deathNoBoss = time.Time{} // a −1 cancels a pending late boss credit
			if a.lastBossHit != "" && now.Sub(a.lastBossAt) < 3*time.Minute {
				AddBossDeath(a.cur, a.lastBossHit, -1) // undo the boss credit with the −1
				a.lastBossHit = ""
			}
		}
	}
	a.store.SaveProfile(a.cur)
	a.store.AppendHistory(time.Now(), a.cur.Name, delta, a.cur.Deaths, source)
	a.ui.refreshCount()
	a.ui.refreshBosses()
}

// bossZones: the user's region, else the game preset's, else the defaults.
func (a *Engine) bossZones() []ROI {
	if a.cur != nil && a.cur.BossROI.W > 0 && a.cur.BossROI.H > 0 {
		return []ROI{a.cur.BossROI.Normalized()}
	}
	if z := a.presetBossZone(); z != nil {
		return []ROI{*z}
	}
	return DefaultBossZones
}

func (a *Engine) presetBossZone() *ROI {
	if a.cur == nil {
		return nil
	}
	if i := a.presetIndex(a.cur.Preset); i >= 0 && a.presets[i].BossZone != nil {
		z := a.presets[i].BossZone.Normalized()
		return &z
	}
	return nil
}

func (a *Engine) presetIndex(name string) int {
	for i, p := range a.presets {
		if p.Game == name {
			return i
		}
	}
	return -1
}

func (a *Engine) hasTranslateRegion() bool { return a.cfg.TranslateROI.W > 0 }

// syncTranslateLoop hands the current settings to the OCR loop and restarts its
// line tracking.
func (a *Engine) syncTranslateLoop() {
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
func (a *Engine) refreshTranslateIgnore() {
	if !a.hasTranslateRegion() {
		return
	}
	var ignore []ROI
	for _, own := range a.ui.ownRects() {
		ignore = append(ignore, own.RelativeTo(a.cfg.TranslateROI))
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.translation.loop.ignore = ignore
}

func (a *Engine) onStableLine(gen int, line string, ev StableEvent) {
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

func (a *Engine) translateLine(line string) {
	t := &a.translation
	t.latest++
	request := t.latest
	a.ui.trSrcText(line)
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
	if t.shown != "" {
		a.showSubtitle(t.shown, "", true)
	} else {
		a.showSubtitle("", "Đang dịch…", false)
	}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		res, err := tr.Translate(t.ctx, line)
		a.ui.sync(func() { a.onTranslated(request, line, tr, res, err) })
	}()
}

func (a *Engine) onTranslated(request int, line string, tr *Translator, res TranslateResult, err error) {
	if request != a.translation.latest { // newer text is already on its way
		return
	}
	if err != nil {
		msg := DescribeTranslateError(err)
		a.ui.trDstText(msg)
		// Still show the English: an unreadable second is worse than an untranslated one.
		a.showSubtitle(line, msg, false)
		return
	}
	a.ui.trDstText(res.Text)
	a.showSubtitle(res.Text, "", false)
	took := "lấy từ cache"
	if !res.Cached {
		took = fmt.Sprintf("%d ms", res.Took.Milliseconds())
	}
	a.ui.trCostText(fmt.Sprintf("Câu vừa rồi: %s  ·  Chi phí ước tính phiên này: $%.4f", took, tr.TotalUSD()))
}

func (a *Engine) showSubtitle(text, note string, dim bool) {
	a.translation.shown = text
	a.ui.drawSubtitle(text, note, dim)
	a.refreshTranslateIgnore()
}

func (a *Engine) hideSubtitle() {
	a.translation.shown = ""
	a.ui.eraseSubtitle()
	a.refreshTranslateIgnore()
}

func templateLabel(i int, name string) string {
	var d, t string
	var ms int
	if n, _ := fmt.Sscanf(strings.TrimSuffix(name, ".png"), "death_%8s_%6s_%d", &d, &t, &ms); n >= 2 && len(d) == 8 && len(t) == 6 {
		return fmt.Sprintf("#%d  %s/%s %s:%s:%s", i+1, d[6:8], d[4:6], t[0:2], t[2:4], t[4:6])
	}
	return fmt.Sprintf("#%d  %s", i+1, name)
}
