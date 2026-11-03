package main

import (
	"image"
	"image/color"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// scene renders a random "game world": gradients + random blobs.
func scene(r *rand.Rand) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, CaptureW, CaptureH))
	br, bg, bb := r.Intn(200), r.Intn(200), r.Intn(200)
	for y := 0; y < CaptureH; y++ {
		for x := 0; x < CaptureW; x++ {
			img.Set(x, y, color.RGBA{uint8((br + x/3) % 256), uint8((bg + y) % 256), uint8(bb), 255})
		}
	}
	for i := 0; i < 25; i++ {
		x, y, w, h := r.Intn(CaptureW), r.Intn(CaptureH), 10+r.Intn(60), 10+r.Intn(40)
		c := color.RGBA{uint8(r.Intn(256)), uint8(r.Intn(256)), uint8(r.Intn(256)), 255}
		fill(img, x, y, w, h, c)
	}
	return img
}

func fill(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	for yy := y; yy < y+h && yy < CaptureH; yy++ {
		for xx := x; xx < x+w && xx < CaptureW; xx++ {
			img.Set(xx, yy, c)
		}
	}
}

// deathOverlay darkens a horizontal band and draws blocky red "letters" in it,
// roughly like a Souls-like death banner over the live game world.
func deathOverlay(img *image.RGBA) {
	for y := 70; y < 110; y++ {
		for x := 0; x < CaptureW; x++ {
			c := img.RGBAAt(x, y)
			img.Set(x, y, color.RGBA{c.R / 6, c.G / 6, c.B / 6, 255})
		}
	}
	red := color.RGBA{170, 20, 20, 255}
	letters := []int{100, 115, 130, 150, 165, 180, 195}
	for i, x := range letters {
		fill(img, x, 78, 10, 4, red)
		fill(img, x, 78, 3, 24, red)
		if i%2 == 0 {
			fill(img, x+7, 78, 3, 24, red)
		}
		fill(img, x, 98, 10, 4, red)
	}
}

func TestDeathBannerDetectionWithROI(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	roi := ROI{0.3, 0.42, 0.33, 0.16} // tight box around the banner text
	tmplImg := scene(r)
	deathOverlay(tmplImg)
	tmpl := ExtractFeature(tmplImg, roi)

	minDeath, maxAlive := 1.0, 0.0
	for i := 0; i < 200; i++ {
		alive := scene(r)
		if s := Similarity(ExtractFeature(alive, roi), tmpl); s > maxAlive {
			maxAlive = s
		}
		dead := scene(r)
		deathOverlay(dead)
		if s := Similarity(ExtractFeature(dead, roi), tmpl); s < minDeath {
			minDeath = s
		}
	}
	t.Logf("ROI: min death sim=%.3f, max alive sim=%.3f", minDeath, maxAlive)
	if minDeath < 0.85 {
		t.Errorf("death frames should score >= 0.85, got min %.3f", minDeath)
	}
	if maxAlive > 0.6 {
		t.Errorf("alive frames should score low, got max %.3f", maxAlive)
	}
}

func TestFullFrameIsWeakerForOverlayBanners(t *testing.T) {
	// Documents why the ROI feature exists: on full frame, the changing
	// world dominates and separation collapses.
	r := rand.New(rand.NewSource(2))
	tmplImg := scene(r)
	deathOverlay(tmplImg)
	tmpl := ExtractFeature(tmplImg, FullROI())
	minDeath := 1.0
	for i := 0; i < 100; i++ {
		dead := scene(r)
		deathOverlay(dead)
		if s := Similarity(ExtractFeature(dead, FullROI()), tmpl); s < minDeath {
			minDeath = s
		}
	}
	t.Logf("full frame: min death sim=%.3f", minDeath)
	if minDeath > 0.6 {
		t.Errorf("expected full frame to be weak for overlays, got %.3f", minDeath)
	}
}

func TestTemplateAgreementFlagsLooseROI(t *testing.T) {
	r := rand.New(rand.NewSource(6))
	var tight, loose []Feature
	for i := 0; i < 3; i++ {
		img := scene(r)
		deathOverlay(img)
		tight = append(tight, ExtractFeature(img, ROI{0.3, 0.42, 0.33, 0.16}))
		loose = append(loose, ExtractFeature(img, FullROI()))
	}
	at, al := TemplateAgreement(tight), TemplateAgreement(loose)
	t.Logf("agreement tight=%.3f loose=%.3f", at, al)
	if at < 0.85 || al > 0.6 {
		t.Errorf("agreement should separate tight (%.3f) from loose (%.3f)", at, al)
	}
	if TemplateAgreement(tight[:1]) != -1 {
		t.Error("single template should return -1")
	}
}

func TestFullScreenDeathScreenFullROI(t *testing.T) {
	// Games whose death screen replaces the whole view (static screen):
	// full-frame matching works, including brightness shifts.
	r := rand.New(rand.NewSource(3))
	static := scene(r)
	tmpl := ExtractFeature(static, FullROI())
	brighter := image.NewRGBA(static.Bounds())
	for i := range static.Pix {
		v := int(static.Pix[i]) + 25
		if v > 255 {
			v = 255
		}
		brighter.Pix[i] = uint8(v)
	}
	if s := Similarity(ExtractFeature(brighter, FullROI()), tmpl); s < 0.95 {
		t.Errorf("brightness shift should keep sim high, got %.3f", s)
	}
	if s := Similarity(ExtractFeature(scene(r), FullROI()), tmpl); s > 0.6 {
		t.Errorf("different scene should be low, got %.3f", s)
	}
}

func TestFlatFrames(t *testing.T) {
	black := image.NewRGBA(image.Rect(0, 0, CaptureW, CaptureH))
	for i := 3; i < len(black.Pix); i += 4 {
		black.Pix[i] = 255
	}
	f := ExtractFeature(black, FullROI())
	if s := Similarity(f, f); s < 0.99 {
		t.Errorf("black vs black = %.3f", s)
	}
	r := rand.New(rand.NewSource(4))
	if s := Similarity(f, ExtractFeature(scene(r), FullROI())); s != 0 {
		t.Errorf("black vs scene = %.3f", s)
	}
}

func TestDetectorCountsOncePerDeathScreen(t *testing.T) {
	d := NewDetector(0.85, 10*time.Second)
	t0 := time.Unix(1000, 0)
	step := 400 * time.Millisecond
	counts := 0
	feed := func(sims ...float64) {
		for _, s := range sims {
			if d.Feed(s, t0) {
				counts++
			}
			t0 = t0.Add(step)
		}
	}
	feed(0.2, 0.3, 0.9) // only 1 frame above: not yet
	if counts != 0 {
		t.Fatal("needs 2 consecutive frames")
	}
	feed(0.92, 0.95, 0.95, 0.95, 0.95) // long death screen
	if counts != 1 {
		t.Fatalf("want 1 count, got %d", counts)
	}
	// Drops low briefly but cooldown not over: must not count again.
	feed(0.1, 0.1, 0.9, 0.9, 0.9)
	if counts != 1 {
		t.Fatalf("cooldown violated: %d", counts)
	}
	// Gameplay for >10s, then a new death.
	for i := 0; i < 30; i++ {
		feed(0.2)
	}
	feed(0.9, 0.9)
	if counts != 2 {
		t.Fatalf("want 2 counts, got %d", counts)
	}
	// Stays on death screen for 30s: still only one.
	for i := 0; i < 75; i++ {
		feed(0.93)
	}
	if counts != 2 {
		t.Fatalf("long screen double-counted: %d", counts)
	}
}

func TestDetectorFlickerAroundThreshold(t *testing.T) {
	d := NewDetector(0.85, 2*time.Second)
	t0 := time.Unix(0, 0)
	n := 0
	// Hovers just under/over threshold without dropping below rearm margin.
	sims := []float64{0.9, 0.9, 0.84, 0.86, 0.83, 0.88, 0.84, 0.9, 0.9, 0.82, 0.9, 0.9}
	for i := 0; i < 5; i++ {
		for _, s := range sims {
			if d.Feed(s, t0) {
				n++
			}
			t0 = t0.Add(time.Second)
		}
	}
	if n != 1 {
		t.Fatalf("flicker should count once, got %d", n)
	}
}

func TestDetectorManual(t *testing.T) {
	d := NewDetector(0.85, 10*time.Second)
	t0 := time.Unix(0, 0)
	d.MarkManual(t0)
	// Death screen already showing when user pressed hotkey: no auto double count.
	for i := 0; i < 10; i++ {
		if d.Feed(0.95, t0.Add(time.Duration(i)*400*time.Millisecond)) {
			t.Fatal("auto counted after manual")
		}
	}
	if d.AutoCountedRecently(t0) {
		t.Fatal("manual is not auto")
	}
	d2 := NewDetector(0.85, 10*time.Second)
	d2.Feed(0.9, t0)
	d2.Feed(0.9, t0.Add(time.Second))
	if !d2.AutoCountedRecently(t0.Add(3 * time.Second)) {
		t.Fatal("should report recent auto")
	}
	if d2.AutoCountedRecently(t0.Add(20 * time.Second)) {
		t.Fatal("should expire")
	}
}

func TestParseHotkey(t *testing.T) {
	cases := map[string][2]uint32{
		"F9":           {0, 0x78},
		"f10":          {0, 0x79},
		"F24":          {0, 0x87},
		"Ctrl+Shift+D": {ModControl | ModShift, 'D'},
		"alt + num5":   {ModAlt, 0x65},
		"Pause":        {0, 0x13},
		"Ctrl+Alt+0":   {ModControl | ModAlt, '0'},
		"NumAdd":       {0, 0x6B},
	}
	for in, want := range cases {
		m, k, err := ParseHotkey(in)
		if err != nil || m != want[0] || k != want[1] {
			t.Errorf("%q: got %x %x %v, want %x %x", in, m, k, err, want[0], want[1])
		}
	}
	for _, bad := range []string{"", "F0", "F25", "Hyper+F1", "Ctrl+", "F9x", "Foo"} {
		if _, _, err := ParseHotkey(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	p, err := s.CreateProfile("Elden Ring: Nightreign")
	if err != nil {
		t.Fatal(err)
	}
	p2, _ := s.CreateProfile("elden ring nightreign")
	if p.ID == p2.ID {
		t.Fatal("ids must be unique")
	}
	p.Deaths = 42
	p.ROI = ROI{0.2, 0.3, 0.5, 0.2}
	r := rand.New(rand.NewSource(5))
	now := time.Now()
	for i := 0; i < MaxTemplates+3; i++ {
		if _, err := s.AddTemplate(p, scene(r), now.Add(time.Duration(i)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.Templates) != MaxTemplates {
		t.Fatalf("templates capped at %d, got %d", MaxTemplates, len(p.Templates))
	}
	ents, _ := os.ReadDir(dir + "/profiles/" + p.ID + "/templates")
	if len(ents) != MaxTemplates {
		t.Fatalf("old template files should be deleted, have %d", len(ents))
	}
	if err := s.RemoveTemplate(p, 0); err != nil {
		t.Fatal(err)
	}
	ps, err := s.LoadProfiles()
	if err != nil || len(ps) != 2 {
		t.Fatalf("load: %v %d", err, len(ps))
	}
	var got *Profile
	for _, x := range ps {
		if x.ID == p.ID {
			got = x
		}
	}
	if got == nil || got.Deaths != 42 || len(got.Templates) != MaxTemplates-1 || got.ROI != p.ROI || got.Name != "Elden Ring: Nightreign" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	img, err := s.LoadTemplateImage(got, got.Templates[0])
	if err != nil || img.Bounds().Dx() != CaptureW {
		t.Fatalf("template load: %v", err)
	}

	c := DefaultConfig()
	c.CurrentProfile = p.ID
	c.Hotkeys.Add = "Ctrl+F9"
	s.SaveConfig(c)
	c2 := s.LoadConfig()
	if c2.CurrentProfile != p.ID || c2.Hotkeys.Add != "Ctrl+F9" || c2.Hotkeys.Sub != "F10" {
		t.Fatalf("config roundtrip: %+v", c2)
	}

	if err := s.WriteObs("{game} deaths: {count}", got.Name, 7); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(s.ObsPath())
	if string(b) != "Elden Ring: Nightreign deaths: 7" {
		t.Fatalf("obs: %q", b)
	}
	s.AppendHistory(now, got.Name, 1, 43, "auto")
	s.AppendHistory(now, got.Name, -1, 42, "manual")
	h, _ := os.ReadFile(s.HistoryPath())
	if lines := strings.Count(string(h), "\n"); lines != 3 {
		t.Fatalf("history lines %d: %s", lines, h)
	}
	if err := s.DeleteProfile(got); err != nil {
		t.Fatal(err)
	}
	ps, _ = s.LoadProfiles()
	if len(ps) != 1 {
		t.Fatal("delete failed")
	}
}

func TestROINormalize(t *testing.T) {
	if !(ROI{}).IsFull() {
		t.Fatal("zero ROI should mean full")
	}
	n := ROI{0.9, 0.9, 0.5, 0.5}.Normalized()
	if n.X+n.W > 1.0001 || n.Y+n.H > 1.0001 {
		t.Fatalf("not clamped: %+v", n)
	}
}

func TestComposeOverlayPremultiplied(t *testing.T) {
	w, h := 4, 1
	fill := []uint8{255, 0, 128, 0}
	outline := []uint8{255, 255, 255, 0}
	px := ComposeOverlay(fill, outline, w, h, RGB{200, 100, 50}, RGB{0, 0, 0}, false)
	// pixel 0: full text
	if px[2] != 200 || px[1] != 100 || px[0] != 50 || px[3] != 255 {
		t.Errorf("px0 %v", px[0:4])
	}
	// pixel 1: outline only -> black, opaque
	if px[4] != 0 || px[5] != 0 || px[6] != 0 || px[7] != 255 {
		t.Errorf("px1 %v", px[4:8])
	}
	// pixel 3: nothing -> fully transparent (click-through)
	if px[12] != 0 || px[13] != 0 || px[14] != 0 || px[15] != 0 {
		t.Errorf("px3 %v", px[12:16])
	}
	// premultiplied invariant: every channel <= alpha
	for i := 0; i < len(px); i += 4 {
		if px[i] > px[i+3] || px[i+1] > px[i+3] || px[i+2] > px[i+3] {
			t.Errorf("not premultiplied at %d: %v", i/4, px[i:i+4])
		}
	}
	// edit mode: empty pixels become visible (grabbable)
	pe := ComposeOverlay(make([]uint8, 25), make([]uint8, 25), 5, 5, RGB{255, 255, 255}, RGB{}, true)
	for i := 0; i < len(pe); i += 4 {
		if pe[i+3] == 0 {
			t.Fatal("edit mode must have no fully transparent pixels")
		}
		if pe[i] > pe[i+3] || pe[i+1] > pe[i+3] || pe[i+2] > pe[i+3] {
			t.Fatalf("edit not premultiplied: %v", pe[i:i+4])
		}
	}
}

func TestOutlineOffsets(t *testing.T) {
	o := OutlineOffsets(2)
	if len(o) < 9 || len(o) > 25 {
		t.Fatalf("unexpected offsets %d", len(o))
	}
	if len(OutlineOffsets(0)) != 1 {
		t.Fatal("r=0 should be a single stamp")
	}
}

func TestClampToMonitors(t *testing.T) {
	rects := [][4]int32{{0, 0, 1920, 1080}, {-1280, 0, 0, 1024}}
	if x, y, ok := ClampToMonitors(100, 100, rects); !ok || x != 100 || y != 100 {
		t.Error("inside primary")
	}
	if _, _, ok := ClampToMonitors(-600, 500, rects); !ok {
		t.Error("inside secondary (negative coords)")
	}
	if x, y, ok := ClampToMonitors(5000, 5000, rects); ok || x != 50 || y != 50 {
		t.Error("off-screen should reset")
	}
}

func TestConfigDefaultsForNewFields(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	// Old config file without new fields, plus an invalid color.
	os.WriteFile(s.Dir+"/config.json", []byte(`{"obs_format":"x","overlay_color":"Tím","sound_volume":500}`), 0o644)
	c := s.LoadConfig()
	if c.OverlayColor != "Trắng" || c.SoundVolume != 80 || c.OverlayFontSize != 40 || !c.ObsEnabled {
		t.Fatalf("defaults not applied: %+v", c)
	}
	for _, n := range OverlayColorNames {
		if _, ok := OverlayColors[n]; !ok {
			t.Fatalf("color %q missing", n)
		}
	}
}

func line(text string, x, y, h float64) OcrLine {
	var ws []OcrWord
	cx := x
	for _, w := range strings.Fields(text) {
		ww := float64(len(w)) * h * 0.6
		ws = append(ws, OcrWord{Text: w, X: cx, Y: y, W: ww, H: h})
		cx += ww + h*0.3
	}
	return OcrLine{Text: text, Words: ws}
}

func TestMatchDeathPositive(t *testing.T) {
	kw := DefaultKeywords
	cases := []OcrLine{
		line("DEAD", 0.45, 0.78, 0.046),   // the user's game: "DEAD" under calligraphy
		line("YOU DIED", 0.4, 0.45, 0.06), // Souls-like
		line("Y0U DlED", 0.4, 0.45, 0.06), // OCR confusions
		line("GAME OVER", 0.38, 0.4, 0.05),
		line("Wasted", 0.42, 0.5, 0.08),
		line("YOU DIED.", 0.4, 0.45, 0.06),
		line("- DEFEATED -", 0.4, 0.45, 0.05),
	}
	for _, c := range cases {
		if m, ok := MatchDeath([]OcrLine{c}, kw, 0.025); !ok {
			t.Errorf("should match %q", c.Text)
		} else if m.Box.W <= 0 || m.Height <= 0 {
			t.Errorf("bad box for %q: %+v", c.Text, m)
		}
	}
}

func TestMatchDeathNegative(t *testing.T) {
	kw := DefaultKeywords
	cases := []OcrLine{
		line("DEAD", 0.45, 0.5, 0.015),                         // too small (menu text)
		line("DEAD", 0.02, 0.05, 0.05),                         // top-left corner (e.g. our own overlay)
		line("READ", 0.45, 0.5, 0.06),                          // one letter off a short keyword
		line("DEAD MAN'S CHEST IS HERE TODAY", 0.2, 0.5, 0.05), // sentence, not a banner
		line("CONTINUE", 0.45, 0.5, 0.05),
		line("LOADING", 0.45, 0.5, 0.05),
	}
	for _, c := range cases {
		if m, ok := MatchDeath([]OcrLine{c}, kw, 0.025); ok {
			t.Errorf("should NOT match %q (got %+v)", c.Text, m)
		}
	}
}

func TestMatchDeathPicksLargest(t *testing.T) {
	ls := []OcrLine{line("DIED", 0.45, 0.3, 0.03), line("YOU DIED", 0.4, 0.5, 0.07)}
	m, ok := MatchDeath(ls, DefaultKeywords, 0.025)
	if !ok || m.Height != 0.07 {
		t.Fatalf("want the big banner, got %+v", m)
	}
}

func TestOverlayTextDoesNotTrigger(t *testing.T) {
	// Our overlay "Deaths: 12" in a corner must never count.
	if _, ok := MatchDeath([]OcrLine{line("Deaths: 12", 0.03, 0.03, 0.04)}, DefaultKeywords, 0.025); ok {
		t.Fatal("overlay text triggered")
	}
	// Even centered, "DEATHS 12" is not "DEATH" as a lone banner? It is 2 words for a 1-word keyword: allowed (<= +2).
	// Guard: DEATHS != DEATH exactly for a 5-letter single keyword.
	if _, ok := MatchDeath([]OcrLine{line("Deaths: 12", 0.45, 0.5, 0.06)}, DefaultKeywords, 0.025); ok {
		t.Fatal("centered overlay text triggered")
	}
}

func TestNormalizeAndParse(t *testing.T) {
	if NormalizeText("  You   Died!! ") != "YOU DIED" {
		t.Fatal(NormalizeText("  You   Died!! "))
	}
	k := ParseKeywords("You died, DEAD,, you died ; Game Over\nWasted")
	if len(k) != 4 || k[0] != "You died" || k[3] != "Wasted" {
		t.Fatalf("%q", k)
	}
}

func TestAutoROI(t *testing.T) {
	r := AutoROI(ROI{0.45, 0.78, 0.1, 0.05})
	if r.X >= 0.45 || r.Y >= 0.78 || r.X+r.W <= 0.55 || r.Y+r.H <= 0.83 || r.IsFull() {
		t.Fatalf("roi %+v", r)
	}
}

func TestDownscale(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i], src.Pix[i+3] = 200, 255
	}
	d := Downscale(src, CaptureW, CaptureH)
	if d.Bounds().Dx() != CaptureW || d.Pix[0] != 200 || d.Pix[3] != 255 {
		t.Fatal("downscale wrong")
	}
}

func TestMigrateLegacyData(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	old := &Store{Dir: src}
	p, _ := old.CreateProfile("Wuchang")
	p.Deaths = 9
	old.SaveProfile(p)
	old.AddTemplate(p, image.NewRGBA(image.Rect(0, 0, CaptureW, CaptureH)), time.Now())
	old.SaveConfig(DefaultConfig())
	ok, err := MigrateLegacyData(src, dst)
	if !ok || err != nil {
		t.Fatalf("migrate: %v %v", ok, err)
	}
	ps, _ := (&Store{Dir: dst}).LoadProfiles()
	if len(ps) != 1 || ps[0].Deaths != 9 || len(ps[0].Templates) != 1 {
		t.Fatalf("migrated data wrong: %+v", ps)
	}
	if _, err := os.Stat(filepath.Join(src, "profiles")); err != nil {
		t.Fatal("source must be left intact")
	}
	// Second run must not overwrite newer data in dst.
	ps[0].Deaths = 20
	(&Store{Dir: dst}).SaveProfile(ps[0])
	if ok, _ := MigrateLegacyData(src, dst); ok {
		t.Fatal("must not migrate twice")
	}
	ps, _ = (&Store{Dir: dst}).LoadProfiles()
	if ps[0].Deaths != 20 {
		t.Fatal("dst overwritten")
	}
}

func TestComposeOverlayOutlineColor(t *testing.T) {
	px := ComposeOverlay([]uint8{0, 128}, []uint8{255, 255}, 2, 1, RGB{255, 255, 255}, RGB{255, 0, 0}, false)
	if px[2] != 255 || px[1] != 0 || px[0] != 0 || px[3] != 255 {
		t.Fatalf("outline pixel should be red: %v", px[0:4])
	}
	// half text over red outline: R stays 255, G/B ~128
	if px[6] != 255 || px[5] < 126 || px[5] > 130 {
		t.Fatalf("blend wrong: %v", px[4:8])
	}
}

func TestHexColors(t *testing.T) {
	c, err := ParseHex("#ff8000")
	if err != nil || c != (RGB{255, 128, 0}) || c.Hex() != "#FF8000" {
		t.Fatal(c, err)
	}
	if c, _ := ParseHex("0f0"); c != (RGB{0, 255, 0}) {
		t.Fatal(c)
	}
	for _, bad := range []string{"", "#12345", "zzzzzz", "#1234567"} {
		if _, err := ParseHex(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestConfigMigratesNamedColor(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	os.WriteFile(s.Dir+"/config.json", []byte(`{"overlay_color":"Vàng"}`), 0o644)
	c := s.LoadConfig()
	if c.TextColor != "#FFD228" || c.FontFamily != "Segoe UI" || !c.Outline || c.ServerPort != 17321 {
		t.Fatalf("%+v", c)
	}
}

func TestPrefixKeywords(t *testing.T) {
	kw := []string{"You were slain by *"}
	if _, ok := MatchDeath([]OcrLine{line("You were slain by Eye of Cthulhu", 0.3, 0.45, 0.04)}, kw, 0.025); !ok {
		t.Error("prefix keyword should match a variable tail")
	}
	if _, ok := MatchDeath([]OcrLine{line("Then you were slain by a bat", 0.3, 0.45, 0.04)}, kw, 0.025); ok {
		t.Error("prefix keyword must start the line")
	}
	if _, ok := MatchDeath([]OcrLine{line("You were slain by a b c d e f g h i j", 0.2, 0.45, 0.04)}, kw, 0.025); ok {
		t.Error("tail longer than 8 words is a sentence, not a banner")
	}
	// Non-prefix keywords keep the strict rule.
	if _, ok := MatchDeath([]OcrLine{line("DEAD", 0.45, 0.5, 0.05)}, []string{"DEAD"}, 0.025); !ok {
		t.Error("plain keyword broke")
	}
}

func TestEffectiveKeywords(t *testing.T) {
	g := []string{"YOU DIED"}
	if k := EffectiveKeywords(&Profile{}, g); k[0] != "YOU DIED" {
		t.Fatal("should fall back to global")
	}
	if k := EffectiveKeywords(&Profile{Keywords: []string{"WASTED"}}, g); k[0] != "WASTED" {
		t.Fatal("profile keywords should win")
	}
}

func TestCustomDataDir(t *testing.T) {
	def, custom := t.TempDir(), filepath.Join(t.TempDir(), "My Data")
	if d, w := ResolveDataDir(def); d != def || w != "" {
		t.Fatal("no pointer -> default")
	}
	// Put some data in the default folder, then move to the custom folder.
	old := &Store{Dir: def}
	p, _ := old.CreateProfile("Elden Ring")
	p.Deaths = 5
	old.SaveProfile(p)
	if err := CheckWritable(custom); err != nil {
		t.Fatal(err)
	}
	if DirHasData(custom) {
		t.Fatal("new folder should be empty")
	}
	if err := CopyDataDir(def, custom); err != nil {
		t.Fatal(err)
	}
	if err := SetDataDir(def, custom); err != nil {
		t.Fatal(err)
	}
	d, w := ResolveDataDir(def)
	if d != filepath.Clean(custom) || w != "" {
		t.Fatalf("resolve: %q %q", d, w)
	}
	ps, _ := (&Store{Dir: d}).LoadProfiles()
	if len(ps) != 1 || ps[0].Deaths != 5 {
		t.Fatal("data not copied")
	}
	if _, err := os.Stat(filepath.Join(custom, dataPointerFile)); err == nil {
		t.Fatal("pointer file must not be copied into the data folder")
	}
	if !DirHasData(custom) {
		t.Fatal("DirHasData")
	}
	// Folder disappears (USB unplugged): fall back with a warning.
	os.RemoveAll(custom)
	os.WriteFile(filepath.Join(custom), []byte("now a file, not a folder"), 0o644)
	if d, w := ResolveDataDir(def); d != def || w == "" {
		t.Fatalf("unreachable folder should fall back with warning: %q %q", d, w)
	}
	// Back to default removes the pointer.
	if err := SetDataDir(def, ""); err != nil {
		t.Fatal(err)
	}
	if d, w := ResolveDataDir(def); d != def || w != "" {
		t.Fatal("reset to default failed")
	}
	if err := CheckWritable(""); err == nil {
		t.Fatal("empty path must be rejected")
	}
}
