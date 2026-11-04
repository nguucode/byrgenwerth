package main

import (
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	settleWait = 200 * time.Millisecond
	// A typewriter often pauses on a comma or an ellipsis, so those wait longer.
	typewriterExtraWait = 600 * time.Millisecond
	// OCR can miss text for a frame or two on a busy background.
	clearAfter   = 700 * time.Millisecond
	similarRatio = 0.85
)

type StableEvent int

const (
	StableNone  StableEvent = iota
	StableText              // a new line is ready to translate
	StableClear             // the text left the screen
)

// Stabilizer decides when a stream of OCR reads has become a line worth
// translating. Game text types out, jitters by a character or two between
// frames, and blinks out for a frame.
type Stabilizer struct {
	candidate  string
	changedAt  time.Time
	growing    bool
	settled    bool
	emitted    string
	emptySince time.Time
}

// Feed takes one OCR read and returns the line when the event is StableText.
func (s *Stabilizer) Feed(raw string, now time.Time) (string, StableEvent) {
	text := strings.TrimSpace(raw)
	if !hasWords(text) {
		return "", s.feedEmpty(now)
	}
	s.emptySince = time.Time{}

	growing := s.candidate != "" && isGrowth(s.candidate, text)
	if s.candidate == "" || growing || !isSimilar(text, s.candidate) {
		s.candidate, s.changedAt, s.growing, s.settled = text, now, growing, false
		return "", StableNone
	}
	if s.settled {
		return "", StableNone
	}
	wait := settleWait
	if s.growing && !endsSentence(text) {
		wait += typewriterExtraWait
	}
	if now.Sub(s.changedAt) < wait {
		return "", StableNone
	}
	s.settled = true
	if s.emitted != "" && isSimilar(text, s.emitted) {
		return "", StableNone
	}
	s.emitted = text
	return text, StableText
}

func (s *Stabilizer) feedEmpty(now time.Time) StableEvent {
	if s.candidate == "" {
		return StableNone
	}
	if s.emptySince.IsZero() {
		s.emptySince = now
	}
	if now.Sub(s.emptySince) < clearAfter {
		return StableNone
	}
	*s = Stabilizer{}
	return StableClear
}

func normalizeLine(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

func isSimilar(a, b string) bool {
	x, y := normalizeLine(a), normalizeLine(b)
	if x == y {
		return true
	}
	if x == "" || y == "" {
		return false
	}
	longest := max(len([]rune(x)), len([]rune(y)))
	return 1-float64(editDistance(x, y))/float64(longest) >= similarRatio
}

// isGrowth: current is previous with characters added, tolerating an OCR misread
// or two. Checked before isSimilar, which would call a long line plus a few new
// letters "the same".
func isGrowth(previous, current string) bool {
	p, c := normalizeLine(previous), normalizeLine(current)
	pn := len([]rune(p))
	added := len([]rune(c)) - pn
	if p == "" || added < 2 { // a single extra character is usually OCR noise
		return false
	}
	if strings.HasPrefix(c, p) {
		return true
	}
	return editDistance(p, c)-added <= max(1, pn/20)
}

var sentenceEnd = regexp.MustCompile(`[.!?"”’)\]♪~▼]$`)

func endsSentence(text string) bool {
	t := strings.TrimRight(text, " \t\r\n")
	if strings.HasSuffix(t, "…") || strings.HasSuffix(t, "..") {
		return false
	}
	return sentenceEnd.MatchString(t)
}

// hasWords needs two letters in a row: filters HP bars, stray marks and button
// glyphs read as text.
func hasWords(text string) bool {
	run := 0
	for _, r := range text {
		if !unicode.IsLetter(r) {
			run = 0
			continue
		}
		if run++; run == 2 {
			return true
		}
	}
	return false
}
