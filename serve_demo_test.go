package main

import (
	"os"
	"testing"
	"time"
)

// Manual helper: DC_SERVE=1 go test -run TestServeDemo  (serves 3 styles for screenshots)
func TestServeDemo(t *testing.T) {
	if os.Getenv("DC_SERVE") == "" {
		t.Skip("manual")
	}
	styles := []func(*Config){
		func(c *Config) {}, // default: white bold, black outline
		func(c *Config) {
			c.TextColor = "#FF3B30"
			c.OutlineColor = "#FFFFFF"
			c.OutlineWidth = 2
			c.FontFamily = "DejaVu Serif"
		},
		func(c *Config) {
			c.TextColor = "#FFD228"
			c.Outline = false
			c.FontBold = false
			c.OverlayFontSize = 56
		},
	}
	for i, f := range styles {
		c := DefaultConfig()
		f(&c)
		s, err := StartOverlayServer(18001 + i)
		if err != nil {
			t.Fatal(err)
		}
		s.Set(StateFromConfig(c, FormatObs("{game} · Deaths: {count}", "Wuchang", 12+i)))
	}
	bs, _ := StartOverlayServer(18010)
	p := &Profile{Bosses: []BossStat{{"Margit, the Fell Omen", 23}, {"Godrick the Grafted", 11}, {"Rennala, Queen of the Full Moon", 7}, {"Starscourge Radahn", 4}}}
	bs.SetSplits(SplitsState{Title: "Elden Ring · Top boss", Rows: BuildSplits(p, "Godrick the Grafted", 3), Total: 58, FontSize: 20})
	bs2, _ := StartOverlayServer(18011)
	bs2.SetSplits(SplitsState{Title: "Elden Ring · Top boss", Rows: BuildSplits(p, "Starscourge Radahn", 3), Total: 58, FontSize: 20})
	time.Sleep(25 * time.Second)
}
