package main

import (
	"os"
	"strings"
	"testing"
)

// The supported-games list in both guides must name every preset.
func TestGuidesListAllPresets(t *testing.T) {
	for _, f := range []string{"docs/user-guide.md", "docs/huong-dan-su-dung.md"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range LoadPresets() {
			if !strings.Contains(string(b), p.Game) {
				t.Errorf("%s does not list %q", f, p.Game)
			}
		}
	}
}

func TestFindPreset(t *testing.T) {
	ps := []Preset{
		{Game: "Dark Souls III"}, {Game: "Dark Souls Remastered"}, {Game: "Elden Ring"},
		{Game: "Elden Ring Nightreign"}, {Game: "Wuchang: Fallen Feathers"}, {Game: "Hades"}, {Game: "Hades II"},
	}
	cases := map[string]string{
		"wuchang":                 "Wuchang: Fallen Feathers",
		"Wuchang Fallen Feathers": "Wuchang: Fallen Feathers",
		"elden ring":              "Elden Ring",
		"Elden Ring Nightreign":   "Elden Ring Nightreign",
		"hades":                   "Hades",
		"Hades II":                "Hades II",
		"dark souls iii":          "Dark Souls III",
	}
	for in, want := range cases {
		i := FindPreset(ps, in)
		if i < 0 || ps[i].Game != want {
			got := "<none>"
			if i >= 0 {
				got = ps[i].Game
			}
			t.Errorf("%q -> %s, want %s", in, got, want)
		}
	}
	for _, in := range []string{"", "ga", "Game mới", "Minecraft"} {
		if i := FindPreset(ps, in); i >= 0 {
			t.Errorf("%q should not match, got %s", in, ps[i].Game)
		}
	}
}

func TestPresetHint(t *testing.T) {
	if (Preset{OCRHint: "none"}).HintText() == (Preset{OCRHint: "ok"}).HintText() {
		t.Fatal("hints should differ")
	}
}

// Every verified death text must trigger its own preset, drawn as a big centered banner.
func TestPresetsMatchVerifiedTexts(t *testing.T) {
	ps := LoadPresets()
	if len(ps) != 100 {
		t.Fatalf("want 100 presets, got %d", len(ps))
	}
	verified := map[string][]string{
		"Elden Ring":                    {"YOU DIED"},
		"Sekiro: Shadows Die Twice":     {"DEATH"},
		"The First Berserker: Khazan":   {"KHAZAN HAS FALLEN"},
		"Diablo IV":                     {"You have died"},
		"The Witcher 3: Wild Hunt":      {"You are dead"},
		"Fortnite":                      {"ELIMINATED BY Ninja"},
		"Apex Legends":                  {"SQUAD ELIMINATED"},
		"PUBG: Battlegrounds":           {"BETTER LUCK NEXT TIME!"},
		"Overwatch 2":                   {"ELIMINATED BY Reaper"},
		"Escape from Tarkov":            {"KILLED IN ACTION"},
		"Team Fortress 2":               {"You were killed by Heavy", "You have been killed by Spy"},
		"Left 4 Dead 2":                 {"You will be rescued soon", "The survivors have been overwhelmed"},
		"Grand Theft Auto V":            {"WASTED"},
		"Red Dead Redemption 2":         {"DEAD"},
		"Resident Evil 4 (2023 remake)": {"YOU ARE DEAD"},
		"Assassin's Creed Shadows":      {"DESYNCHRONIZED"},
		"Hades":                         {"There is no escape", "Is there no escape?"},
		"Minecraft":                     {"You Died!", "Game Over!"},
		"Terraria":                      {"You were slain...", "You were slain by Eye of Cthulhu"},
		"Subnautica":                    {"You died and lost some belongings"},
		"Lethal Company":                {"[LIFE SUPPORT: OFFLINE]"},
		"Risk of Rain 2":                {"Defeat!"},
		"Balatro":                       {"GAME OVER"},
		"Fall Guys":                     {"ELIMINATED!"},
		"Wuchang: Fallen Feathers":      {"DEAD"},
	}
	byName := map[string]Preset{}
	for _, p := range ps {
		byName[p.Game] = p
	}
	for game, texts := range verified {
		p, ok := byName[game]
		if !ok {
			t.Errorf("preset missing: %s", game)
			continue
		}
		for _, tx := range texts {
			ln := line(tx, 0.3, 0.45, 0.05)
			if _, ok := MatchDeath([]OcrLine{ln}, p.Keywords, 0.025); !ok {
				t.Errorf("%s: %q not matched by %q", game, tx, p.Keywords)
			}
		}
	}
	// Screens that are NOT a death must not trigger any preset.
	notDeath := []string{"BUSTED", "QUALIFIED!", "New Best!", "KNOCKED BY Ninja", "CONTINUE", "LOADING",
		"PRESS ANY KEY", "MISSION COMPLETE", "Resurrect at Checkpoint", "VICTORY", "TIME PARADOX", "KILL CAM"}
	for _, p := range ps {
		for _, tx := range notDeath {
			if m, ok := MatchDeath([]OcrLine{line(tx, 0.3, 0.45, 0.05)}, EffectiveKeywords(&Profile{Keywords: p.Keywords}, DefaultKeywords), 0.025); ok {
				t.Errorf("%s: non-death %q matched keyword %q", p.Game, tx, m.Keyword)
			}
		}
	}
	// Presets without keywords fall back to the generic list and say why.
	for _, p := range ps {
		if len(p.Keywords) == 0 && p.OCRHint == "ok" {
			t.Errorf("%s: verified preset without keywords", p.Game)
		}
		if p.HintText() == "" {
			t.Errorf("%s: empty hint", p.Game)
		}
	}
}
