package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOverlayServer(t *testing.T) {
	s, err := StartOverlayServer(17321)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !strings.HasPrefix(s.URL(), "http://127.0.0.1:") {
		t.Fatal(s.URL())
	}
	// second server on the same base port must pick another port, not fail
	s2, err := StartOverlayServer(17321)
	if err != nil || s2.URL() == s.URL() {
		t.Fatalf("port fallback: %v %s", err, s2.URL())
	}
	s2.Close()

	c := DefaultConfig()
	c.LabelText = "{game} deaths"
	s.Set(StateFromConfig(c, "Elden Ring", 7, 1))
	r, err := http.Get(s.URL() + "state")
	if err != nil {
		t.Fatal(err)
	}
	var st OverlayState
	json.NewDecoder(r.Body).Decode(&st)
	r.Body.Close()
	if st.Count != "7" || st.Label != "Elden Ring deaths" || st.LabelImg != "" || st.Corner != CornerTL ||
		st.Color != "#FFFFFF" || !st.Outline || st.Version != 1 {
		t.Fatalf("%+v", st)
	}
	r, _ = http.Get(s.URL())
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	// fill must paint above the outline layer
	if !strings.Contains(string(b), ".t .f{position:relative;z-index:1}") || !strings.Contains(string(b), ".t .o{position:absolute;z-index:0") {
		t.Fatal("outline must be under the fill")
	}
	if !strings.Contains(string(b), "fetch('/state'") || !strings.Contains(r.Header.Get("Content-Type"), "text/html") {
		t.Fatal("page wrong")
	}
	if r, _ := http.Get(s.URL() + "nope"); r.StatusCode != 404 {
		t.Fatal("expected 404")
	}
	if r, _ := http.Get(s.URL() + "label"); r.StatusCode != 404 {
		t.Fatal("no picture yet: expected 404")
	}
}

func TestOverlayServerLabelPicture(t *testing.T) {
	s, err := StartOverlayServer(17341)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p := filepath.Join(t.TempDir(), "label.svg")
	os.WriteFile(p, []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"/>`), 0o644)
	s.SetLabelImage(p)
	c := DefaultConfig()
	c.LabelMode, c.LabelImage = LabelModeImage, "label.svg"
	st := StateFromConfig(c, "", 3, 4)
	if st.LabelImg != "/label?v=4" || st.Label != "" {
		t.Fatalf("%+v", st)
	}
	r, err := http.Get(s.URL() + "label?v=4")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.Header.Get("Content-Type") != "image/svg+xml" || !strings.Contains(string(b), "<svg") {
		t.Fatalf("%s %s", r.Header.Get("Content-Type"), b)
	}
}
