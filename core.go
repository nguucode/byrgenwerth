package main

// Platform-independent logic: image matching, detector state machine,
// storage, OBS output, hotkey parsing. Unit-tested on any OS.

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// ---------- Image matching ----------

const (
	CaptureW     = 320 // size every capture/template is stored at
	CaptureH     = 180
	FeatW        = 64 // size of the comparison vector
	FeatH        = 32
	MaxTemplates = 10
)

// ROI is a region of interest in relative coordinates (0..1).
type ROI struct {
	X, Y, W, H float64
}

func FullROI() ROI { return ROI{0, 0, 1, 1} }

func (r ROI) Normalized() ROI {
	if r.W <= 0 || r.H <= 0 {
		return FullROI()
	}
	clamp := func(v float64) float64 { return math.Max(0, math.Min(1, v)) }
	x, y := clamp(r.X), clamp(r.Y)
	w, h := math.Min(r.W, 1-x), math.Min(r.H, 1-y)
	if w < 0.02 || h < 0.02 {
		return FullROI()
	}
	return ROI{x, y, w, h}
}

func (r ROI) IsFull() bool {
	n := r.Normalized()
	return n.X == 0 && n.Y == 0 && n.W == 1 && n.H == 1
}

// Feature is a zero-mean grayscale vector of the ROI, ready for NCC.
type Feature struct {
	v    []float64
	norm float64 // sqrt(sum v^2)
	mean float64
}

// ExtractFeature crops img to roi and area-averages it into FeatW x FeatH gray.
func ExtractFeature(img image.Image, roi ROI) Feature {
	vals := ExtractFeatureSize(img, roi, FeatW, FeatH)
	var mean float64
	for _, v := range vals {
		mean += v
	}
	mean /= float64(len(vals))
	var ss float64
	for i := range vals {
		vals[i] -= mean
		ss += vals[i] * vals[i]
	}
	return Feature{v: vals, norm: math.Sqrt(ss), mean: mean}
}

// TemplateAgreement is the lowest pairwise similarity between templates.
// A low value means the ROI includes too much changing background.
// Returns -1 when there are fewer than 2 templates.
func TemplateAgreement(ts []Feature) float64 {
	if len(ts) < 2 {
		return -1
	}
	minS := 1.0
	for i := range ts {
		for j := i + 1; j < len(ts); j++ {
			if s := Similarity(ts[i], ts[j]); s < minS {
				minS = s
			}
		}
	}
	return minS
}

// Similarity returns 0..1. Normalized cross-correlation, so it ignores
// overall brightness/contrast changes and focuses on structure.
func Similarity(a, b Feature) float64 {
	if len(a.v) == 0 || len(a.v) != len(b.v) {
		return 0
	}
	// Nearly flat images (e.g. a black frame): NCC is undefined, compare means.
	if a.norm < 50 || b.norm < 50 {
		if a.norm < 50 && b.norm < 50 {
			return math.Max(0, 1-math.Abs(a.mean-b.mean)/64)
		}
		return 0
	}
	var dot float64
	for i := range a.v {
		dot += a.v[i] * b.v[i]
	}
	s := dot / (a.norm * b.norm)
	if s < 0 {
		return 0
	}
	if s > 1 {
		return 1
	}
	return s
}

// BestSimilarity returns the highest similarity against all templates and its index.
func BestSimilarity(f Feature, templates []Feature) (float64, int) {
	best, idx := 0.0, -1
	for i, t := range templates {
		if s := Similarity(f, t); s > best {
			best, idx = s, i
		}
	}
	return best, idx
}

// ---------- Detector ----------

// Detector turns a stream of similarity scores into discrete death events.
// One death screen = one count: after counting it disarms until the score
// drops clearly below the threshold AND the cooldown has passed.
type Detector struct {
	Threshold    float64       // 0..1
	Cooldown     time.Duration // min time between two counts
	ConsecNeeded int           // frames above threshold needed to count
	RearmMargin  float64       // must drop below Threshold-RearmMargin to rearm

	consec    int
	below     int
	disarmed  bool
	lastCount time.Time
	lastAuto  time.Time
}

func NewDetector(threshold float64, cooldown time.Duration) *Detector {
	return &Detector{Threshold: threshold, Cooldown: cooldown, ConsecNeeded: 2, RearmMargin: 0.07}
}

// Feed returns true when a new death should be counted automatically.
func (d *Detector) Feed(sim float64, now time.Time) bool {
	if sim >= d.Threshold {
		d.consec++
	} else {
		d.consec = 0
	}
	if d.disarmed {
		if sim < d.Threshold-d.RearmMargin {
			d.below++
		} else {
			d.below = 0
		}
		if d.below >= 2 && now.Sub(d.lastCount) >= d.Cooldown {
			d.disarmed = false
			d.below = 0
		}
		return false
	}
	if d.consec >= d.ConsecNeeded && now.Sub(d.lastCount) >= d.Cooldown {
		d.disarmed = true
		d.consec = 0
		d.below = 0
		d.lastCount = now
		d.lastAuto = now
		return true
	}
	return false
}

// MarkManual records a manual count so the same death screen isn't counted again.
func (d *Detector) MarkManual(now time.Time) {
	d.disarmed = true
	d.consec = 0
	d.below = 0
	d.lastCount = now
}

// Trigger counts a death signalled by another detector (health bar), unless a
// death was already counted within the cooldown.
func (d *Detector) Trigger(now time.Time) bool {
	if !d.lastCount.IsZero() && now.Sub(d.lastCount) < d.Cooldown {
		return false
	}
	d.disarmed = true
	d.consec, d.below = 0, 0
	d.lastCount = now
	d.lastAuto = now
	return true
}

// AutoCountedRecently is true if an automatic count happened within the cooldown,
// meaning a manual hotkey press right now is most likely the same death.
func (d *Detector) AutoCountedRecently(now time.Time) bool {
	return !d.lastAuto.IsZero() && now.Sub(d.lastAuto) < d.Cooldown
}

func (d *Detector) Reset() {
	d.consec, d.below, d.disarmed = 0, 0, false
	d.lastCount, d.lastAuto = time.Time{}, time.Time{}
}

// ---------- Storage ----------

type Hotkeys struct {
	Add     string `json:"add"`
	Sub     string `json:"sub"`
	Capture string `json:"capture"`
	Toggle  string `json:"toggle"`
	Window  string `json:"window"`
}

type Config struct {
	CurrentProfile string  `json:"current_profile"`
	AlwaysOnTop    bool    `json:"always_on_top"`
	Monitor        int     `json:"monitor"`
	IntervalMs     int     `json:"interval_ms"`
	Sound          bool    `json:"sound"`
	ObsFormat      string  `json:"obs_format"`
	Hotkeys        Hotkeys `json:"hotkeys"`

	ObsEnabled      bool   `json:"obs_enabled"`
	OverlayEnabled  bool   `json:"overlay_enabled"`
	OverlayX        int32  `json:"overlay_x"`
	OverlayY        int32  `json:"overlay_y"`
	OverlayPosSet   bool   `json:"overlay_pos_set"`
	OverlayFontSize int    `json:"overlay_font_size"`
	OverlayColor    string `json:"overlay_color"`
	SoundVolume     int    `json:"sound_volume"`
	SoundFile       string `json:"sound_file"` // empty = built-in sound

	// Text style shared by the on-screen overlay and the OBS browser source.
	FontFamily   string `json:"font_family"`
	FontBold     bool   `json:"font_bold"`
	TextColor    string `json:"text_color"` // #RRGGBB
	Outline      bool   `json:"outline"`
	OutlineWidth int    `json:"outline_width"`
	OutlineColor string `json:"outline_color"` // #RRGGBB
	ServerPort   int    `json:"server_port"`

	BossPanelEnabled  bool  `json:"boss_panel_enabled"`
	BossPanelX        int32 `json:"boss_panel_x"`
	BossPanelY        int32 `json:"boss_panel_y"`
	BossPanelPosSet   bool  `json:"boss_panel_pos_set"`
	BossPanelFontSize int   `json:"boss_panel_font_size"`

	Keywords     []string `json:"keywords"`
	OcrMinHeight float64  `json:"ocr_min_height"` // min text height, fraction of screen height
}

// DefaultKeywords are common death-screen phrases across games.
var DefaultKeywords = []string{
	"YOU DIED", "YOU ARE DEAD", "YOU HAVE DIED", "YOU WERE KILLED", "YOU DIE",
	"DEAD", "DIED", "GAME OVER", "WASTED", "DEFEATED", "YOU ARE DEFEATED",
	"YOU HAVE BEEN DEFEATED", "MISSION FAILED", "DEATH",
	// verified in the top-100 research
	"DESYNCHRONIZED", "SQUAD ELIMINATED", "KILLED IN ACTION",
	"YOU WERE SLAIN *", "YOU WERE KILLED BY *", "ELIMINATED BY *",
	// Vietnamese fan translations of "YOU DIED"
	"BẠN ĐÃ CHẾT",
}

func DefaultConfig() Config {
	return Config{
		AlwaysOnTop: true,
		IntervalMs:  400,
		Sound:       true,
		ObsFormat:   "Deaths: {count}",
		Hotkeys:     Hotkeys{Add: "F9", Sub: "F10", Capture: "F11", Toggle: "F8", Window: "F7"},

		ObsEnabled:      true,
		OverlayFontSize: 40,
		OverlayColor:    "Trắng",
		SoundVolume:     80,

		FontFamily:   "Segoe UI",
		FontBold:     true,
		TextColor:    "#FFFFFF",
		Outline:      true,
		OutlineWidth: 3,
		OutlineColor: "#000000",
		ServerPort:   17321,

		BossPanelFontSize: 18,

		Keywords:     append([]string(nil), DefaultKeywords...),
		OcrMinHeight: 0.025,
	}
}

type Profile struct {
	ID          string     `json:"-"`
	Name        string     `json:"name"`
	Deaths      int        `json:"deaths"`
	Threshold   float64    `json:"threshold"`
	CooldownSec float64    `json:"cooldown_sec"`
	AutoEnabled bool       `json:"auto_enabled"`
	OcrEnabled  bool       `json:"ocr_enabled"`
	Preset      string     `json:"preset,omitempty"`   // game preset name, "" = generic
	Keywords    []string   `json:"keywords,omitempty"` // empty = use global keywords
	OcrLang     string     `json:"ocr_lang,omitempty"` // "" = Windows user languages
	BossTrack   bool       `json:"boss_track"`
	BossROI     ROI        `json:"boss_roi"` // zero = automatic zones
	Bosses      []BossStat `json:"bosses,omitempty"`
	HPEnabled   bool       `json:"hp_enabled"`
	HPZone      ROI        `json:"hp_zone"`
	HPColor     HPColor    `json:"hp_color"`
	HPFull      float64    `json:"hp_full"`
	ROI         ROI        `json:"roi"`
	Templates   []string   `json:"templates"` // file names inside templates/
}

func NewProfile(name string) *Profile {
	return &Profile{Name: name, Threshold: 0.80, CooldownSec: 15, AutoEnabled: true, OcrEnabled: true, BossTrack: true, ROI: FullROI()}
}

type Store struct {
	Dir string
}

func (s *Store) profilesDir() string         { return filepath.Join(s.Dir, "profiles") }
func (s *Store) ProfileDir(id string) string { return filepath.Join(s.profilesDir(), id) }
func (s *Store) TemplatePath(p *Profile, name string) string {
	return filepath.Join(s.ProfileDir(p.ID), "templates", name)
}
func (s *Store) ObsPath() string     { return filepath.Join(s.Dir, "obs_deaths.txt") }
func (s *Store) HistoryPath() string { return filepath.Join(s.Dir, "history.csv") }
func (s *Store) MissLogPath() string { return filepath.Join(s.Dir, "ocr-misses.log") }

// OcrFrame is what OCR read at one moment, kept to explain a missed death.
type OcrFrame struct {
	At    time.Time
	Lines []string
}

// AppendMissLog records what OCR read before a death the app did not count on
// its own (the user pressed +1), so the keywords can be fixed later.
// ponytail: append-only, grows ~1 KB per miss; trim it if that ever matters.
func (s *Store) AppendMissLog(game string, frames []OcrFrame, now time.Time) error {
	var b strings.Builder
	fmt.Fprintf(&b, "== %s  %s\n", now.Format("2006-01-02 15:04:05"), game)
	if len(frames) == 0 {
		b.WriteString("(OCR did not run: detection or OCR is off)\n")
	}
	for _, f := range frames {
		text := strings.Join(f.Lines, " | ")
		if text == "" {
			text = "(no text)"
		}
		fmt.Fprintf(&b, "%s  %s\n", f.At.Format("15:04:05.0"), text)
	}
	f, err := os.OpenFile(s.MissLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(b.String())
	return err
}

// writeAtomic writes via temp file + rename so OBS never reads a half-written file.
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		// Rename can fail on Windows if another process holds the file; fall back.
		os.Remove(tmp)
		return os.WriteFile(path, data, 0o644)
	}
	return nil
}

func (s *Store) LoadConfig() Config {
	c := DefaultConfig()
	hasTextColor := false
	if b, err := os.ReadFile(filepath.Join(s.Dir, "config.json")); err == nil {
		_ = json.Unmarshal(b, &c)
		var raw map[string]json.RawMessage
		if json.Unmarshal(b, &raw) == nil {
			_, hasTextColor = raw["text_color"]
		}
	}
	d := DefaultConfig()
	if c.OverlayFontSize < 10 || c.OverlayFontSize > 300 {
		c.OverlayFontSize = d.OverlayFontSize
	}
	if _, ok := OverlayColors[c.OverlayColor]; !ok {
		c.OverlayColor = d.OverlayColor
	}
	if !hasTextColor { // migrate the old named color from v0.2-v0.4
		if rgb, ok := OverlayColors[c.OverlayColor]; ok {
			c.TextColor = rgb.Hex()
		}
	}
	if _, err := ParseHex(c.TextColor); err != nil {
		c.TextColor = d.TextColor
	}
	if _, err := ParseHex(c.OutlineColor); err != nil {
		c.OutlineColor = d.OutlineColor
	}
	if strings.TrimSpace(c.FontFamily) == "" {
		c.FontFamily = d.FontFamily
	}
	if c.OutlineWidth < 0 || c.OutlineWidth > 30 {
		c.OutlineWidth = d.OutlineWidth
	}
	if c.BossPanelFontSize < 10 || c.BossPanelFontSize > 80 {
		c.BossPanelFontSize = d.BossPanelFontSize
	}
	if c.ServerPort < 1024 || c.ServerPort > 65000 {
		c.ServerPort = d.ServerPort
	}
	c.Keywords = UpgradeKeywords(c.Keywords, DefaultKeywords)
	if len(c.Keywords) == 0 {
		c.Keywords = append([]string(nil), DefaultKeywords...)
	}
	if c.OcrMinHeight <= 0 || c.OcrMinHeight > 0.5 {
		c.OcrMinHeight = d.OcrMinHeight
	}
	if c.SoundVolume < 0 || c.SoundVolume > 100 {
		c.SoundVolume = d.SoundVolume
	}
	if c.IntervalMs < 100 {
		c.IntervalMs = d.IntervalMs
	}
	if c.Hotkeys.Add == "" {
		c.Hotkeys.Add = d.Hotkeys.Add
	}
	if c.Hotkeys.Sub == "" {
		c.Hotkeys.Sub = d.Hotkeys.Sub
	}
	if c.Hotkeys.Capture == "" {
		c.Hotkeys.Capture = d.Hotkeys.Capture
	}
	if c.Hotkeys.Toggle == "" {
		c.Hotkeys.Toggle = d.Hotkeys.Toggle
	}
	if c.Hotkeys.Window == "" {
		c.Hotkeys.Window = d.Hotkeys.Window
	}
	return c
}

func (s *Store) SaveConfig(c Config) error {
	b, _ := json.MarshalIndent(c, "", "  ")
	return writeAtomic(filepath.Join(s.Dir, "config.json"), b)
}

var nonID = regexp.MustCompile(`[^a-z0-9]+`)

func slug(name string) string {
	s := nonID.ReplaceAllString(strings.ToLower(name), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "game"
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

func (s *Store) LoadProfiles() ([]*Profile, error) {
	entries, err := os.ReadDir(s.profilesDir())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var out []*Profile
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.profilesDir(), e.Name(), "profile.json"))
		if err != nil {
			continue
		}
		p := NewProfile(e.Name())
		if json.Unmarshal(b, p) != nil {
			continue
		}
		p.ID = e.Name()
		if p.Threshold <= 0 || p.Threshold > 1 {
			p.Threshold = 0.80
		}
		if p.CooldownSec < 1 {
			p.CooldownSec = 15
		}
		p.ROI = p.ROI.Normalized()
		// Drop references to missing template files.
		kept := p.Templates[:0]
		for _, t := range p.Templates {
			if _, err := os.Stat(s.TemplatePath(p, t)); err == nil {
				kept = append(kept, t)
			}
		}
		p.Templates = kept
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (s *Store) CreateProfile(name string) (*Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("tên game trống")
	}
	base := slug(name)
	id := base
	for i := 2; ; i++ {
		if _, err := os.Stat(s.ProfileDir(id)); errors.Is(err, os.ErrNotExist) {
			break
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
	p := NewProfile(name)
	p.ID = id
	return p, s.SaveProfile(p)
}

func (s *Store) SaveProfile(p *Profile) error {
	b, _ := json.MarshalIndent(p, "", "  ")
	return writeAtomic(filepath.Join(s.ProfileDir(p.ID), "profile.json"), b)
}

func (s *Store) DeleteProfile(p *Profile) error {
	return os.RemoveAll(s.ProfileDir(p.ID))
}

// AddTemplate saves img as a new template; if the profile is full the oldest is dropped.
func (s *Store) AddTemplate(p *Profile, img image.Image, now time.Time) (string, error) {
	name := fmt.Sprintf("death_%s_%03d.png", now.Format("20060102_150405"), now.Nanosecond()/1e6)
	path := s.TemplatePath(p, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	p.Templates = append(p.Templates, name)
	for len(p.Templates) > MaxTemplates {
		os.Remove(s.TemplatePath(p, p.Templates[0]))
		p.Templates = p.Templates[1:]
	}
	return name, s.SaveProfile(p)
}

func (s *Store) RemoveTemplate(p *Profile, idx int) error {
	if idx < 0 || idx >= len(p.Templates) {
		return nil
	}
	os.Remove(s.TemplatePath(p, p.Templates[idx]))
	p.Templates = append(p.Templates[:idx], p.Templates[idx+1:]...)
	return s.SaveProfile(p)
}

func (s *Store) LoadTemplateImage(p *Profile, name string) (image.Image, error) {
	f, err := os.Open(s.TemplatePath(p, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func FormatObs(format, game string, count int) string {
	if strings.TrimSpace(format) == "" {
		format = "{count}"
	}
	r := strings.NewReplacer("{count}", fmt.Sprint(count), "{game}", game)
	return r.Replace(format)
}

func (s *Store) WriteObs(format, game string, count int) error {
	return writeAtomic(s.ObsPath(), []byte(FormatObs(format, game, count)))
}

// AppendHistory logs every change: time, game, delta, total, source.
func (s *Store) AppendHistory(now time.Time, game string, delta, total int, source string) {
	path := s.HistoryPath()
	newFile := false
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		newFile = true
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if newFile {
		f.WriteString("time,game,delta,total,source\n")
	}
	g := strings.ReplaceAll(game, `"`, `""`)
	fmt.Fprintf(f, "%s,\"%s\",%+d,%d,%s\n", now.Format("2006-01-02 15:04:05"), g, delta, total, source)
}

// ---------- Hotkey parsing ----------

const (
	ModAlt      = 0x0001
	ModControl  = 0x0002
	ModShift    = 0x0004
	ModWin      = 0x0008
	ModNoRepeat = 0x4000
)

var namedKeys = map[string]uint32{
	"PAUSE": 0x13, "SCROLLLOCK": 0x91, "INSERT": 0x2D, "DELETE": 0x2E,
	"HOME": 0x24, "END": 0x23, "PAGEUP": 0x21, "PAGEDOWN": 0x22,
	"SPACE": 0x20, "TAB": 0x09, "BACKSPACE": 0x08,
	"UP": 0x26, "DOWN": 0x28, "LEFT": 0x25, "RIGHT": 0x27,
	"NUMADD": 0x6B, "NUMSUBTRACT": 0x6D, "NUMMULTIPLY": 0x6A, "NUMDIVIDE": 0x6F, "NUMDECIMAL": 0x6E,
	"PLUS": 0xBB, "MINUS": 0xBD,
}

// ParseHotkey parses strings like "F9", "Ctrl+Shift+D", "Alt+Num5".
func ParseHotkey(s string) (mods, vk uint32, err error) {
	parts := strings.Split(strings.ToUpper(strings.ReplaceAll(s, " ", "")), "+")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return 0, 0, fmt.Errorf("phím tắt không hợp lệ: %q", s)
	}
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "CTRL", "CONTROL":
			mods |= ModControl
		case "ALT":
			mods |= ModAlt
		case "SHIFT":
			mods |= ModShift
		case "WIN":
			mods |= ModWin
		default:
			return 0, 0, fmt.Errorf("modifier không hợp lệ %q trong %q", m, s)
		}
	}
	k := parts[len(parts)-1]
	switch {
	case len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z', len(k) == 1 && k[0] >= '0' && k[0] <= '9':
		vk = uint32(k[0])
	case strings.HasPrefix(k, "F") && len(k) >= 2:
		var n int
		if _, e := fmt.Sscanf(k[1:], "%d", &n); e == nil && n >= 1 && n <= 24 && fmt.Sprint(n) == k[1:] {
			vk = 0x70 + uint32(n-1)
		}
	case strings.HasPrefix(k, "NUM") && len(k) == 4 && k[3] >= '0' && k[3] <= '9':
		vk = 0x60 + uint32(k[3]-'0')
	default:
		vk = namedKeys[k]
	}
	if vk == 0 {
		return 0, 0, fmt.Errorf("không nhận ra phím %q trong %q", k, s)
	}
	return mods, vk, nil
}

// ExtractFeatureSize returns raw (non-normalized) gray values of roi at w x h.
func ExtractFeatureSize(img image.Image, roi ROI, w, h int) []float64 {
	roi = roi.Normalized()
	b := img.Bounds()
	x0 := float64(b.Min.X) + roi.X*float64(b.Dx())
	y0 := float64(b.Min.Y) + roi.Y*float64(b.Dy())
	cw := roi.W * float64(b.Dx())
	ch := roi.H * float64(b.Dy())
	vals := make([]float64, w*h)
	for fy := 0; fy < h; fy++ {
		sy0 := int(y0 + float64(fy)*ch/float64(h))
		sy1 := int(y0 + float64(fy+1)*ch/float64(h))
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for fx := 0; fx < w; fx++ {
			sx0 := int(x0 + float64(fx)*cw/float64(w))
			sx1 := int(x0 + float64(fx+1)*cw/float64(w))
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var sum float64
			var n int
			for y := sy0; y < sy1 && y < b.Max.Y; y++ {
				for x := sx0; x < sx1 && x < b.Max.X; x++ {
					r, g, bb, _ := img.At(x, y).RGBA()
					sum += 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bb>>8)
					n++
				}
			}
			if n > 0 {
				vals[fy*w+fx] = sum / float64(n)
			}
		}
	}
	return vals
}

// ---------- Overlay rendering ----------

type RGB struct{ R, G, B uint8 }

// OverlayColors are the text colors offered in the UI.
var OverlayColors = map[string]RGB{
	"Trắng":      {255, 255, 255},
	"Đỏ":         {235, 40, 40},
	"Vàng":       {255, 210, 40},
	"Xanh lá":    {60, 220, 90},
	"Xanh dương": {70, 160, 255},
	"Cam":        {255, 140, 30},
}

// OverlayColorNames is the display order.
var OverlayColorNames = []string{"Trắng", "Đỏ", "Vàng", "Cam", "Xanh lá", "Xanh dương"}

// OutlineOffsets returns the pixel offsets used to stamp a round outline of radius r.
func OutlineOffsets(r int) [][2]int {
	var out [][2]int
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r+r/2 {
				out = append(out, [2]int{dx, dy})
			}
		}
	}
	return out
}

// ComposeOverlay builds a premultiplied BGRA buffer (what UpdateLayeredWindow
// wants) from two coverage masks: the text fill and the text outline.
// In edit mode a translucent dark box with a border is drawn underneath so
// the overlay is visible and grabbable even where there is no text.
func ComposeOverlay(fill, outline []uint8, w, h int, c, oc RGB, edit bool) []byte {
	out := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		fa := float64(fill[i]) / 255
		oa := float64(outline[i]) / 255
		// text (color, fa) over outline (outline color, oa), premultiplied
		a := fa + oa*(1-fa)
		r := float64(c.R)*fa + float64(oc.R)*oa*(1-fa)
		g := float64(c.G)*fa + float64(oc.G)*oa*(1-fa)
		b := float64(c.B)*fa + float64(oc.B)*oa*(1-fa)
		if edit {
			x, y := i%w, i/w
			bgA, bgR, bgG, bgB := 0.55, 30.0, 30.0, 30.0
			if x < 2 || y < 2 || x >= w-2 || y >= h-2 {
				bgA, bgR, bgG, bgB = 0.95, 255, 200, 40
			}
			// premultiplied src-over: out = src + bg*(1-a_src)
			r += bgR * bgA * (1 - a)
			g += bgG * bgA * (1 - a)
			b += bgB * bgA * (1 - a)
			a += bgA * (1 - a)
		}
		out[i*4+0] = uint8(b + 0.5)
		out[i*4+1] = uint8(g + 0.5)
		out[i*4+2] = uint8(r + 0.5)
		out[i*4+3] = uint8(a*255 + 0.5)
	}
	return out
}

// ClampToMonitors returns (x,y) if the point lies inside one of the rects,
// otherwise a default position near the top-left of the first rect.
func ClampToMonitors(x, y int32, rects [][4]int32) (int32, int32, bool) {
	for _, r := range rects { // left, top, right, bottom
		if x >= r[0] && x < r[2]-20 && y >= r[1] && y < r[3]-20 {
			return x, y, true
		}
	}
	if len(rects) == 0 {
		return 50, 50, false
	}
	return rects[0][0] + 50, rects[0][1] + 50, false
}

// DropLinesInside removes OCR lines centered inside any of rects (normalized to
// the captured monitor), so the app never reads its own overlay or window.
func DropLinesInside(lines []OcrLine, rects []ROI) []OcrLine {
	if len(rects) == 0 {
		return lines
	}
	out := lines[:0:0]
	for _, ln := range lines {
		x0, y0, x1, y1 := 1.0, 1.0, 0.0, 0.0
		for _, w := range ln.Words {
			x0, y0 = math.Min(x0, w.X), math.Min(y0, w.Y)
			x1, y1 = math.Max(x1, w.X+w.W), math.Max(y1, w.Y+w.H)
		}
		cx, cy := (x0+x1)/2, (y0+y1)/2
		inside := false
		for _, r := range rects {
			if cx >= r.X && cx <= r.X+r.W && cy >= r.Y && cy <= r.Y+r.H {
				inside = true
				break
			}
		}
		if !inside {
			out = append(out, ln)
		}
	}
	return out
}

// ---------- OCR keyword matching ----------

// OcrWord has its box normalized to 0..1 of the captured image.
type OcrWord struct {
	Text       string
	X, Y, W, H float64
}

type OcrLine struct {
	Text  string
	Words []OcrWord
}

type OcrMatch struct {
	Keyword string
	Line    string
	Box     ROI     // union of matched words, normalized
	Height  float64 // tallest matched word, fraction of image height
}

// NormalizeText uppercases and keeps only A-Z/0-9 separated by single spaces.
// Common OCR digit confusions inside words are mapped to letters.
func NormalizeText(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToUpper(s) {
		switch {
		case r >= '0' && r <= '9':
			m := map[rune]rune{'0': 'O', '1': 'I', '5': 'S', '8': 'B'}
			if c, ok := m[r]; ok {
				b.WriteRune(c)
			} else {
				b.WriteRune(r)
			}
			space = false
		case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r):
			b.WriteRune(r)
			space = false
		default:
			if !space {
				b.WriteByte(' ')
				space = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// isCJK reports whether s contains Chinese/Japanese/Korean characters,
// which OCR returns without spaces between words.
func isCJK(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}

func compact(s string) string { return strings.ReplaceAll(NormalizeText(s), " ", "") }

// latinFold maps accented capitals (all Vietnamese ones and most European ones)
// to their base letter, so "BẠN ĐÃ CHẾT" also matches OCR output that lost marks.
var latinFold = func() map[rune]rune {
	from := []rune("ÀÁẠẢÃÂẦẤẬẨẪĂẰẮẶẲẴÄÅÈÉẸẺẼÊỀẾỆỂỄËÌÍỊỈĨÎÏÒÓỌỎÕÔỒỐỘỔỖƠỜỚỢỞỠÖÙÚỤỦŨƯỪỨỰỬỮÛÜỲÝỴỶỸŸĐÇÑ")
	to := []rune("AAAAAAAAAAAAAAAAAAAEEEEEEEEEEEEIIIIIIIOOOOOOOOOOOOOOOOOOUUUUUUUUUUUUUYYYYYYDCN")
	m := make(map[rune]rune, len(from))
	for i, r := range from {
		m[r] = to[i]
	}
	return m
}()

// matchText is NormalizeText without accents on Latin letters, for keyword matching.
func matchText(s string) string {
	var b strings.Builder
	prev := rune(0)
	for _, r := range NormalizeText(s) {
		if f, ok := latinFold[r]; ok {
			r = f
		} else if unicode.Is(unicode.Mn, r) && prev >= 'A' && prev <= 'Z' {
			continue // combining accent on a Latin letter
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			c := 1
			if ra[i-1] == rb[j-1] {
				c = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+c)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// wordTolerance: short single words must match exactly ("DEAD" must not match "READ").
func wordTolerance(word string, phraseLen int) int {
	n := len(word)
	if phraseLen > 1 {
		if n >= 4 {
			return 1
		}
		return 0
	}
	switch {
	case n <= 5:
		return 0
	case n <= 8:
		return 1
	default:
		return 2
	}
}

// MatchDeath looks for a death keyword that is big, near the screen center,
// and stands (almost) alone on its line, the way death banners are drawn.
func MatchDeath(lines []OcrLine, keywords []string, minHeight float64) (OcrMatch, bool) {
	var best OcrMatch
	found := false
	for _, ln := range lines {
		// Normalize per word so indices map back to boxes.
		var words []string
		var boxes []OcrWord
		for _, w := range ln.Words {
			for _, part := range strings.Fields(matchText(w.Text)) {
				words = append(words, part)
				boxes = append(boxes, w)
			}
		}
		if len(words) == 0 {
			continue
		}
		for _, kw := range keywords {
			if isCJK(kw) {
				// CJK: compare the whole line without spaces.
				k := compact(strings.TrimSuffix(strings.TrimSpace(kw), "*"))
				l := compact(ln.Text)
				if k == "" || !strings.Contains(l, k) || len([]rune(l)) > len([]rune(k))+4 {
					continue
				}
				x0, y0, x1, y1, h := 1.0, 1.0, 0.0, 0.0, 0.0
				for _, b := range ln.Words {
					x0, y0 = math.Min(x0, b.X), math.Min(y0, b.Y)
					x1, y1 = math.Max(x1, b.X+b.W), math.Max(y1, b.Y+b.H)
					h = math.Max(h, b.H)
				}
				cx, cy := (x0+x1)/2, (y0+y1)/2
				if h < minHeight || cx < 0.2 || cx > 0.8 || cy < 0.12 || cy > 0.92 {
					continue
				}
				if !found || h > best.Height {
					best = OcrMatch{Keyword: kw, Line: ln.Text, Box: ROI{x0, y0, x1 - x0, y1 - y0}, Height: h}
					found = true
				}
				continue
			}
			prefix := strings.HasSuffix(strings.TrimSpace(kw), "*")
			kws := strings.Fields(matchText(strings.TrimSuffix(strings.TrimSpace(kw), "*")))
			if len(kws) == 0 || len(kws) > len(words) {
				continue
			}
			if prefix {
				// "YOU WERE SLAIN BY *": phrase must start the line; the variable tail
				// (killer name etc.) may be up to 8 words.
				if len(words) > len(kws)+8 {
					continue
				}
			} else if len(words) > len(kws)+2 {
				continue
			}
			for start := 0; start+len(kws) <= len(words); start++ {
				if prefix && start > 0 {
					break
				}
				ok := true
				for k, kwWord := range kws {
					if editDistance(words[start+k], kwWord) > wordTolerance(kwWord, len(kws)) {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				x0, y0, x1, y1, h := 1.0, 1.0, 0.0, 0.0, 0.0
				for k := range kws {
					b := boxes[start+k]
					x0, y0 = math.Min(x0, b.X), math.Min(y0, b.Y)
					x1, y1 = math.Max(x1, b.X+b.W), math.Max(y1, b.Y+b.H)
					h = math.Max(h, b.H)
				}
				cx, cy := (x0+x1)/2, (y0+y1)/2
				if h < minHeight || cx < 0.2 || cx > 0.8 || cy < 0.12 || cy > 0.92 {
					continue
				}
				if !found || h > best.Height {
					best = OcrMatch{Keyword: kw, Line: ln.Text, Box: ROI{x0, y0, x1 - x0, y1 - y0}, Height: h}
					found = true
				}
			}
		}
	}
	return best, found
}

// AutoROI expands a text box into a matching region for template learning.
func AutoROI(box ROI) ROI {
	px := math.Max(box.W*0.4, 0.04)
	py := math.Max(box.H*0.6, 0.03)
	return ROI{box.X - px, box.Y - py, box.W + 2*px, box.H + 2*py}.Normalized()
}

// ParseKeywords splits a comma/newline separated list, dropping empties and duplicates.
func ParseKeywords(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		p := strings.TrimSpace(part)
		n := NormalizeText(p)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, p)
	}
	return out
}

// Downscale area-averages an RGBA image to w x h.
func Downscale(src *image.RGBA, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	for y := 0; y < h; y++ {
		y0, y1 := y*sh/h, (y+1)*sh/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0, x1 := x*sw/w, (x+1)*sw/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, b, n int
			for yy := y0; yy < y1 && yy < sh; yy++ {
				i := yy*src.Stride + x0*4
				for xx := x0; xx < x1 && xx < sw; xx++ {
					r += int(src.Pix[i])
					g += int(src.Pix[i+1])
					b += int(src.Pix[i+2])
					i += 4
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			o := y*dst.Stride + x*4
			dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = uint8(r/n), uint8(g/n), uint8(b/n), 255
		}
	}
	return dst
}

// ---------- data location ----------

// MigrateLegacyData copies an old data folder into dst when dst has no profiles yet.
// The source is left untouched. Returns true if anything was copied.
func MigrateLegacyData(src, dst string) (bool, error) {
	if src == "" || src == dst {
		return false, nil
	}
	if _, err := os.Stat(filepath.Join(src, "profiles")); err != nil {
		return false, nil
	}
	if DirHasData(dst) {
		return false, nil
	}
	err := CopyDataDir(src, dst)
	return err == nil, err
}

// ---------- custom data folder ----------

// dataPointerFile lives in the default folder and holds the user's chosen folder.
const dataPointerFile = "datadir.txt"

// DirHasData reports whether dir already holds Death Counter data.
func DirHasData(dir string) bool {
	if ents, err := os.ReadDir(filepath.Join(dir, "profiles")); err == nil && len(ents) > 0 {
		return true
	}
	return false
}

// CheckWritable creates dir if needed and verifies files can be written there.
func CheckWritable(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return errors.New("đường dẫn trống")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	test := filepath.Join(dir, ".dc_write_test")
	if err := os.WriteFile(test, []byte("ok"), 0o644); err != nil {
		return err
	}
	return os.Remove(test)
}

// CopyDataDir copies every data file from src to dst (the pointer file is not copied).
func CopyDataDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if strings.HasSuffix(path, ".tmp") || rel == dataPointerFile {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

// LegacyDataDirs lists where versions before 0.9 kept data, newest first: the
// custom folder %APPDATA%\\DeathCounter\\datadir.txt pointed to, then appData itself.
func LegacyDataDirs(appData string) []string {
	var dirs []string
	if b, err := os.ReadFile(filepath.Join(appData, dataPointerFile)); err == nil {
		if custom := strings.TrimSpace(string(b)); custom != "" && filepath.Clean(custom) != filepath.Clean(appData) {
			dirs = append(dirs, filepath.Clean(custom))
		}
	}
	return append(dirs, appData)
}

// appDataFiles are the entries Death Counter writes into a data folder.
var appDataFiles = []string{"profiles", "config.json", "obs_deaths.txt", "history.csv", "ocr-misses.log", dataPointerFile}

// RemoveAppData deletes Death Counter's own files from dir and then dir itself if
// it is left empty, so a folder shared with other files keeps them.
func RemoveAppData(dir string) error {
	var firstErr error
	for _, name := range appDataFiles {
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	tmps, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	for _, t := range tmps {
		os.Remove(t)
	}
	os.Remove(dir) // fails harmlessly when other files remain
	return firstErr
}

// ---------- colors ----------

func (c RGB) Hex() string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

// ParseHex accepts #RGB, #RRGGBB (with or without #).
func ParseHex(s string) (RGB, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return RGB{}, fmt.Errorf("màu không hợp lệ: %q", s)
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); err != nil {
		return RGB{}, fmt.Errorf("màu không hợp lệ: %q", s)
	}
	return RGB{r, g, b}, nil
}

func MustHex(s string, def RGB) RGB {
	if c, err := ParseHex(s); err == nil {
		return c
	}
	return def
}

// EffectiveKeywords returns the profile's own keywords, or the global list.
// UpgradeKeywords brings a saved copy of a built-in list up to date. A list the
// user added their own keywords to is left alone.
func UpgradeKeywords(have, latest []string) []string {
	known := make(map[string]bool, len(latest))
	for _, k := range latest {
		known[k] = true
	}
	for _, k := range have {
		if !known[k] {
			return have
		}
	}
	if len(have) == 0 || len(have) == len(latest) {
		return have
	}
	return append([]string(nil), latest...)
}

func EffectiveKeywords(p *Profile, global []string) []string {
	if p != nil && len(p.Keywords) > 0 {
		return p.Keywords
	}
	return global
}
