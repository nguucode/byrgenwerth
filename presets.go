package main

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
)

//go:embed presets.json
var presetsJSON []byte

// Preset holds the death-screen keywords researched for one game.
type Preset struct {
	Game     string   `json:"game"`
	Keywords []string `json:"keywords"`
	// OCRHint: "ok" readable text; "stylized" decorative font, OCR may miss it;
	// "none" no death text at all -> use F9 + templates.
	OCRHint string `json:"ocr"`
	Note    string `json:"note"`
	// BossZone: where this game prints the boss name (nil = default zones).
	BossZone *ROI `json:"boss_zone,omitempty"`
}

func LoadPresets() []Preset {
	var ps []Preset
	if json.Unmarshal(presetsJSON, &ps) != nil {
		return nil
	}
	sort.Slice(ps, func(i, j int) bool { return strings.ToLower(ps[i].Game) < strings.ToLower(ps[j].Game) })
	return ps
}

// FindPreset matches a profile name to a preset, e.g. "wuchang" -> "Wuchang: Fallen Feathers".
// Returns -1 when nothing matches clearly.
func FindPreset(ps []Preset, name string) int {
	n := NormalizeText(name)
	if len(n) < 3 {
		return -1
	}
	best, bestLen := -1, 0
	for i, p := range ps {
		g := NormalizeText(p.Game)
		switch {
		case g == n:
			return i
		case strings.HasPrefix(g, n+" ") || strings.HasPrefix(n, g+" ") || strings.HasPrefix(g, n):
			if len(g) > bestLen && (best == -1 || len(n) >= 4) {
				best, bestLen = i, len(g)
			}
		}
	}
	return best
}

func (p Preset) HintText() string {
	base := ""
	switch p.OCRHint {
	case "none":
		base = "Game này không hiện chữ khi chết: dùng F9 lần đầu để app học ảnh mẫu."
	case "small":
		base = "Chữ chết nhỏ hoặc nằm ở góc: OCR khó bắt, nên dùng F9 + ảnh mẫu."
	case "stylized":
		base = "Chữ chết cách điệu, OCR có thể đọc trượt: nếu không tự đếm, bấm F9 lần đầu."
	case "unverified":
		base = "Chưa xác minh được chữ chết của game này: đang dùng từ khóa chung."
	default:
		base = "Đã xác minh chữ chết của game này."
	}
	if p.Note != "" {
		return base + " " + p.Note
	}
	return base
}
