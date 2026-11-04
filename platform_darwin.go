package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=14.0 -Wno-deprecated-declarations
#cgo LDFLAGS: -mmacosx-version-min=14.0 -framework Cocoa -framework WebKit -framework ScreenCaptureKit -framework Vision -framework Carbon -framework Security -framework CoreGraphics
#include <stdlib.h>
#include "platform_darwin.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"runtime"
	"unsafe"
)

func init() {
	runtime.LockOSThread() // AppKit must run on the main thread
}

// goString returns a malloc'd C string as a Go string and frees it.
func goString(s *C.char) string {
	defer C.free(unsafe.Pointer(s))
	return C.GoString(s)
}

func cstr(s string, f func(*C.char)) {
	c := C.CString(s)
	defer C.free(unsafe.Pointer(c))
	f(c)
}

// ListMonitors returns the screens, primary first.
func ListMonitors() []Monitor {
	var raw [16 * 5]C.int32_t
	n := int(C.bw_screens(&raw[0], 16))
	mons := make([]Monitor, n)
	for i := range mons {
		e := raw[i*5:]
		mons[i] = Monitor{Rect: PxRect{int32(e[0]), int32(e[1]), int32(e[2]), int32(e[3])}, Primary: e[4] != 0}
	}
	return mons
}

// CaptureMonitorScaled grabs the monitor scaled to cw x ch.
func CaptureMonitorScaled(m Monitor, cw, ch int32) (*image.RGBA, error) {
	return captureRect(m.Rect, cw, ch)
}

// CaptureRegion grabs a screen rectangle for OCR, at most 1920 px wide. The
// rectangle is in points, so a Retina screen gets twice the pixels for free.
func CaptureRegion(r PxRect) (*image.RGBA, error) {
	w, h := r.R-r.L, r.B-r.T
	if w <= 0 || h <= 0 {
		return nil, errors.New("vùng khoanh rỗng")
	}
	cw := min(w*2, 1920)
	return captureRect(r, cw, max(h*cw/w, 1))
}

func captureRect(r PxRect, cw, ch int32) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, int(cw), int(ch)))
	e := C.bw_capture(C.int(r.L), C.int(r.T), C.int(r.R-r.L), C.int(r.B-r.T), C.int(cw), C.int(ch), (*C.uint8_t)(&img.Pix[0]))
	if e != nil {
		return nil, errors.New(goString(e))
	}
	return img, nil
}

// OcrEngine reads text with Apple Vision. tag "" lets Vision detect the language.
type OcrEngine struct {
	Lang  string
	Langs []OcrLang
}

func NewOcrEngine(tag string) (*OcrEngine, error) {
	var raw []struct{ Tag, Name string }
	if err := json.Unmarshal([]byte(goString(C.bw_ocr_langs())), &raw); err != nil {
		return nil, err
	}
	e := &OcrEngine{Lang: tag}
	for _, l := range raw {
		e.Langs = append(e.Langs, OcrLang{Tag: l.Tag, Name: l.Name})
	}
	return e, nil
}

func (e *OcrEngine) Close() {}

func (e *OcrEngine) Recognize(img *image.RGBA) ([]OcrLine, error) {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return nil, nil
	}
	var out *C.char
	cstr(e.Lang, func(lang *C.char) {
		out = C.bw_ocr((*C.uint8_t)(&img.Pix[0]), C.int(b.Dx()), C.int(b.Dy()), lang)
	})
	return parseVisionOCR(goString(out))
}

//export goHotkey
func goHotkey(id C.int) {
	go hotkeyPressed(HotkeyAction(id))
}

//export goWillQuit
func goWillQuit() {
	appWillQuit()
}

// macKeyCodes maps the Windows virtual keys ParseHotkey returns to Carbon key codes.
var macKeyCodes = map[uint32]int{
	0x70: 122, 0x71: 120, 0x72: 99, 0x73: 118, 0x74: 96, 0x75: 97, 0x76: 98, 0x77: 100, 0x78: 101, 0x79: 109, 0x7A: 103, 0x7B: 111,
	'A': 0, 'B': 11, 'C': 8, 'D': 2, 'E': 14, 'F': 3, 'G': 5, 'H': 4, 'I': 34, 'J': 38, 'K': 40, 'L': 37, 'M': 46,
	'N': 45, 'O': 31, 'P': 35, 'Q': 12, 'R': 15, 'S': 1, 'T': 17, 'U': 32, 'V': 9, 'W': 13, 'X': 7, 'Y': 16, 'Z': 6,
	'0': 29, '1': 18, '2': 19, '3': 20, '4': 21, '5': 23, '6': 22, '7': 26, '8': 28, '9': 25,
}

// registerHotkey accepts the same "Ctrl+Alt+T" spelling as Windows.
func registerHotkey(act HotkeyAction, spec string) error {
	mods, vk, err := ParseHotkey(spec)
	if err != nil {
		return err
	}
	code, ok := macKeyCodes[vk]
	if !ok {
		return fmt.Errorf("phím %q chưa hỗ trợ trên Mac", spec)
	}
	carbon := 0
	if mods&ModControl != 0 {
		carbon |= 0x1000 // controlKey
	}
	if mods&ModAlt != 0 {
		carbon |= 0x0800 // optionKey
	}
	if mods&ModShift != 0 {
		carbon |= 0x0200 // shiftKey
	}
	if mods&ModWin != 0 {
		carbon |= 0x0100 // cmdKey
	}
	if C.bw_hotkey(C.int(act), C.int(code), C.int(carbon)) == 0 {
		return fmt.Errorf("không đăng ký được %s (có app khác đang dùng?)", spec)
	}
	return nil
}

func saveAPIKeyMac(key string) error {
	var ok C.int
	cstr(key, func(k *C.char) { ok = C.bw_key_save(k) })
	if ok == 0 {
		return errors.New("Keychain từ chối lưu")
	}
	return nil
}

func loadAPIKeyMac() string { return goString(C.bw_key_load()) }

func fontFamilies() []string {
	var out []string
	json.Unmarshal([]byte(goString(C.bw_font_families())), &out)
	return out
}

func screenAccessGranted() bool { return C.bw_screen_access(0) != 0 }
