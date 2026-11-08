package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestLabelFromFormat(t *testing.T) {
	cases := map[string]string{
		"Deaths: {count}":          "Deaths",
		"{game} · Deaths: {count}": "{game} · Deaths",
		"{count}":                  "Deaths",
		"Chết {count} lần":         "Chết  lần",
	}
	for in, want := range cases {
		if got := LabelFromFormat(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestOldConfigKeepsItsLabelWord(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"obs_format":"Tử vong: {count}","label_corner":"zz","narrate_pad":"LB+Nope"}`), 0o644)
	c := (&Store{Dir: dir}).LoadConfig()
	if c.LabelText != "Tử vong" || c.LabelMode != LabelModeText || c.LabelCorner != CornerTL || c.NarratePad != "" {
		t.Fatalf("%+v", c)
	}
	if c.Hotkeys.Narrate != "Ctrl+Alt+V" || c.NarrateVoice != VoiceFemale {
		t.Fatalf("narration defaults: %+v %s", c.Hotkeys, c.NarrateVoice)
	}
	// A picture label without its file falls back to the word.
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"label_mode":"image","label_text":"X"}`), 0o644)
	if c := (&Store{Dir: dir}).LoadConfig(); c.LabelMode != LabelModeText || c.LabelText != "X" {
		t.Fatalf("%+v", c)
	}
}

func TestNewProfileDefaults(t *testing.T) {
	p := NewProfile("x")
	if p.Threshold != 0.70 || p.CooldownSec != 10 {
		t.Fatalf("%v %v", p.Threshold, p.CooldownSec)
	}
}

func TestTextFileLine(t *testing.T) {
	c := DefaultConfig()
	c.LabelText = "{game} deaths"
	if got := TextFileLine(c, "Hades", 4); got != "Hades deaths 4" {
		t.Fatal(got)
	}
	c.LabelMode = LabelModeImage
	if got := TextFileLine(c, "Hades", 4); got != "4" {
		t.Fatal(got)
	}
}

func TestLabelLayoutCorners(t *testing.T) {
	// count 100x50, label 40x20, gap 5
	w, h, cnt, lab := LabelLayout(100, 50, 40, 20, CornerTL, 5)
	if w != 100 || h != 75 || cnt != (Box{0, 25, 100, 50}) || lab != (Box{0, 0, 40, 20}) {
		t.Fatal("tl", w, h, cnt, lab)
	}
	_, _, cnt, lab = LabelLayout(100, 50, 40, 20, CornerTR, 5)
	if cnt.X != 0 || lab.X != 60 || lab.Y != 0 {
		t.Fatal("tr", cnt, lab)
	}
	_, _, cnt, lab = LabelLayout(100, 50, 40, 20, CornerBL, 5)
	if cnt.Y != 0 || lab.Y != 55 || lab.X != 0 {
		t.Fatal("bl", cnt, lab)
	}
	// A label wider than the count: the count moves to the right edge.
	w, _, cnt, lab = LabelLayout(30, 50, 80, 20, CornerBR, 5)
	if w != 80 || cnt.X != 50 || lab.X != 0 {
		t.Fatal("br", w, cnt, lab)
	}
	w, h, cnt, _ = LabelLayout(30, 50, 0, 0, CornerBR, 5)
	if w != 30 || h != 50 || cnt != (Box{0, 0, 30, 50}) {
		t.Fatal("no label", w, h, cnt)
	}
}

func TestBlitPremulOver(t *testing.T) {
	dst := []byte{0, 0, 255, 255} // opaque red (BGRA)
	src := []byte{128, 0, 0, 128} // half-transparent blue, premultiplied
	BlitPremul(dst, 1, 1, src, 1, 1, 0, 0)
	if dst[0] != 128 || dst[2] != 127 || dst[3] != 255 {
		t.Fatal(dst)
	}
	// Out of bounds is clipped, not a panic.
	BlitPremul(dst, 1, 1, src, 1, 1, 5, -3)
}

func TestLoadLabelImageScalesToHeight(t *testing.T) {
	dir := t.TempDir()
	svg := filepath.Join(dir, "a.svg")
	os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 10"><rect width="20" height="10" fill="#ff0000"/></svg>`), 0o644)
	img, err := LoadLabelImage(svg, 30)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 60 || b.Dy() != 30 {
		t.Fatal(b)
	}
	if r, _, _, a := img.At(30, 15).RGBA(); r>>8 < 200 || a>>8 < 200 {
		t.Fatal("svg not drawn", r, a)
	}

	src := image.NewRGBA(image.Rect(0, 0, 10, 40))
	for i := range src.Pix {
		src.Pix[i] = 255
	}
	src.Set(0, 0, color.RGBA{})
	p := filepath.Join(dir, "a.png")
	f, _ := os.Create(p)
	png.Encode(f, src)
	f.Close()
	img, err = LoadLabelImage(p, 20)
	if err != nil || img.Bounds().Dx() != 5 || img.Bounds().Dy() != 20 {
		t.Fatal(err, img.Bounds())
	}
	os.WriteFile(filepath.Join(dir, "x.gif"), []byte("GIF89a"), 0o644)
	if _, err := LoadLabelImage(filepath.Join(dir, "x.gif"), 20); err == nil {
		t.Fatal("gif must be refused")
	}
}

func TestKeyNamesRoundTrip(t *testing.T) {
	for _, s := range []string{"Ctrl+Alt+V", "F10", "Shift+Num5", "Alt+PageDown", "Ctrl+9"} {
		mods, vk, err := ParseHotkey(s)
		if err != nil {
			t.Fatal(s, err)
		}
		got := HotkeyText(mods&ModControl != 0, mods&ModAlt != 0, mods&ModShift != 0, vk)
		if got != s {
			t.Errorf("%s → %s", s, got)
		}
	}
	if HotkeyText(true, false, false, 0x10) != "" {
		t.Fatal("a modifier alone is not a hotkey")
	}
}

func TestPadCombo(t *testing.T) {
	mask, err := ParsePadCombo("lb + RB+y")
	if err != nil || PadComboName(mask) != "Y+LB+RB" {
		t.Fatal(PadComboName(mask), err)
	}
	if _, err := ParsePadCombo("LB+Z"); err == nil {
		t.Fatal("unknown button")
	}
	if m, err := ParsePadCombo(""); m != 0 || err != nil {
		t.Fatal("empty = none")
	}
	p := PadCombo{Mask: mask}
	lb, rb, y, a := uint32(0x0100), uint32(0x0200), uint32(0x8000), uint32(0x1000)
	steps := []struct {
		buttons uint32
		fire    bool
	}{
		{lb, false}, {lb | rb, false}, {lb | rb | y, true}, {lb | rb | y | a, false}, // held: once
		{lb | rb, false}, {lb | rb | y, true}, {0, false},
	}
	for i, s := range steps {
		if got := p.Feed(s.buttons); got != s.fire {
			t.Fatalf("step %d: %v", i, got)
		}
	}

	var c PadCapture
	for _, b := range []uint32{lb, lb | PadRT, PadRT} {
		if _, ok := c.Feed(b); ok {
			t.Fatal("reported before release")
		}
	}
	if got, ok := c.Feed(0); !ok || PadComboName(got) != "LB+RT" {
		t.Fatal(PadComboName(got), ok)
	}
	if _, ok := c.Feed(0); ok {
		t.Fatal("nothing pressed since")
	}
}
