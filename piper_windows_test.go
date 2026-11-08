//go:build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Live: BYR_PIPER=<folder> go test -run TestPiperLive -v
// Downloads Piper and both voices into the folder (about 110 MB the first
// time) and writes one WAV per voice there.
func TestPiperLive(t *testing.T) {
	dir := os.Getenv("BYR_PIPER")
	if dir == "" {
		t.Skip("live")
	}
	for _, v := range Voices {
		if err := EnsurePiper(context.Background(), dir, v, func(s string) { t.Log(s) }); err != nil {
			t.Fatal(v, err)
		}
		out := filepath.Join(dir, "sample-"+v+".wav")
		if err := PiperSpeak(context.Background(), dir, v, "Xin chào, đây là giọng thuyết minh của Byrgenwerth. Bạn đã chết rồi!", 0, 80, out); err != nil {
			t.Fatal(v, err)
		}
		fi, err := os.Stat(out)
		if err != nil || fi.Size() < 20000 {
			t.Fatal(v, err, fi)
		}
		t.Logf("%s: %d bytes", v, fi.Size())
	}
}
