//go:build windows

package main

import (
	"image"
	"runtime"
	"strings"
	"time"
)

const translateInterval = 250 * time.Millisecond

func (a *Engine) captureLoop() {
	defer a.wg.Done()
	// On Windows, WinRT OCR objects live on this thread for its whole life.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var ocr *OcrEngine
	defer func() { ocr.Close() }()
	ocrTried := false

	interval := time.Duration(a.cfg.IntervalMs) * time.Millisecond
	lastErr := ""
	for {
		select {
		case <-a.quit:
			return
		case <-time.After(interval):
		}
		a.mu.Lock()
		st := a.loop
		a.mu.Unlock()
		if !st.enabled {
			continue
		}
		var newLangs []OcrLang
		if st.ocr && (!ocrTried || (ocr != nil && ocr.Lang != st.ocrLang)) {
			ocrTried = true
			ocr.Close()
			var err error
			ocr, err = NewOcrEngine(st.ocrLang)
			if err != nil {
				ocr = nil
				msg := err.Error()
				a.ui.sync(func() { a.onOcrUnavailable(msg) })
				continue
			}
			newLangs = ocr.Langs
		}

		fr := frameResult{gen: st.gen, langs: newLangs}
		var err error
		if st.ocr && ocr != nil {
			ow, oh := OcrSize(st.mon)
			var big *image.RGBA
			big, err = CaptureMonitorScaled(st.mon, ow, oh)
			if err == nil {
				fr.small = Downscale(big, CaptureW, CaptureH)
				fr.lines, fr.ocrErr = ocr.Recognize(big)
				fr.ranOcr = true
			}
		} else {
			fr.small, err = CaptureMonitor(st.mon)
		}
		if err != nil {
			if err.Error() != lastErr {
				lastErr = err.Error()
				msg := err.Error()
				a.ui.sync(func() { a.ui.setStatus("Lỗi chụp màn hình: " + msg) })
			}
			continue
		}
		lastErr = ""
		if st.hasTemplates {
			fr.feat = ExtractFeature(fr.small, st.roi)
			fr.hasFeat = true
		}
		a.ui.sync(func() { a.onFrame(fr) })
	}
}

// translateLoop reads the region and reports settled lines to the UI thread.
func (a *Engine) translateLoop() {
	defer a.wg.Done()
	// On Windows, WinRT OCR objects live on this thread for its whole life.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var ocr *OcrEngine
	defer func() { ocr.Close() }()

	var stab Stabilizer
	gen := 0
	lastErr := ""
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
				a.ui.sync(func() { a.ui.translateOcrFailed(msg) })
				continue
			}
		}
		img, err := CaptureRegion(st.region)
		if err != nil {
			if msg := err.Error(); msg != lastErr {
				lastErr = msg
				a.ui.sync(func() { a.ui.setStatus("Không chụp được vùng hội thoại: " + msg) })
			}
			continue
		}
		lastErr = ""
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
			a.ui.sync(func() { a.onStableLine(st.gen, line, ev) })
		}
	}
}

// CaptureMonitor grabs the monitor scaled down to CaptureW x CaptureH.
func CaptureMonitor(m Monitor) (*image.RGBA, error) {
	return CaptureMonitorScaled(m, CaptureW, CaptureH)
}
