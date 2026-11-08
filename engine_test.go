package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

// fakeUI records what the engine asks of the platform.
type fakeUI struct {
	status, src, dst string
	game             bool
	subtitle         string
	subtitleOn       bool
	counts, sounds   int
	own              []ROI
}

func (f *fakeUI) sync(g func())                    { g() }
func (f *fakeUI) setStatus(s string)               { f.status = s }
func (f *fakeUI) simText(string)                   {}
func (f *fakeUI) ocrText(string)                   {}
func (f *fakeUI) setOcrLanguages([]OcrLang)        {}
func (f *fakeUI) ownRects() []ROI                  { return f.own }
func (f *fakeUI) inGame() bool                     { return f.game }
func (f *fakeUI) playSound()                       { f.sounds++ }
func (f *fakeUI) refreshCount()                    { f.counts++ }
func (f *fakeUI) refreshROILabel()                 {}
func (f *fakeUI) reloadTemplates()                 {}
func (f *fakeUI) trSrcText(s string)               { f.src = s }
func (f *fakeUI) trDstText(s string)               { f.dst = s }
func (f *fakeUI) trCostText(string)                {}
func (f *fakeUI) translateOcrFailed(string)        {}
func (f *fakeUI) drawSubtitle(t, n string, d bool) { f.subtitle, f.subtitleOn = t+n, true }
func (f *fakeUI) eraseSubtitle()                   { f.subtitle, f.subtitleOn = "", false }

func newTestEngine(t *testing.T) (*Engine, *fakeUI) {
	t.Helper()
	ui := &fakeUI{game: true}
	e := &Engine{ui: ui, store: &Store{Dir: t.TempDir()}, cfg: DefaultConfig(), quit: make(chan struct{}), ocrAvailable: true,
		monitors: []Monitor{{Rect: PxRect{R: 1920, B: 1080}, Primary: true}}}
	p, err := e.store.CreateProfile("Dark Souls")
	if err != nil {
		t.Fatal(err)
	}
	p.AutoEnabled, p.OcrEnabled = true, true
	e.cur = p
	e.det = NewDetector(p.Threshold, 15*time.Second)
	return e, ui
}

// bigLine builds an OCR line of large centered text.
func bigLine(text string) OcrLine {
	return OcrLine{Text: text, Words: []OcrWord{{Text: text, X: 0.35, Y: 0.45, W: 0.3, H: 0.08}}}
}

func TestEngineCountsTheFirstDeathOnItsFirstOcrRead(t *testing.T) {
	e, ui := newTestEngine(t)
	e.onFrame(frameResult{ranOcr: true, lines: []OcrLine{bigLine("YOU DIED")}})
	if e.cur.Deaths != 1 || ui.counts == 0 || ui.sounds != 1 {
		t.Fatalf("deaths=%d refreshes=%d sounds=%d", e.cur.Deaths, ui.counts, ui.sounds)
	}
	if !strings.Contains(ui.status, "YOU DIED") {
		t.Fatalf("status %q", ui.status)
	}
	e.onFrame(frameResult{ranOcr: true, lines: []OcrLine{bigLine("YOU DIED")}})
	if e.cur.Deaths != 1 {
		t.Fatalf("one death screen counted %d times", e.cur.Deaths)
	}
}

func TestEngineIgnoresTextOnTheAppsOwnWindows(t *testing.T) {
	e, ui := newTestEngine(t)
	ui.own = []ROI{{X: 0.3, Y: 0.4, W: 0.4, H: 0.2}} // e.g. the subtitle says BẠN ĐÃ CHẾT
	e.onFrame(frameResult{ranOcr: true, lines: []OcrLine{bigLine("YOU DIED")}})
	if e.cur.Deaths != 0 {
		t.Fatal("counted text from the app's own window")
	}
}

func TestEngineIgnoresFramesOfAnOldGeneration(t *testing.T) {
	e, _ := newTestEngine(t)
	e.gen = 2
	e.onFrame(frameResult{gen: 1, ranOcr: true, lines: []OcrLine{bigLine("YOU DIED")}})
	if e.cur.Deaths != 0 {
		t.Fatal("a frame from before a profile switch was counted")
	}
}

func TestEngineMinusOneNeverGoesBelowZero(t *testing.T) {
	e, _ := newTestEngine(t)
	e.addDeath(-1, "button")
	if e.cur.Deaths != 0 {
		t.Fatalf("deaths %d", e.cur.Deaths)
	}
}

func TestEngineTranslatesASettledLineAndDropsAStaleReply(t *testing.T) {
	e, ui := newTestEngine(t)
	e.cfg.TranslateROI = ROI{X: 0.1, Y: 0.7, W: 0.8, H: 0.2}
	tr := &e.translation
	tr.on, tr.apiKey = true, "k"
	tr.ctx, tr.cancel = context.WithCancel(context.Background())
	defer tr.cancel()
	release := make(chan struct{})
	tr.translator = NewTranslator("Dark Souls", func(context.Context, string) (TranslateReply, error) {
		<-release
		return okReply, nil
	})
	e.syncTranslateLoop()
	gen := tr.loop.gen

	e.onStableLine(gen, "Hello", StableText)
	if ui.src != "Hello" || ui.subtitle != "Đang dịch…" {
		t.Fatalf("src=%q subtitle=%q", ui.src, ui.subtitle)
	}
	e.onStableLine(gen, "", StableClear) // the text left while the reply is in flight
	close(release)
	e.wg.Wait()
	if ui.subtitleOn || ui.dst != "" {
		t.Fatalf("a stale reply reappeared: on=%v dst=%q", ui.subtitleOn, ui.dst)
	}

	e.onStableLine(gen, "Hello again", StableText)
	e.wg.Wait()
	if ui.dst != "Xin chào" || ui.subtitle != "Xin chào" {
		t.Fatalf("dst=%q subtitle=%q", ui.dst, ui.subtitle)
	}
}

func TestEngineRefusesARegionWithTooMuchText(t *testing.T) {
	e, ui := newTestEngine(t)
	e.cfg.TranslateROI = ROI{W: 1, H: 1}
	tr := &e.translation
	tr.on = true
	e.syncTranslateLoop()
	e.onStableLine(tr.loop.gen, strings.Repeat("a", MaxTranslateChars+1), StableText)
	if !strings.Contains(ui.subtitle, "quá nhiều chữ") {
		t.Fatalf("subtitle %q", ui.subtitle)
	}
}

func TestOcrOffStopsTheOcrLoop(t *testing.T) {
	e, _ := newTestEngine(t)
	e.updateLoop()
	if !e.loop.ocr || !e.loop.enabled {
		t.Fatalf("OCR on must run the loop: %+v", e.loop)
	}
	e.cur.OcrEnabled = false
	e.updateLoop()
	if e.loop.ocr || e.loop.enabled {
		t.Fatalf("OCR off with no templates or health bar must not capture: %+v", e.loop)
	}
}
