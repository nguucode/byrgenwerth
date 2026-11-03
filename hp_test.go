package main

import (
	"image"
	"image/color"
	"math/rand"
	"testing"
	"time"
)

func fillAny(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	b := img.Bounds()
	for yy := y; yy < y+h && yy < b.Max.Y; yy++ {
		for xx := x; xx < x+w && xx < b.Max.X; xx++ {
			img.Set(xx, yy, c)
		}
	}
}

// spineScene: dull teal scenery with a vivid cyan "spine" of n lit segments (0..6).
func spineScene(r *rand.Rand, lit int, dx int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 640, 360))
	for y := 0; y < 360; y++ {
		for x := 0; x < 640; x++ {
			v := uint8(30 + r.Intn(50))
			img.Set(x, y, color.RGBA{v / 2, v, v, 255}) // desaturated teal fog
		}
	}
	// a bright but dull window light, should not count
	fillAny(img, 500, 40, 80, 60, color.RGBA{200, 230, 230, 255})
	for s := 0; s < lit; s++ {
		y := 230 - s*14
		fillAny(img, 120+dx, y, 10, 10, color.RGBA{40, 225, 180, 255})
	}
	return img
}

func TestHPColorDiegetic(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	zone := ROI{0.1, 0.3, 0.3, 0.5}
	full := spineScene(r, 6, 0)
	c, ok := LearnHPColor(full, zone)
	if !ok || hueDist(c.Hue, 165) > 10 {
		t.Fatalf("should learn the cyan spine color, got %+v ok=%v", c, ok)
	}
	f := MeasureHP(full, zone, c)
	if f <= 0 {
		t.Fatal("full spine must measure > 0")
	}
	half := MeasureHP(spineScene(r, 3, 15), zone, c) // character moved a bit
	none := MeasureHP(spineScene(r, 0, 0), zone, c)
	if half/f < 0.4 || half/f > 0.6 {
		t.Errorf("half health should be ~50%%, got %.2f", half/f)
	}
	if none/f > 0.05 {
		t.Errorf("dead spine should be ~0, got %.2f", none/f)
	}
}

func TestIsBlack(t *testing.T) {
	r := rand.New(rand.NewSource(8))
	if IsBlack(spineScene(r, 6, 0)) {
		t.Fatal("dark scene is not a black frame")
	}
	b := image.NewRGBA(image.Rect(0, 0, 320, 180))
	for i := 3; i < len(b.Pix); i += 4 {
		b.Pix[i] = 255
	}
	fill(b, 300, 170, 10, 6, color.RGBA{255, 255, 255, 255}) // small loading icon in a corner
	if !IsBlack(b) {
		t.Fatal("black frame with a tiny icon should count as black")
	}
}

func TestHPDeathSequence(t *testing.T) {
	d := NewHPDeath(0)
	t0 := time.Unix(0, 0)
	step := 500 * time.Millisecond
	now := t0
	feed := func(raw float64, black bool, n int) int {
		c := 0
		for i := 0; i < n; i++ {
			if d.Feed(d.Ratio(raw), black, now) {
				c++
			}
			now = now.Add(step)
		}
		return c
	}
	// Loading screen while healthy: black but health never emptied -> no death.
	if feed(0.02, false, 10)+feed(0, true, 6)+feed(0.02, false, 4) != 0 {
		t.Fatal("loading screen counted as death")
	}
	// Take damage, health empties, fade to black for 2s -> exactly one death.
	n := feed(0.01, false, 4) + feed(0.0, false, 3) + feed(0, true, 8)
	if n != 1 {
		t.Fatalf("want 1 death, got %d", n)
	}
	// Still black / reload: no second count; respawn with full health re-arms.
	n = feed(0, true, 6) + feed(0.02, false, 10)
	if n != 0 {
		t.Fatalf("double count during reload: %d", n)
	}
	// Bar never visible (menu) then black: no death.
	d2 := NewHPDeath(0.02)
	now2 := t0
	for i := 0; i < 20; i++ {
		if d2.Feed(0, i > 10, now2) {
			t.Fatal("no alive reading before, must not count")
		}
		now2 = now2.Add(step)
	}
	// Empty, but black comes 15s later (e.g. walked into a loading zone): no death.
	d3 := NewHPDeath(0.02)
	now3 := t0
	d3.Feed(1, false, now3)
	now3 = now3.Add(time.Second)
	d3.Feed(0, false, now3)
	now3 = now3.Add(15 * time.Second)
	for i := 0; i < 4; i++ {
		if d3.Feed(0, true, now3) {
			t.Fatal("black long after empty health must not count")
		}
		now3 = now3.Add(step)
	}
}

func TestTriggerSharesCooldownWithOCR(t *testing.T) {
	d := NewDetector(0.85, 10*time.Second)
	t0 := time.Unix(0, 0)
	// OCR counts the death screen...
	d.Feed(1, t0)
	if !d.Feed(1, t0.Add(500*time.Millisecond)) {
		t.Fatal("OCR should count")
	}
	// ...then the health-bar detector fires for the same death: ignored.
	if d.Trigger(t0.Add(3 * time.Second)) {
		t.Fatal("same death counted twice")
	}
	// A later death from the health bar alone counts.
	if !d.Trigger(t0.Add(40 * time.Second)) {
		t.Fatal("new death should count")
	}
	// And OCR right after that health-bar count is suppressed.
	d.Feed(1, t0.Add(41*time.Second))
	if d.Feed(1, t0.Add(41500*time.Millisecond)) {
		t.Fatal("OCR double count after health-bar trigger")
	}
}
