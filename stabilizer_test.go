package main

import (
	"strings"
	"testing"
	"time"
)

type stabFrame struct {
	ms   int
	text string
}

// runStabilizer feeds frames and returns what came out, "" meaning a clear.
func runStabilizer(frames []stabFrame) []string {
	var s Stabilizer
	start := time.Unix(1000, 0)
	var out []string
	for _, f := range frames {
		if text, ev := s.Feed(f.text, start.Add(time.Duration(f.ms)*time.Millisecond)); ev == StableText {
			out = append(out, text)
		} else if ev == StableClear {
			out = append(out, "")
		}
	}
	return out
}

func sameStrings(a, b []string) bool {
	return strings.Join(a, "\x00") == strings.Join(b, "\x00") && len(a) == len(b)
}

const stabLine = "The king has fallen, and the north will not forgive you."

func TestStabilizerEmitsAStableLineOnce(t *testing.T) {
	got := runStabilizer([]stabFrame{{0, stabLine}, {100, stabLine}, {250, stabLine}, {500, stabLine}, {750, stabLine}})
	if !sameStrings(got, []string{stabLine}) {
		t.Fatalf("got %q", got)
	}
}

func TestStabilizerWaitsForTypewriterToFinish(t *testing.T) {
	var frames []stabFrame
	at := 0
	for n := 10; n < len(stabLine); n += 6 {
		frames = append(frames, stabFrame{at, stabLine[:n]})
		at += 250
	}
	for i := 0; i < 4; i++ {
		frames = append(frames, stabFrame{at, stabLine})
		at += 250
	}
	if got := runStabilizer(frames); !sameStrings(got, []string{stabLine}) {
		t.Fatalf("got %q", got)
	}
}

func TestStabilizerGivesAMidSentencePauseLonger(t *testing.T) {
	part := "Well, you see, I never really wanted to"
	got := runStabilizer([]stabFrame{{0, "Well, you see,"}, {250, part}, {500, part}, {750, part}})
	if len(got) != 0 {
		t.Fatalf("emitted a half sentence: %q", got)
	}
	got = runStabilizer([]stabFrame{{0, "Well, you see,"}, {250, part}, {500, part}, {750, part}, {1100, part}})
	if !sameStrings(got, []string{part}) {
		t.Fatalf("never emitted after the long wait: %q", got)
	}
}

func TestEndsSentence(t *testing.T) {
	for text, want := range map[string]bool{stabLine: true, "Wait...": false, "Wait…": false, "Really?": true, "and then": false, `"Go."`: true} {
		if endsSentence(text) != want {
			t.Errorf("endsSentence(%q) = %v", text, !want)
		}
	}
}

func TestStabilizerOcrJitterDoesNotTranslateTwice(t *testing.T) {
	jitter := strings.Replace(stabLine, "king", "k1ng", 1)
	got := runStabilizer([]stabFrame{{0, stabLine}, {250, jitter}, {500, stabLine}, {750, jitter}, {1000, stabLine}})
	if len(got) != 1 {
		t.Fatalf("got %q", got)
	}
}

func TestStabilizerEmitsANewLineAfterThePrevious(t *testing.T) {
	next := "Then let them come. I have nothing left to lose."
	got := runStabilizer([]stabFrame{{0, stabLine}, {250, stabLine}, {500, next}, {750, next}})
	if !sameStrings(got, []string{stabLine, next}) {
		t.Fatalf("got %q", got)
	}
}

func TestStabilizerSurvivesAOneFrameDropout(t *testing.T) {
	got := runStabilizer([]stabFrame{{0, stabLine}, {250, stabLine}, {500, ""}, {750, stabLine}, {1000, stabLine}})
	if !sameStrings(got, []string{stabLine}) {
		t.Fatalf("got %q", got)
	}
}

func TestStabilizerClearsAndTheSameLineCanReturn(t *testing.T) {
	got := runStabilizer([]stabFrame{{0, stabLine}, {250, stabLine}, {500, ""}, {1300, ""}, {1500, stabLine}, {1750, stabLine}})
	if !sameStrings(got, []string{stabLine, "", stabLine}) {
		t.Fatalf("got %q", got)
	}
}

func TestStabilizerTreatsFramesWithoutWordsAsEmpty(t *testing.T) {
	if got := runStabilizer([]stabFrame{{0, "100/100"}, {250, "|| -"}, {500, "7"}}); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}

func TestStabilizerHandlesVietnameseLettersAsWords(t *testing.T) {
	if !hasWords("đã") || hasWords("1 2") {
		t.Fatal("hasWords must count letters of any script, not digits")
	}
}

func TestIsSimilarAndIsGrowth(t *testing.T) {
	if !isSimilar(stabLine, strings.ToUpper(stabLine)) || isSimilar("Attack", "Defend") {
		t.Error("isSimilar")
	}
	if !isGrowth("The king", "The king has fallen") {
		t.Error("real growth not seen")
	}
	if isGrowth("The king", "The king.") {
		t.Error("one stray character is noise, not growth")
	}
	if isGrowth("The king", "A completely different sentence") {
		t.Error("a different sentence is not growth")
	}
}
