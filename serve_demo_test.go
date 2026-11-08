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
		c.LabelText = "{game} · Deaths"
		s.Set(StateFromConfig(c, "Wuchang", 12+i, 0))
	}
	time.Sleep(25 * time.Second)
}
