package main

import (
	"strings"
	"testing"
	"time"
)

func TestBossStatsFuzzyMerge(t *testing.T) {
	p := &Profile{}
	AddBossDeath(p, "Margit, the Fell Omen", 1)
	AddBossDeath(p, "Margit the Fell 0men", 1) // OCR noise
	AddBossDeath(p, "Godrick the Grafted", 1)
	if len(p.Bosses) != 2 || p.Bosses[0].Deaths != 2 || p.Bosses[0].Name != "Margit, the Fell Omen" {
		t.Fatalf("%+v", p.Bosses)
	}
	AddBossDeath(p, "Godrick the Grafted", -1)
	if len(p.Bosses) != 1 {
		t.Fatal("boss at 0 deaths should be removed")
	}
	// Short distinct names must not merge.
	q := &Profile{}
	AddBossDeath(q, "Ifrit", 1)
	AddBossDeath(q, "Ifrir", 1)
	if len(q.Bosses) != 2 {
		t.Fatal("short names need exact match")
	}
}

func TestSortedRenameMerge(t *testing.T) {
	p := &Profile{Bosses: []BossStat{{"B", 3}, {"A", 3}, {"C", 9}, {"Malenia", 1}, {"Malenia, Blade", 2}}}
	s := SortedBosses(p)
	if s[0].Name != "C" || s[1].Name != "A" || s[2].Name != "B" {
		t.Fatalf("%+v", s)
	}
	MergeBoss(p, "Malenia", "Malenia, Blade")
	RenameBoss(p, "Malenia, Blade", "Malenia, Blade of Miquella")
	found := false
	for _, b := range p.Bosses {
		if b.Name == "Malenia, Blade of Miquella" && b.Deaths == 3 {
			found = true
		}
		if b.Name == "Malenia" {
			t.Fatal("merged boss should be gone")
		}
	}
	if !found {
		t.Fatalf("%+v", p.Bosses)
	}
	RemoveBoss(p, "C")
	if len(p.Bosses) != 3 {
		t.Fatal("remove failed")
	}
}

func TestBossCandidates(t *testing.T) {
	lines := []OcrLine{
		line("Margit, the Fell Omen", 0.25, 0.80, 0.025), // Souls-like boss bar, bottom
		line("Gwyn, Lord of Cinder", 0.35, 0.05, 0.03),   // top-center bar
		line("YOU DIED", 0.4, 0.45, 0.06),                // death banner: not a boss
		line("HP x3", 0.01, 0.9, 0.02),                   // corner HUD
		line("12", 0.5, 0.85, 0.03),                      // numbers only
		line("tiny", 0.45, 0.8, 0.005),                   // too small
	}
	c := BossCandidates(lines, DefaultBossZones, DefaultKeywords)
	if len(c) != 2 || c[0] != "Margit, the Fell Omen" || c[1] != "Gwyn, Lord of Cinder" {
		t.Fatalf("%q", c)
	}
	// Custom zone overrides defaults.
	c = BossCandidates(lines, []ROI{{0, 0, 1, 0.2}}, DefaultKeywords)
	if len(c) != 1 || c[0] != "Gwyn, Lord of Cinder" {
		t.Fatalf("custom zone: %q", c)
	}
}

func TestBossTrackerIgnoresSubtitles(t *testing.T) {
	tr := NewBossTracker()
	now := time.Unix(0, 0)
	step := 600 * time.Millisecond
	// A subtitle shows for ~3s (5 frames), then another subtitle.
	for i := 0; i < 5; i++ {
		tr.Observe([]string{"I will not let you pass"}, now)
		now = now.Add(step)
	}
	if _, ok := tr.Current(now); ok {
		t.Fatal("subtitle must not become a boss")
	}
	// Boss bar stays during the fight.
	for i := 0; i < 20; i++ {
		tr.Observe([]string{"Margit, the Fell Omen"}, now)
		now = now.Add(step)
	}
	if b, ok := tr.Current(now); !ok || b != "Margit, the Fell Omen" {
		t.Fatalf("boss not detected: %q", b)
	}
	// The bar disappears on the death screen; death 10s later still attributes.
	if b, ok := tr.Current(now.Add(10 * time.Second)); !ok || b != "Margit, the Fell Omen" {
		t.Fatal("recent boss should be kept for the death")
	}
	// Long after, no boss.
	if _, ok := tr.Current(now.Add(2 * time.Minute)); ok {
		t.Fatal("boss should expire")
	}
}

func TestBossTrackerIgnoresAlwaysOnHUD(t *testing.T) {
	tr := NewBossTracker()
	now := time.Unix(0, 0)
	for i := 0; i < 2000; i++ { // ~17 minutes of a label that never leaves
		tr.Observe([]string{"Player One"}, now)
		now = now.Add(500 * time.Millisecond)
	}
	if b, ok := tr.Current(now); ok {
		t.Fatalf("always-on HUD label taken as boss: %q", b)
	}
	// A boss that appears, the player dies (bar gone), and it appears again: still a boss.
	tr2 := NewBossTracker()
	for round := 0; round < 3; round++ {
		for i := 0; i < 240; i++ { // 2 min fight
			tr2.Observe([]string{"Malenia"}, now)
			now = now.Add(500 * time.Millisecond)
		}
		now = now.Add(20 * time.Second) // death + run back
	}
	tr2.Observe([]string{"Malenia"}, now)
	if b, ok := tr2.Current(now); !ok || b != "Malenia" {
		t.Fatal("repeated fights must still count as a boss")
	}
}

func TestBossTrackerSwitchesToNewestBoss(t *testing.T) {
	tr := NewBossTracker()
	now := time.Unix(0, 0)
	for i := 0; i < 30; i++ {
		tr.Observe([]string{"Boss One"}, now)
		now = now.Add(500 * time.Millisecond)
	}
	for i := 0; i < 12; i++ {
		tr.Observe([]string{"Second Boss"}, now)
		now = now.Add(500 * time.Millisecond)
	}
	if b, _ := tr.Current(now); b != "Second Boss" {
		t.Fatalf("want newest boss, got %q", b)
	}
}

func TestBuildSplits(t *testing.T) {
	p := &Profile{Bosses: []BossStat{{"A", 9}, {"B", 5}, {"C", 3}, {"D", 1}}}
	r := BuildSplits(p, "B", 3)
	if len(r) != 3 || !r[1].Active || r[0].Rank != 1 {
		t.Fatalf("%+v", r)
	}
	r = BuildSplits(p, "D", 3)
	if len(r) != 4 || !r[3].Active || r[3].Rank != 4 || r[3].Deaths != 1 {
		t.Fatalf("current boss outside top3 should be appended: %+v", r)
	}
	r = BuildSplits(p, "New Boss", 3)
	if len(r) != 4 || r[3].Deaths != 0 || r[3].Rank != 0 {
		t.Fatalf("unseen boss row: %+v", r)
	}
}

func TestCJKKeywordAndNormalize(t *testing.T) {
	if NormalizeText("Lời nguyền") != "LỜI NGUYỀN" {
		t.Fatal(NormalizeText("Lời nguyền"))
	}
	// Chinese OCR returns characters as separate words.
	l := OcrLine{Text: "殒 命", Words: []OcrWord{{Text: "殒", X: 0.45, Y: 0.4, W: 0.05, H: 0.08}, {Text: "命", X: 0.5, Y: 0.4, W: 0.05, H: 0.08}}}
	if _, ok := MatchDeath([]OcrLine{l}, []string{"殒命"}, 0.025); !ok {
		t.Fatal("CJK keyword should match across split characters")
	}
	long := OcrLine{Text: "这是一段很长的对话殒命之后继续", Words: []OcrWord{{Text: "这是一段很长的对话殒命之后继续", X: 0.3, Y: 0.4, W: 0.4, H: 0.05}}}
	if _, ok := MatchDeath([]OcrLine{long}, []string{"殒命"}, 0.025); ok {
		t.Fatal("CJK keyword inside a long sentence must not match")
	}
}

func fakeMeasure(text string, px int, bold bool) (int, int) {
	w := len([]rune(text)) * px * 55 / 100
	if bold {
		w = w * 11 / 10
	}
	return w, px * 13 / 10
}

func TestLayoutSplits(t *testing.T) {
	p := &Profile{Bosses: []BossStat{{"Margit, the Fell Omen", 23}, {"Godrick the Grafted", 11}, {"Rennala", 7}, {"Starscourge Radahn", 4}}}
	st := SplitsState{Title: "Elden Ring", Rows: BuildSplits(p, "Starscourge Radahn", 3), Total: 58}
	W, H, rules, texts := LayoutSplits(st, 20, fakeMeasure)
	if len(rules) != 2 {
		t.Fatalf("want 2 rules (under the title and under Deaths), got %d", len(rules))
	}
	for _, rc := range rules {
		if rc.X < 0 || rc.Y < 0 || rc.X+rc.W > W || rc.Y+rc.H > H {
			t.Fatalf("rule outside panel: %+v (%dx%d)", rc, W, H)
		}
	}
	want := map[string]bool{"Elden Ring": false, "Deaths": false, "58": false, "▶ Starscourge Radahn": false, "23": false}
	for _, tx := range texts {
		tw, th := fakeMeasure(tx.Text, tx.Px, tx.Bold)
		x0 := tx.X
		if tx.Right {
			x0 = tx.X - tw
		}
		if tx.MaxW == 0 && (x0 < 0 || x0+tw > W) || tx.Y < 0 || tx.Y+th > H {
			t.Fatalf("text outside panel: %+v", tx)
		}
		if _, ok := want[tx.Text]; ok {
			want[tx.Text] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("missing %q", k)
		}
	}
	// Very long names are cut to one line so the panel stays compact.
	p.Bosses[0].Name = strings.Repeat("Very Long Boss Name ", 10)
	W2, _, _, tx2 := LayoutSplits(SplitsState{Rows: BuildSplits(p, "", 3)}, 20, fakeMeasure)
	if W2 > 20*20 {
		t.Fatalf("panel too wide: %d", W2)
	}
	for _, x := range tx2 {
		if strings.HasPrefix(x.Text, "Very Long") && (x.MaxW <= 0 || x.MaxW >= W2) {
			t.Fatalf("long name must be capped: %+v", x)
		}
	}
	// No bosses yet: title, rule, Deaths only.
	_, _, r3, tx3 := LayoutSplits(SplitsState{Total: 3}, 18, fakeMeasure)
	if len(r3) != 1 || len(tx3) != 3 {
		t.Fatalf("empty panel: %d rules, %d texts", len(r3), len(tx3))
	}
}

func TestSekiroTopLeftBossName(t *testing.T) {
	// Sekiro: short boss name at the far top-left, above the boss vitality bar.
	sek := line("Genichiro Ashina", 0.02, 0.04, 0.025)
	hud := line("x 3", 0.03, 0.85, 0.02) // item count bottom-left
	if c := BossCandidates([]OcrLine{sek, hud}, DefaultBossZones, DefaultKeywords); len(c) != 1 || c[0] != "Genichiro Ashina" {
		t.Fatalf("default zones should catch the top-left name: %q", c)
	}
	var zone *ROI
	for _, p := range LoadPresets() {
		if p.Game == "Sekiro: Shadows Die Twice" {
			zone = p.BossZone
		}
	}
	if zone == nil {
		t.Fatal("Sekiro preset needs a boss zone")
	}
	if c := BossCandidates([]OcrLine{sek}, []ROI{*zone}, DefaultKeywords); len(c) != 1 {
		t.Fatalf("Sekiro preset zone missed the name: %q", c)
	}
	// A bottom-center Souls name is outside Sekiro's zone.
	if c := BossCandidates([]OcrLine{line("Margit, the Fell Omen", 0.25, 0.8, 0.025)}, []ROI{*zone}, DefaultKeywords); len(c) != 0 {
		t.Fatalf("zone too wide: %q", c)
	}
}
