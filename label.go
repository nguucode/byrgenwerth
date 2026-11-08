package main

// The label shown with the death count (a word or a picture in one corner around
// the number), the pixel helpers that put it together for the on-screen overlay,
// and the names of keys and gamepad buttons used by the narration setup.

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	xdraw "golang.org/x/image/draw"
)

// LabelFromFormat turns the display format of versions before 0.12, such as
// "Deaths: {count}", into the label word ("Deaths").
func LabelFromFormat(format string) string {
	s := strings.ReplaceAll(format, "{count}", "")
	s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), ":·-|"))
	if s == "" {
		return "Deaths"
	}
	return s
}

// CornerIndex is the position of c in LabelCorners, -1 when unknown.
func CornerIndex(c string) int {
	for i, k := range LabelCorners {
		if k == c {
			return i
		}
	}
	return -1
}

// CornerName is how a corner reads in the UI.
func CornerName(c string) string {
	switch c {
	case CornerTR:
		return "Trên, bên phải"
	case CornerBL:
		return "Dưới, bên trái"
	case CornerBR:
		return "Dưới, bên phải"
	}
	return "Trên, bên trái"
}

// LabelTextFor is the label word with {game} filled in; "" when no word is shown.
func LabelTextFor(c Config, game string) string {
	if c.LabelMode != LabelModeText {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(c.LabelText, "{game}", game))
}

// TextFileLine is what goes into the plain text file for OBS's Text source.
func TextFileLine(c Config, game string, count int) string {
	if l := LabelTextFor(c, game); l != "" {
		return fmt.Sprintf("%s %d", l, count)
	}
	return fmt.Sprint(count)
}

// Box is a placed rectangle in pixels.
type Box struct{ X, Y, W, H int }

// LabelLayout places the count (cw x ch) and the label (lw x lh) for a corner:
// the label sits above (top corners) or below (bottom corners) the count, lined
// up on the left or right edge. A zero-size label leaves the count alone.
func LabelLayout(cw, ch, lw, lh int, corner string, gap int) (w, h int, count, label Box) {
	if lw <= 0 || lh <= 0 {
		return cw, ch, Box{0, 0, cw, ch}, Box{}
	}
	w, h = max(cw, lw), ch+gap+lh
	count, label = Box{0, 0, cw, ch}, Box{0, 0, lw, lh}
	if corner == CornerBL || corner == CornerBR {
		label.Y = ch + gap
	} else {
		count.Y = lh + gap
	}
	if corner == CornerTR || corner == CornerBR {
		count.X, label.X = w-cw, w-lw
	}
	return w, h, count, label
}

// BlitPremul draws src (premultiplied BGRA, sw x sh) over dst (dw x dh) at x, y.
func BlitPremul(dst []byte, dw, dh int, src []byte, sw, sh, x, y int) {
	for sy := 0; sy < sh; sy++ {
		dy := y + sy
		if dy < 0 || dy >= dh {
			continue
		}
		for sx := 0; sx < sw; sx++ {
			dx := x + sx
			if dx < 0 || dx >= dw {
				continue
			}
			s, d := (sy*sw+sx)*4, (dy*dw+dx)*4
			inv := 255 - int(src[s+3])
			for k := 0; k < 4; k++ {
				dst[d+k] = byte(int(src[s+k]) + (int(dst[d+k])*inv+127)/255)
			}
		}
	}
}

// AddEditBackground puts the translucent box with a yellow border used while the
// overlay is being moved under what is already drawn (premultiplied BGRA).
func AddEditBackground(buf []byte, w, h int) {
	for i := 0; i < w*h; i++ {
		x, y := i%w, i/w
		bgA, bgR, bgG, bgB := 0.55, 30.0, 30.0, 30.0
		if x < 2 || y < 2 || x >= w-2 || y >= h-2 {
			bgA, bgR, bgG, bgB = 0.95, 255, 200, 40
		}
		a := float64(buf[i*4+3]) / 255
		buf[i*4+0] = byte(float64(buf[i*4+0]) + bgB*bgA*(1-a) + 0.5)
		buf[i*4+1] = byte(float64(buf[i*4+1]) + bgG*bgA*(1-a) + 0.5)
		buf[i*4+2] = byte(float64(buf[i*4+2]) + bgR*bgA*(1-a) + 0.5)
		buf[i*4+3] = byte((a+bgA*(1-a))*255 + 0.5)
	}
}

// PremulBGRA converts an image to premultiplied BGRA (Go's RGBA is premultiplied).
func PremulBGRA(img *image.RGBA) []byte {
	b := img.Bounds()
	out := make([]byte, b.Dx()*b.Dy()*4)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			s := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			d := (y*b.Dx() + x) * 4
			out[d], out[d+1], out[d+2], out[d+3] = img.Pix[s+2], img.Pix[s+1], img.Pix[s], img.Pix[s+3]
		}
	}
	return out
}

// LabelImageExts are the picture files the label accepts.
var LabelImageExts = []string{".png", ".svg"}

// LoadLabelImage reads a PNG or SVG file and scales it to the given height,
// keeping its proportions.
func LoadLabelImage(path string, height int) (*image.RGBA, error) {
	if height < 1 {
		height = 1
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		icon, err := oksvg.ReadIconStream(f, oksvg.IgnoreErrorMode)
		if err != nil {
			return nil, err
		}
		vw, vh := icon.ViewBox.W, icon.ViewBox.H
		if vw <= 0 || vh <= 0 {
			return nil, errors.New("file SVG không có kích thước (viewBox)")
		}
		w := max(1, int(float64(height)*vw/vh+0.5))
		icon.SetTarget(0, 0, float64(w), float64(height))
		img := image.NewRGBA(image.Rect(0, 0, w, height))
		scanner := rasterx.NewScannerGV(w, height, img, img.Bounds())
		icon.Draw(rasterx.NewDasher(w, height, scanner), 1)
		return img, nil
	case ".png":
		src, err := png.Decode(f)
		if err != nil {
			return nil, err
		}
		sb := src.Bounds()
		if sb.Dx() == 0 || sb.Dy() == 0 {
			return nil, errors.New("ảnh trống")
		}
		w := max(1, int(float64(height)*float64(sb.Dx())/float64(sb.Dy())+0.5))
		img := image.NewRGBA(image.Rect(0, 0, w, height))
		xdraw.CatmullRom.Scale(img, img.Bounds(), src, sb, xdraw.Over, nil)
		return img, nil
	}
	return nil, errors.New("chỉ nhận file .png hoặc .svg")
}

// ---------- keys and gamepad buttons ----------

// KeyName is the hotkey text ParseHotkey reads back for a virtual key, "" when
// the key cannot be a hotkey (a modifier on its own, for instance).
func KeyName(vk uint32) string {
	switch {
	case vk >= 'A' && vk <= 'Z', vk >= '0' && vk <= '9':
		return string(rune(vk))
	case vk >= 0x70 && vk <= 0x87:
		return fmt.Sprintf("F%d", vk-0x70+1)
	case vk >= 0x60 && vk <= 0x69:
		return fmt.Sprintf("Num%d", vk-0x60)
	}
	for name, k := range namedKeys {
		if k == vk {
			return keyDisplay[name]
		}
	}
	return ""
}

var keyDisplay = map[string]string{
	"PAUSE": "Pause", "SCROLLLOCK": "ScrollLock", "INSERT": "Insert", "DELETE": "Delete",
	"HOME": "Home", "END": "End", "PAGEUP": "PageUp", "PAGEDOWN": "PageDown",
	"SPACE": "Space", "TAB": "Tab", "BACKSPACE": "Backspace",
	"UP": "Up", "DOWN": "Down", "LEFT": "Left", "RIGHT": "Right",
	"NUMADD": "NumAdd", "NUMSUBTRACT": "NumSubtract", "NUMMULTIPLY": "NumMultiply", "NUMDIVIDE": "NumDivide", "NUMDECIMAL": "NumDecimal",
	"PLUS": "Plus", "MINUS": "Minus",
}

// HotkeyText joins modifiers and a key the way ParseHotkey expects.
func HotkeyText(ctrl, alt, shift bool, vk uint32) string {
	k := KeyName(vk)
	if k == "" {
		return ""
	}
	var parts []string
	if ctrl {
		parts = append(parts, "Ctrl")
	}
	if alt {
		parts = append(parts, "Alt")
	}
	if shift {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, k), "+")
}

// Xbox (XInput) buttons. The triggers are not buttons in XInput; they get the
// two bits above the real ones once pressed past half way.
var padButtons = []struct {
	Name string
	Bit  uint32
}{
	{"A", 0x1000}, {"B", 0x2000}, {"X", 0x4000}, {"Y", 0x8000},
	{"LB", 0x0100}, {"RB", 0x0200}, {"LT", 0x10000}, {"RT", 0x20000},
	{"Back", 0x0020}, {"Start", 0x0010}, {"LS", 0x0040}, {"RS", 0x0080},
	{"Up", 0x0001}, {"Down", 0x0002}, {"Left", 0x0004}, {"Right", 0x0008},
}

const (
	PadLT = 0x10000
	PadRT = 0x20000
)

// ParsePadCombo reads "LB+Y" into a button mask; "" is no binding (0).
func ParsePadCombo(s string) (uint32, error) {
	var mask uint32
	for _, part := range strings.Split(s, "+") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		found := false
		for _, b := range padButtons {
			if strings.EqualFold(b.Name, part) {
				mask |= b.Bit
				found = true
			}
		}
		if !found {
			return 0, fmt.Errorf("không nhận ra nút tay cầm %q", part)
		}
	}
	return mask, nil
}

// PadComboName writes a button mask as "LB+Y", in a fixed order.
func PadComboName(mask uint32) string {
	var parts []string
	for _, b := range padButtons {
		if mask&b.Bit != 0 {
			parts = append(parts, b.Name)
		}
	}
	return strings.Join(parts, "+")
}

// PadCombo turns gamepad states into presses of one combination: it fires once
// when every button of the combination is down, and again only after one of
// them was let go.
type PadCombo struct {
	Mask uint32
	held bool
}

func (p *PadCombo) Feed(buttons uint32) bool {
	if p.Mask == 0 {
		return false
	}
	down := buttons&p.Mask == p.Mask
	fire := down && !p.held
	p.held = down
	return fire
}

// PadCapture learns a combination for the setup dialog: every button held at the
// same time, reported when they are all let go.
type PadCapture struct{ seen uint32 }

func (p *PadCapture) Feed(buttons uint32) (uint32, bool) {
	if buttons != 0 {
		p.seen |= buttons
		return 0, false
	}
	got := p.seen
	p.seen = 0
	return got, got != 0
}
