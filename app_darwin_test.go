package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newWebApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	a := newApp(t.TempDir())
	a.monitors = []Monitor{{Rect: PxRect{R: 1440, B: 900}, Primary: true}}
	srv, err := StartOverlayServer(a.cfg.ServerPort + 200)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	a.server = srv
	a.registerRoutes(srv)
	a.loadProfiles()
	return a, srv.mux
}

func post(t *testing.T, h http.Handler, a *App, body any) int {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", "http://127.0.0.1:1/api/do", bytes.NewReader(b))
	r.Header.Set("X-Token", a.token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

func TestWebAPICountsAndManagesGames(t *testing.T) {
	a, h := newWebApp(t)
	post(t, h, a, map[string]any{"do": "add"})
	post(t, h, a, map[string]any{"do": "add"})
	post(t, h, a, map[string]any{"do": "sub"})
	if a.cur.Deaths != 1 {
		t.Fatalf("deaths %d", a.cur.Deaths)
	}
	post(t, h, a, map[string]any{"do": "profile-add", "s": "Dark Souls Remastered"})
	if a.cur.Name != "Dark Souls Remastered" || a.cur.Preset == "" || a.cur.Deaths != 0 {
		t.Fatalf("new game %+v", a.cur)
	}
	post(t, h, a, map[string]any{"do": "keywords", "s": "NGÃ RỒI, YOU DIED"})
	if len(a.cur.Keywords) != 2 {
		t.Fatalf("keywords %v", a.cur.Keywords)
	}
	post(t, h, a, map[string]any{"do": "profile-delete"})
	if len(a.profiles) != 1 {
		t.Fatalf("profiles %d", len(a.profiles))
	}
	post(t, h, a, map[string]any{"do": "profile-delete"})
	if len(a.profiles) != 1 {
		t.Fatal("the last game must stay")
	}
}

func TestWebAPIRejectsOtherPages(t *testing.T) {
	a, h := newWebApp(t)
	r := httptest.NewRequest("POST", "http://127.0.0.1:1/api/do", strings.NewReader(`{"do":"add"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden || a.cur.Deaths != 0 {
		t.Fatalf("code %d deaths %d", w.Code, a.cur.Deaths)
	}
}

func TestStateNeverContainsTheAPIKey(t *testing.T) {
	a, h := newWebApp(t)
	a.translation.apiKey = "sk-ant-secret-value"
	r := httptest.NewRequest("GET", "http://127.0.0.1:1/api/state?t="+a.token, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret-value") || !strings.Contains(w.Body.String(), `"hasKey":true`) {
		t.Fatalf("code %d body %s", w.Code, w.Body.String())
	}
}

func TestTranslateNeedsAKeyFirst(t *testing.T) {
	a, h := newWebApp(t)
	post(t, h, a, map[string]any{"do": "translate", "b": true})
	if a.translation.on || !strings.Contains(a.status, "API key") {
		t.Fatalf("on=%v status=%q", a.translation.on, a.status)
	}
}

// Manual helper: BW_SERVE=1 go test -run TestServeAppUI, then open the printed URL.
func TestServeAppUI(t *testing.T) {
	if os.Getenv("BW_SERVE") == "" {
		t.Skip("manual")
	}
	a, _ := newWebApp(t)
	t.Log(a.server.URL() + "app?t=" + a.token)
	os.Stderr.WriteString(a.server.URL() + "app?t=" + a.token + "\n")
	select {}
}

func TestVisionReadsDialogueAndLocatesWords(t *testing.T) {
	f, err := os.Open("testdata/dialogue.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	e, err := NewOcrEngine("en-US")
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Langs) == 0 {
		t.Fatal("Vision lists no languages")
	}
	lines, err := e.Recognize(rgba)
	if err != nil {
		t.Fatal(err)
	}
	var all []string
	for _, l := range lines {
		all = append(all, l.Text)
		for _, w := range l.Words {
			if w.X < 0 || w.Y < 0 || w.X+w.W > 1.001 || w.Y+w.H > 1.001 || w.W <= 0 || w.H <= 0 {
				t.Errorf("word %q has a box outside the frame: %+v", w.Text, w)
			}
		}
	}
	if text := strings.Join(all, " "); !hasWords(text) || len(lines) == 0 {
		t.Fatalf("read %q", text)
	} else {
		t.Log(text)
	}
}

// Runs only where this process may record the screen, so a CI runner skips it.
func TestCaptureRealScreenThenOcr(t *testing.T) {
	if !screenAccessGranted() {
		t.Skip("no Screen Recording permission")
	}
	img, err := CaptureRegion(PxRect{L: 0, T: 0, R: 1400, B: 850})
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() < 800 || b.Dy() < 480 {
		t.Fatalf("frame %v", b)
	}
	if IsBlack(Downscale(img, CaptureW, CaptureH)) {
		t.Log("the captured frame is black: nothing on screen, or the capture failed silently")
	}
	e, _ := NewOcrEngine("en-US")
	lines, err := e.Recognize(img)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("read %d lines from the screen", len(lines))
}

func TestBossListReachesThePageWithTheNamesItReads(t *testing.T) {
	a, h := newWebApp(t)
	AddBossDeath(a.cur, "Asylum Demon", 2)
	r := httptest.NewRequest("GET", "http://127.0.0.1:1/api/state?t="+a.token, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), `"name":"Asylum Demon"`) || !strings.Contains(w.Body.String(), `"deaths":2`) {
		t.Fatalf("body %s", w.Body.String())
	}
	if !strings.Contains(appPage, "x.deaths + '   ' + x.name") {
		t.Fatal("the page must read the same field names")
	}
}

func TestUninstallLeavesNoConfigBehind(t *testing.T) {
	a := newApp(t.TempDir())
	theApp = a
	defer func() { theApp = nil }()
	a.uninstalling = true
	os.RemoveAll(a.store.Dir)
	appWillQuit()
	if _, err := os.Stat(a.store.Dir); err == nil {
		t.Fatal("quitting recreated the data folder")
	}
}
