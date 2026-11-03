package main

// Death detection for games without a death screen: the health bar empties and
// then the screen fades to black before reloading the last checkpoint.
// Works for HUD bars and for diegetic bars that move with the character (Dead
// Space's glowing spine) by counting pixels of the bar's color inside a zone.

import (
	"image"
	"math"
	"sort"
	"time"
)

// HPColor describes which pixels belong to the health bar.
type HPColor struct {
	Hue    float64 `json:"hue"`     // degrees 0..360
	HueTol float64 `json:"hue_tol"` // +/- degrees
	SatMin float64 `json:"sat_min"` // 0..1
	ValMin float64 `json:"val_min"` // 0..1
}

func (c HPColor) Valid() bool { return c.SatMin > 0 && c.ValMin > 0 && c.HueTol > 0 }

func hsv(r, g, b uint8) (h, s, v float64) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	mx := math.Max(rf, math.Max(gf, bf))
	mn := math.Min(rf, math.Min(gf, bf))
	v = mx
	if mx <= 0 {
		return 0, 0, 0
	}
	s = (mx - mn) / mx
	d := mx - mn
	if d < 1e-9 {
		return 0, s, v
	}
	switch mx {
	case rf:
		h = math.Mod(60*((gf-bf)/d)+360, 360)
	case gf:
		h = 60*((bf-rf)/d) + 120
	default:
		h = 60*((rf-gf)/d) + 240
	}
	return h, s, v
}

func hueDist(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}

func zoneRect(b image.Rectangle, z ROI) image.Rectangle {
	z = z.Normalized()
	return image.Rect(
		b.Min.X+int(z.X*float64(b.Dx())), b.Min.Y+int(z.Y*float64(b.Dy())),
		b.Min.X+int((z.X+z.W)*float64(b.Dx())), b.Min.Y+int((z.Y+z.H)*float64(b.Dy())),
	).Intersect(b)
}

// LearnHPColor finds the dominant vivid color in the zone (taken at full health).
// Returns false when the zone has no vivid color to track.
func LearnHPColor(img *image.RGBA, zone ROI) (HPColor, bool) {
	r := zoneRect(img.Bounds(), zone)
	var bins [36]int
	type px struct{ h, s, v float64 }
	var vivid []px
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := img.PixOffset(x, y)
			h, s, v := hsv(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			if s > 0.5 && v > 0.45 {
				bins[int(h/10)%36]++
				vivid = append(vivid, px{h, s, v})
			}
		}
	}
	total := r.Dx() * r.Dy()
	if total == 0 || len(vivid) < 20 {
		return HPColor{}, false
	}
	best := 0
	for i := range bins { // peak over 3 neighbouring bins
		if bins[(i+35)%36]+bins[i]+bins[(i+1)%36] > bins[(best+35)%36]+bins[best]+bins[(best+1)%36] {
			best = i
		}
	}
	center := float64(best*10 + 5)
	var ss, vs []float64
	var hx, hy float64
	for _, p := range vivid {
		if hueDist(p.h, center) <= 20 {
			ss = append(ss, p.s)
			vs = append(vs, p.v)
			hx += math.Cos(p.h * math.Pi / 180)
			hy += math.Sin(p.h * math.Pi / 180)
		}
	}
	if len(ss) < 20 {
		return HPColor{}, false
	}
	sort.Float64s(ss)
	sort.Float64s(vs)
	hue := math.Mod(math.Atan2(hy, hx)*180/math.Pi+360, 360)
	// Thresholds below the typical bar pixel, but above dull scenery.
	return HPColor{
		Hue: hue, HueTol: 15,
		SatMin: math.Max(0.45, ss[len(ss)/2]*0.8),
		ValMin: math.Max(0.4, vs[len(vs)/2]*0.75),
	}, true
}

// MeasureHP returns the fraction of zone pixels that match the bar color.
func MeasureHP(img *image.RGBA, zone ROI, c HPColor) float64 {
	r := zoneRect(img.Bounds(), zone)
	total := r.Dx() * r.Dy()
	if total == 0 || !c.Valid() {
		return 0
	}
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := img.PixOffset(x, y)
			h, s, v := hsv(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			if s >= c.SatMin && v >= c.ValMin && hueDist(h, c.Hue) <= c.HueTol {
				n++
			}
		}
	}
	return float64(n) / float64(total)
}

// IsBlack: nearly the whole frame is (almost) black, as in a death fade.
func IsBlack(img *image.RGBA) bool {
	b := img.Bounds()
	dark, total := 0, 0
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			i := img.PixOffset(x, y)
			l := 0.299*float64(img.Pix[i]) + 0.587*float64(img.Pix[i+1]) + 0.114*float64(img.Pix[i+2])
			if l < 16 {
				dark++
			}
			total++
		}
	}
	return total > 0 && float64(dark)/float64(total) >= 0.97
}

// HPDeath turns health readings + black frames into death events.
type HPDeath struct {
	Full      float64       // calibrated full-health fraction (learned as running max)
	LowRatio  float64       // health <= this ratio counts as empty
	AliveRat  float64       // health >= this ratio means alive and visible
	Window    time.Duration // empty -> black must happen within this time
	BlackMin  time.Duration // black must last this long
	AliveWin  time.Duration // must have been alive within this time before emptying
	lastAlive time.Time
	lastEmpty time.Time
	blackFrom time.Time
	armed     bool
}

func NewHPDeath(full float64) *HPDeath {
	return &HPDeath{Full: full, LowRatio: 0.08, AliveRat: 0.25, Window: 8 * time.Second,
		BlackMin: time.Second, AliveWin: 20 * time.Second, armed: true}
}

// Ratio converts a raw measurement to 0..1 health and updates calibration.
func (d *HPDeath) Ratio(raw float64) float64 {
	if raw > d.Full {
		d.Full = raw
	}
	if d.Full < 0.0005 {
		return -1 // not calibrated / bar never seen
	}
	return math.Min(1, raw/d.Full)
}

// Feed returns true when a death should be counted.
func (d *HPDeath) Feed(ratio float64, black bool, now time.Time) bool {
	if black {
		if d.blackFrom.IsZero() {
			d.blackFrom = now
		}
	} else {
		d.blackFrom = time.Time{}
		if ratio >= d.AliveRat {
			d.lastAlive = now
			d.armed = true // respawned
		} else if ratio >= 0 && ratio <= d.LowRatio && !d.lastAlive.IsZero() && now.Sub(d.lastAlive) <= d.AliveWin {
			d.lastEmpty = now
		}
	}
	if d.armed && black && !d.lastEmpty.IsZero() && now.Sub(d.blackFrom) >= d.BlackMin &&
		d.blackFrom.Sub(d.lastEmpty) <= d.Window {
		d.armed = false
		d.lastEmpty = time.Time{}
		return true
	}
	return false
}
