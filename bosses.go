package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// BossStat is how many times a boss killed the player.
type BossStat struct {
	Name   string `json:"name"`
	Deaths int    `json:"deaths"`
}

// bossKey is the comparison form of a boss name.
func bossKey(s string) string { return NormalizeText(s) }

// sameBoss tolerates small OCR errors: "Margit, the Fell 0men" == "Margit the Fell Omen".
func sameBoss(a, b string) bool {
	ka, kb := bossKey(a), bossKey(b)
	if ka == kb {
		return true
	}
	n := len([]rune(ka))
	if m := len([]rune(kb)); m > n {
		n = m
	}
	tol := n / 8
	if tol < 1 {
		tol = 1
	}
	if n <= 6 {
		tol = 0 // short names: one letter is a different boss
	}
	return editDistance(ka, kb) <= tol
}

// AddBossDeath adds delta (+1/-1) to the boss, creating it if needed; returns the stored name.
func AddBossDeath(p *Profile, name string, delta int) string {
	for i := range p.Bosses {
		if sameBoss(p.Bosses[i].Name, name) {
			p.Bosses[i].Deaths += delta
			stored := p.Bosses[i].Name
			if p.Bosses[i].Deaths <= 0 {
				p.Bosses = append(p.Bosses[:i], p.Bosses[i+1:]...)
			}
			return stored
		}
	}
	if delta > 0 {
		p.Bosses = append(p.Bosses, BossStat{Name: strings.TrimSpace(name), Deaths: delta})
	}
	return strings.TrimSpace(name)
}

// SortedBosses returns bosses by deaths desc, then name.
func SortedBosses(p *Profile) []BossStat {
	out := append([]BossStat(nil), p.Bosses...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Deaths != out[j].Deaths {
			return out[i].Deaths > out[j].Deaths
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func RenameBoss(p *Profile, old, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	for i := range p.Bosses {
		if p.Bosses[i].Name == old {
			p.Bosses[i].Name = name
			return
		}
	}
}

// MergeBoss moves src's deaths into dst and removes src.
func MergeBoss(p *Profile, src, dst string) {
	si, di := -1, -1
	for i := range p.Bosses {
		switch p.Bosses[i].Name {
		case src:
			si = i
		case dst:
			di = i
		}
	}
	if si < 0 || di < 0 || si == di {
		return
	}
	p.Bosses[di].Deaths += p.Bosses[si].Deaths
	p.Bosses = append(p.Bosses[:si], p.Bosses[si+1:]...)
}

func RemoveBoss(p *Profile, name string) {
	for i := range p.Bosses {
		if p.Bosses[i].Name == name {
			p.Bosses = append(p.Bosses[:i], p.Bosses[i+1:]...)
			return
		}
	}
}

// ---------- boss name detection ----------

// DefaultBossZones are where games usually print the boss name above its health
// bar: bottom-center (Souls-likes) and top-center (many action games).
var DefaultBossZones = []ROI{
	{0.10, 0.58, 0.80, 0.40}, // bottom band (Souls-likes: name above a bottom-center bar)
	{0.00, 0.00, 1.00, 0.24}, // full top band (top-center bars, Sekiro's top-left bar)
}

// BossCandidates returns OCR lines that look like a boss name label.
func BossCandidates(lines []OcrLine, zones []ROI, deathKeywords []string) []string {
	var out []string
	for _, ln := range lines {
		if len(ln.Words) == 0 {
			continue
		}
		x0, y0, x1, y1, h := 1.0, 1.0, 0.0, 0.0, 0.0
		for _, w := range ln.Words {
			x0, y0 = math.Min(x0, w.X), math.Min(y0, w.Y)
			x1, y1 = math.Max(x1, w.X+w.W), math.Max(y1, w.Y+w.H)
			h = math.Max(h, w.H)
		}
		cx, cy := (x0+x1)/2, (y0+y1)/2
		in := false
		for _, z := range zones {
			if cx >= z.X && cx <= z.X+z.W && cy >= z.Y && cy <= z.Y+z.H {
				in = true
				break
			}
		}
		if !in || h < 0.012 || h > 0.08 {
			continue
		}
		key := bossKey(ln.Text)
		letters := 0
		for _, r := range key {
			if r != ' ' && !(r >= '0' && r <= '9') {
				letters++
			}
		}
		words := len(strings.Fields(key))
		if letters < 3 || len([]rune(key)) > 48 || (!isCJK(key) && words > 8) {
			continue
		}
		if strings.Contains(ln.Text, ":") {
			continue // button prompts ("E : Rest at bonfire") and HUD labels, not boss names
		}
		if _, isDeath := MatchDeath([]OcrLine{ln}, deathKeywords, 0); isDeath {
			continue
		}
		out = append(out, strings.TrimSpace(ln.Text))
	}
	return out
}

// WithoutNames drops candidates that are really the app's own labels (the
// counter overlay, the boss panel title), e.g. when OBS's preview shows them.
func WithoutNames(cands, own []string) []string {
	out := cands[:0:0]
	for _, c := range cands {
		mine := false
		for _, o := range own {
			if strings.TrimSpace(o) != "" && sameBoss(c, o) {
				mine = true
				break
			}
		}
		if !mine {
			out = append(out, c)
		}
	}
	return out
}

type bossSighting struct {
	text  string
	count int
	since time.Time // start of the current continuous appearance
	last  time.Time
	boss  bool // once on screen for MinOnScreen; stays a boss when it comes back
}

// BossTracker decides which on-screen label is the current boss: a label that
// stays on screen for many OCR frames (boss bars persist for the whole fight,
// subtitles and item pickups do not).
type BossTracker struct {
	MinOnScreen time.Duration // a label counts as a boss after staying on screen this long
	Forget      time.Duration // drop labels not seen for this long
	Recent      time.Duration // a boss seen within this window gets the death
	HUDAfter    time.Duration // a label visible continuously this long is HUD, not a boss
	seen        []*bossSighting
}

func NewBossTracker() *BossTracker {
	return &BossTracker{MinOnScreen: 5 * time.Second, Forget: 45 * time.Second, Recent: 30 * time.Second, HUDAfter: 15 * time.Minute}
}

func (t *BossTracker) Observe(cands []string, now time.Time) {
	kept := t.seen[:0]
	for _, s := range t.seen {
		if now.Sub(s.last) <= t.Forget {
			kept = append(kept, s)
		}
	}
	t.seen = kept
	for _, c := range cands {
		found := false
		for _, s := range t.seen {
			if sameBoss(s.text, c) {
				if now.Sub(s.last) > 5*time.Second {
					s.since = now // it went away and came back: new appearance
				}
				s.count++
				s.last = now
				s.boss = s.boss || now.Sub(s.since) >= t.MinOnScreen
				found = true
				break
			}
		}
		if !found {
			t.seen = append(t.seen, &bossSighting{text: c, count: 1, since: now, last: now})
		}
	}
}

// Current returns the boss the player is (or was very recently) fighting.
func (t *BossTracker) Current(now time.Time) (string, bool) {
	var best *bossSighting
	for _, s := range t.seen {
		// Time, not frame count: OCR runs about once a second on big screens, and a
		// first fight can end within 10 frames.
		if !s.boss || now.Sub(s.last) > t.Recent {
			continue
		}
		if t.HUDAfter > 0 && s.last.Sub(s.since) > t.HUDAfter {
			continue // always-on label (player name, area name...)
		}
		if best == nil || s.last.After(best.last) || (s.last.Equal(best.last) && s.count > best.count) {
			best = s
		}
	}
	if best == nil {
		return "", false
	}
	return best.text, true
}

func (t *BossTracker) Reset() { t.seen = nil }

// ---------- speedrun-style panel ----------

type SplitRow struct {
	Rank   int    `json:"rank"`
	Name   string `json:"name"`
	Deaths int    `json:"deaths"`
	Active bool   `json:"active"`
}

type SplitsState struct {
	Title    string     `json:"title"`
	Rows     []SplitRow `json:"rows"`
	Total    int        `json:"total"`
	FontSize int        `json:"size"`
	Font     string     `json:"font"`
	// Text style shared with the death count overlay.
	Color        string `json:"color"`
	Bold         bool   `json:"bold"`
	OutlineWidth int    `json:"outlineWidth"` // 0 = no outline
	OutlineColor string `json:"outlineColor"`
	Version      int64  `json:"v"`
}

// BuildSplits makes the top-n rows; if the boss being fought is outside the
// top n it is appended as an extra highlighted row, like LiveSplit's current split.
func BuildSplits(p *Profile, current string, n int) []SplitRow {
	all := SortedBosses(p)
	var rows []SplitRow
	activeShown := false
	for i, b := range all {
		if i >= n {
			break
		}
		act := current != "" && sameBoss(b.Name, current)
		activeShown = activeShown || act
		rows = append(rows, SplitRow{Rank: i + 1, Name: b.Name, Deaths: b.Deaths, Active: act})
	}
	if current != "" && !activeShown {
		r := SplitRow{Rank: 0, Name: current, Active: true}
		for i, b := range all {
			if sameBoss(b.Name, current) {
				r.Rank, r.Deaths, r.Name = i+1, b.Deaths, b.Name
			}
		}
		rows = append(rows, r)
	}
	return rows
}

// ---------- native panel layout (rendered by the Windows overlay) ----------

// PanelRect is a rule line, drawn in the text color.
type PanelRect struct{ X, Y, W, H int }

type PanelText struct {
	X, Y  int
	MaxW  int // 0 = no limit; renderer truncates to one line with "…"
	Text  string
	Px    int
	Bold  bool
	Right bool // X is the right edge
}

// LayoutSplits lays out the boss panel as plain text and rules: game name, a
// rule, "Deaths" with the total, a rule, then one line per boss with its count.
// measure returns text size in px.
func LayoutSplits(st SplitsState, fs int, measure func(text string, px int, bold bool) (int, int)) (int, int, []PanelRect, []PanelText) {
	if fs < 10 {
		fs = 10
	}
	r := func(f float64) int { return int(math.Round(f)) }
	gap, space := r(1.5*float64(fs)), r(0.3*float64(fs))
	rule := max(2, r(0.1*float64(fs)))
	_, lineH := measure("Ag", fs, st.Bold)

	title := st.Title
	if title == "" {
		title = "Byrgenwerth"
	}
	name := func(row SplitRow) string {
		if row.Active {
			return "▶ " + row.Name // the boss being fought
		}
		return row.Name
	}
	tw, _ := measure(title, fs, st.Bold)
	dw, _ := measure("Deaths", fs, st.Bold)
	cw, _ := measure(fmt.Sprint(st.Total), fs, st.Bold)
	nameW := 0
	for _, row := range st.Rows {
		w, _ := measure(name(row), fs, st.Bold)
		nameW = max(nameW, w)
		w, _ = measure(fmt.Sprint(row.Deaths), fs, st.Bold)
		cw = max(cw, w)
	}
	nameW = min(nameW, fs*14) // long names are cut to one line
	W := max(min(tw, fs*20), dw+gap+cw, nameW+gap+cw, fs*10)

	var rules []PanelRect
	var texts []PanelText
	y := 0
	texts = append(texts, PanelText{X: 0, Y: y, Text: title, Px: fs, Bold: st.Bold, MaxW: W})
	y += lineH + space
	rules = append(rules, PanelRect{0, y, W, rule})
	y += rule + space
	texts = append(texts,
		PanelText{X: 0, Y: y, Text: "Deaths", Px: fs, Bold: st.Bold},
		PanelText{X: W, Y: y, Text: fmt.Sprint(st.Total), Px: fs, Bold: st.Bold, Right: true},
	)
	y += lineH
	if len(st.Rows) > 0 {
		y += space
		rules = append(rules, PanelRect{0, y, W, rule})
		y += rule + space
	}
	for _, row := range st.Rows {
		texts = append(texts,
			PanelText{X: 0, Y: y, Text: name(row), Px: fs, Bold: st.Bold, MaxW: W - gap - cw},
			PanelText{X: W, Y: y, Text: fmt.Sprint(row.Deaths), Px: fs, Bold: st.Bold, Right: true},
		)
		y += lineH
	}
	return W, y, rules, texts
}
