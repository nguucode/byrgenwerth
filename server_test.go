package main

import (
	"encoding/json"
	"io"
	"net/http"
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
	s.Set(StateFromConfig(c, "Deaths: 7"))
	r, err := http.Get(s.URL() + "state")
	if err != nil {
		t.Fatal(err)
	}
	var st OverlayState
	json.NewDecoder(r.Body).Decode(&st)
	r.Body.Close()
	if st.Text != "Deaths: 7" || st.Color != "#FFFFFF" || !st.Outline || st.Version != 1 {
		t.Fatalf("%+v", st)
	}
	r, _ = http.Get(s.URL())
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	// fill must paint above the outline layer
	if !strings.Contains(string(b), "#f{position:relative;z-index:1}") || !strings.Contains(string(b), "#o{position:absolute;z-index:0") {
		t.Fatal("outline must be under the fill")
	}
	if !strings.Contains(string(b), "fetch('/state'") || !strings.Contains(r.Header.Get("Content-Type"), "text/html") {
		t.Fatal("page wrong")
	}
	if r, _ := http.Get(s.URL() + "nope"); r.StatusCode != 404 {
		t.Fatal("expected 404")
	}
}

func TestBossEndpoint(t *testing.T) {
	s, err := StartOverlayServer(17400)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, _ := http.Get(s.BossURL() + "/state")
	var st SplitsState
	json.NewDecoder(r.Body).Decode(&st)
	r.Body.Close()
	if st.Rows == nil || len(st.Rows) != 0 {
		t.Fatal("empty state must have rows: []")
	}
	p := &Profile{Bosses: []BossStat{{"Margit, the Fell Omen", 23}, {"Godrick the Grafted", 11}}}
	s.SetSplits(SplitsState{Title: "Elden Ring", Rows: BuildSplits(p, "Godrick the Grafted", 3), Total: 41})
	r, _ = http.Get(s.BossURL() + "/state")
	json.NewDecoder(r.Body).Decode(&st)
	r.Body.Close()
	if len(st.Rows) != 2 || !st.Rows[1].Active || st.Total != 41 || st.Version != 1 {
		t.Fatalf("%+v", st)
	}
	r, _ = http.Get(s.BossURL())
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if !strings.Contains(string(b), "/bosses/state") {
		t.Fatal("boss page wrong")
	}
}
