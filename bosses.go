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
		if _, isDeath := MatchDeath([]OcrLine{ln}, deathKeywords, 0); isDeath {
			continue
		}
		out = append(out, strings.TrimSpace(ln.Text))
	}
	return out
}

type bossSighting struct {
	text  string
	count int
	since time.Time // start of the current continuous appearance
	last  time.Time
}

// BossTracker decides which on-screen label is the current boss: a label that
// stays on screen for many OCR frames (boss bars persist for the whole fight,
// subtitles and item pickups do not).
type BossTracker struct {
	MinFrames int           // frames needed before a label counts as a boss
	Forget    time.Duration // drop labels not seen for this long
	Recent    time.Duration // a boss seen within this window gets the death
	HUDAfter  time.Duration // a label visible continuously this long is HUD, not a boss
	seen      []*bossSighting
}

func NewBossTracker() *BossTracker {
	return &BossTracker{MinFrames: 10, Forget: 45 * time.Second, Recent: 30 * time.Second, HUDAfter: 15 * time.Minute}
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
		if s.count < t.MinFrames || now.Sub(s.last) > t.Recent {
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
	Version  int64      `json:"v"`
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

type PanelFill struct {
	X, Y, W, H  int
	Top, Bottom RGB // vertical gradient
}

type PanelText struct {
	X, Y  int
	MaxW  int // 0 = no limit; renderer truncates with "…"
	Text  string
	Px    int
	Bold  bool
	Color RGB
	Right bool // X is the right edge
}

var (
	pnBorder   = RGB{0x2b, 0x2b, 0x33}
	pnHeadTop  = RGB{0x26, 0x26, 0x2d}
	pnHeadBot  = RGB{0x16, 0x16, 0x1a}
	pnRowTop   = RGB{0x14, 0x14, 0x18}
	pnRowBot   = RGB{0x0e, 0x0e, 0x11}
	pnActTop   = RGB{0x2f, 0x62, 0xd6}
	pnActBot   = RGB{0x1a, 0x3c, 0x94}
	pnLine     = RGB{0x1f, 0x1f, 0x25}
	pnFoot     = RGB{0x10, 0x10, 0x13}
	pnGray     = RGB{0x9a, 0x9a, 0xa6}
	pnRankGray = RGB{0x8b, 0x8b, 0x96}
	pnGold     = RGB{0xff, 0xd2, 0x3f}
	pnSilver   = RGB{0xd7, 0xd7, 0xe0}
	pnBronze   = RGB{0xd9, 0x89, 0x4a}
	pnWhite    = RGB{0xff, 0xff, 0xff}
	pnFootTxt  = RGB{0xc8, 0xc8, 0xd0}
	pnRed      = RGB{0xff, 0x4d, 0x4d}
)

// LayoutSplits computes the LiveSplit-style panel; measure returns text size in px.
func LayoutSplits(st SplitsState, fs int, measure func(text string, px int, bold bool) (int, int)) (int, int, []PanelFill, []PanelText) {
	if fs < 10 {
		fs = 10
	}
	r := func(f float64) int { return int(math.Round(f)) }
	pad, gap := r(0.6*float64(fs)), r(0.55*float64(fs))
	hPx, fPx := r(0.72*float64(fs)), r(0.85*float64(fs))
	_, lineH := measure("Ag", fs, false)
	_, hLineH := measure("Ag", hPx, false)
	_, fLineH := measure("Ag", fPx, false)
	headerH := hLineH + r(0.7*float64(fs))
	rowH := lineH + r(0.64*float64(fs))
	footerH := fLineH + r(0.8*float64(fs))

	title := strings.ToUpper(st.Title)
	if title == "" {
		title = "TOP BOSS"
	}
	rankW, countW, nameW := 0, 0, 0
	for _, row := range st.Rows {
		rk := "▶"
		if row.Rank > 0 {
			rk = fmt.Sprint(row.Rank)
		}
		w, _ := measure(rk, fs, true)
		rankW = max(rankW, w)
		w, _ = measure(fmt.Sprint(row.Deaths), fs, true)
		countW = max(countW, w)
		w, _ = measure(row.Name, fs, false)
		nameW = max(nameW, w)
	}
	w8, _ := measure("88", fs, true)
	countW = max(countW, w8)
	nameW = min(nameW, fs*16)
	tw, _ := measure(title, hPx, false)
	dw, _ := measure("DEATHS", hPx, false)
	W := max(pad+rankW+gap+nameW+gap+countW+pad, pad+tw+gap*2+dw+pad, fs*12)
	ftw, _ := measure(fmt.Sprint(st.Total), fPx, true)
	flw, _ := measure("Tổng số lần chết", fPx, false)
	W = max(W, pad+flw+gap*2+ftw+pad)

	var fills []PanelFill
	var texts []PanelText
	y := 1
	inner := W - 2
	fills = append(fills, PanelFill{0, 0, W, 0, pnBorder, pnBorder}) // height set below
	fills = append(fills, PanelFill{1, y, inner, headerH, pnHeadTop, pnHeadBot})
	texts = append(texts, PanelText{X: pad, Y: y + (headerH-hLineH)/2, Text: title, Px: hPx, Color: pnGray, MaxW: inner - pad*2 - dw - gap})
	texts = append(texts, PanelText{X: W - pad, Y: y + (headerH-hLineH)/2, Text: "DEATHS", Px: hPx, Color: pnGray, Right: true})
	y += headerH + 1
	if len(st.Rows) == 0 {
		fills = append(fills, PanelFill{1, y, inner, rowH, pnRowTop, pnRowBot})
		texts = append(texts, PanelText{X: pad, Y: y + (rowH-lineH)/2, Text: "Chưa có dữ liệu boss", Px: fPx, Color: pnRankGray})
		y += rowH + 1
	}
	for i, row := range st.Rows {
		if (row.Rank == 0 || row.Rank > 3) && i > 0 {
			y += 2 // separator before the extra "current boss" row
		}
		top, bot := pnRowTop, pnRowBot
		if row.Active {
			top, bot = pnActTop, pnActBot
		}
		fills = append(fills, PanelFill{1, y, inner, rowH, top, bot})
		rankC, countC := pnRankGray, pnWhite
		switch row.Rank {
		case 1:
			rankC, countC = pnGold, pnGold
		case 2:
			rankC = pnSilver
		case 3:
			rankC = pnBronze
		}
		if row.Active {
			countC = pnWhite
		}
		rk := "▶"
		if row.Rank > 0 {
			rk = fmt.Sprint(row.Rank)
		}
		ty := y + (rowH-lineH)/2
		texts = append(texts,
			PanelText{X: pad + rankW, Y: ty, Text: rk, Px: fs, Bold: true, Color: rankC, Right: true},
			PanelText{X: pad + rankW + gap, Y: ty, Text: row.Name, Px: fs, Color: pnWhite, MaxW: nameW},
			PanelText{X: W - pad, Y: ty, Text: fmt.Sprint(row.Deaths), Px: fs, Bold: true, Color: countC, Right: true},
		)
		y += rowH
		fills = append(fills, PanelFill{1, y, inner, 1, pnLine, pnLine})
		y++
	}
	fills = append(fills, PanelFill{1, y, inner, footerH, pnFoot, pnFoot})
	fy := y + (footerH-fLineH)/2
	texts = append(texts,
		PanelText{X: pad, Y: fy, Text: "Tổng số lần chết", Px: fPx, Color: pnFootTxt},
		PanelText{X: W - pad, Y: fy, Text: fmt.Sprint(st.Total), Px: fPx, Bold: true, Color: pnRed, Right: true},
	)
	y += footerH + 1
	fills[0].H = y
	return W, y, fills, texts
}
