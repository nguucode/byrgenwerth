package main

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLengthScale(t *testing.T) {
	if LengthScale(0) != 1 || LengthScale(100) != 0.5 || LengthScale(-50) != 2 || LengthScale(500) != 0.5 {
		t.Fatal(LengthScale(0), LengthScale(100), LengthScale(-50))
	}
}

func TestNarrateTextJoinsLinesAndCutsAtSentence(t *testing.T) {
	if got := NarrateText("Xin chào\nbạn   nhé "); got != "Xin chào bạn nhé" {
		t.Fatalf("%q", got)
	}
	got := NarrateText(strings.Repeat("Một câu dài. ", 300))
	if len([]rune(got)) > MaxNarrateChars || !strings.HasSuffix(got, ".") {
		t.Fatalf("len %d", len([]rune(got)))
	}
}

func wav16(samples ...int16) []byte {
	var b []byte
	b = append(b, "RIFF"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(36+2*len(samples)))
	b = append(b, "WAVEfmt "...)
	b = binary.LittleEndian.AppendUint32(b, 16)
	b = binary.LittleEndian.AppendUint16(b, 1)     // PCM
	b = binary.LittleEndian.AppendUint16(b, 1)     // mono
	b = binary.LittleEndian.AppendUint32(b, 22050) // rate
	b = binary.LittleEndian.AppendUint32(b, 44100)
	b = binary.LittleEndian.AppendUint16(b, 2)
	b = binary.LittleEndian.AppendUint16(b, 16)
	b = append(b, "data"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(2*len(samples)))
	for _, s := range samples {
		b = binary.LittleEndian.AppendUint16(b, uint16(s))
	}
	return b
}

func TestScaleWavVolume(t *testing.T) {
	w := wav16(1000, -1000, 32767)
	if err := ScaleWavVolume(w, 50); err != nil {
		t.Fatal(err)
	}
	got := func(i int) int16 { return int16(binary.LittleEndian.Uint16(w[44+2*i:])) }
	if got(0) != 500 || got(1) != -500 || got(2) != 16383 {
		t.Fatal(got(0), got(1), got(2))
	}
	full := wav16(1234)
	ScaleWavVolume(full, 100)
	if int16(binary.LittleEndian.Uint16(full[44:])) != 1234 {
		t.Fatal("100% must not change anything")
	}
	if ScaleWavVolume([]byte("nope"), 10) == nil {
		t.Fatal("not a wav")
	}
}

func TestVoicesArePinned(t *testing.T) {
	for _, v := range Voices {
		pv := piperVoices[v]
		for _, f := range []piperFile{pv.Model, pv.Config} {
			if len(f.SHA256) != 64 || f.Size <= 0 || len(f.URLs) == 0 || !strings.HasPrefix(f.URLs[0], "https://huggingface.co/rhasspy/piper-voices/") {
				t.Fatalf("%s: %+v", v, f)
			}
		}
	}
	if piperVoices[VoiceMale].Speaker < 0 || piperVoices[VoiceFemale].Speaker != -1 {
		t.Fatal("speakers")
	}
}

func TestDownloadChecksHash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/broken" {
			http.Error(w, "no", http.StatusGatewayTimeout)
			return
		}
		w.Write([]byte("hello"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	good := piperFile{URLs: []string{srv.URL + "/broken", srv.URL + "/ok"}, Name: "a.txt",
		SHA256: "2CF24DBA5FB0A30E26E83B2AC5B9E29E1B161E5C1FA7425E73043362938B9824", Size: 5}
	if err := download(context.Background(), good, filepath.Join(dir, "a.txt"), func(string) {}); err != nil {
		t.Fatal("second address must be tried:", err)
	}
	bad := good
	bad.SHA256 = strings.Repeat("0", 64)
	if err := download(context.Background(), bad, filepath.Join(dir, "b.txt"), func(string) {}); err == nil {
		t.Fatal("wrong hash must fail")
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err == nil {
		t.Fatal("a file with the wrong hash must not stay")
	}
}

func TestUnzipRefusesPathsOutside(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string) string {
		p := filepath.Join(dir, strings.ReplaceAll(name, "/", "_")+".zip")
		f, _ := os.Create(p)
		z := zip.NewWriter(f)
		w, _ := z.Create(name)
		w.Write([]byte("x"))
		z.Close()
		f.Close()
		return p
	}
	out := filepath.Join(dir, "out")
	if err := unzip(mk("piper/piper.exe"), out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "piper", "piper.exe")); err != nil {
		t.Fatal(err)
	}
	if err := unzip(mk("../evil.txt"), out); err == nil {
		t.Fatal("zip slip must be refused")
	}
}
