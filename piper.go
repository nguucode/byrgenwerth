package main

// Vietnamese narration with Piper (github.com/rhasspy/piper), a text-to-speech
// engine that runs on the PC. The engine and the chosen voice are downloaded
// once into the data folder, checked against the hashes below.

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	VoiceFemale = "female"
	VoiceMale   = "male"

	// MaxNarrateChars keeps one reading short enough to start quickly.
	MaxNarrateChars = 2000
)

// piperFile is one download with the hash it must have.
type piperFile struct {
	URLs   []string // tried in order
	Name   string   // file name inside the piper folder
	SHA256 string
	Size   int64
}

// The Windows build of Piper 2023.11.14-2. The API address works when GitHub's
// download servers do not (they answered 504 for days in November 2026).
var piperEngine = piperFile{
	URLs: []string{
		"https://api.github.com/repos/rhasspy/piper/releases/assets/135529797",
		"https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_windows_amd64.zip",
	},
	Name:   "piper_windows_amd64.zip",
	SHA256: "F3C58906402B24F3A96D92145F58ACBA6D86C9B5DB896D207F78DC80811EFCEA",
	Size:   22477236,
}

// piperVoice is one of the two narration voices.
type piperVoice struct {
	Label   string
	Model   piperFile
	Config  piperFile
	Speaker int // -1 for single-speaker models
}

const hfVoices = "https://huggingface.co/rhasspy/piper-voices/resolve/main/vi/vi_VN/"

// Piper has two single-speaker Vietnamese voices and both are women, so the
// man's voice is one speaker of the 65 in "vivos", picked for a low pitch.
var piperVoices = map[string]piperVoice{
	VoiceFemale: {
		Label: "Nữ (vais1000)",
		Model: piperFile{URLs: []string{hfVoices + "vais1000/medium/vi_VN-vais1000-medium.onnx"}, Name: "vi_VN-vais1000-medium.onnx",
			SHA256: "EC7C89E2C85F4D1EDC24B6120C18AAF1BDA614F06B511567EB9C7C0DE15E2DAB", Size: 63201294},
		Config: piperFile{URLs: []string{hfVoices + "vais1000/medium/vi_VN-vais1000-medium.onnx.json"}, Name: "vi_VN-vais1000-medium.onnx.json",
			SHA256: "FAFB9DA1354ED4B77C31AF228ED41FB41CD825C14CFFA105454B25E6AE751EE0", Size: 4860},
		Speaker: -1,
	},
	VoiceMale: {
		Label: "Nam (vivos)",
		Model: piperFile{URLs: []string{hfVoices + "vivos/x_low/vi_VN-vivos-x_low.onnx"}, Name: "vi_VN-vivos-x_low.onnx",
			SHA256: "6AB13374EB0862021A545BEFE7727AEF59E16117F1C075AA9E0362237ECC98AE", Size: 27789413},
		Config: piperFile{URLs: []string{hfVoices + "vivos/x_low/vi_VN-vivos-x_low.onnx.json"}, Name: "vi_VN-vivos-x_low.onnx.json",
			SHA256: "9DC373D69B0E4F39D864CB8AC9FE42CEE49817A0F02D831E57A9CE6E944E234A", Size: 5592},
		Speaker: 45,
	},
}

// VoiceName is how a voice reads in the UI.
func VoiceName(v string) string {
	if pv, ok := piperVoices[v]; ok {
		return pv.Label
	}
	return v
}

// Voices is the order the voices are offered in.
var Voices = []string{VoiceFemale, VoiceMale}

// PiperDir is where the engine and voices live inside the data folder.
func PiperDir(dataDir string) string { return filepath.Join(dataDir, "piper") }

func piperExe(dir string) string { return filepath.Join(dir, "piper", "piper.exe") }

// PiperReady reports whether the engine and the voice are already downloaded.
func PiperReady(dir, voice string) bool {
	v, ok := piperVoices[voice]
	if !ok {
		return false
	}
	for _, p := range []string{piperExe(dir), filepath.Join(dir, v.Model.Name), filepath.Join(dir, v.Config.Name)} {
		if _, err := os.Stat(p); err != nil {
			return false
		}
	}
	return true
}

// PiperDownloadSize is how many bytes are still missing for the voice.
func PiperDownloadSize(dir, voice string) int64 {
	var n int64
	if _, err := os.Stat(piperExe(dir)); err != nil {
		n += piperEngine.Size
	}
	v := piperVoices[voice]
	for _, f := range []piperFile{v.Model, v.Config} {
		if _, err := os.Stat(filepath.Join(dir, f.Name)); err != nil {
			n += f.Size
		}
	}
	return n
}

// EnsurePiper downloads what the voice still needs. progress gets a short
// line for the status bar.
func EnsurePiper(ctx context.Context, dir, voice string, progress func(string)) error {
	v, ok := piperVoices[voice]
	if !ok {
		return fmt.Errorf("không có giọng %q", voice)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(piperExe(dir)); err != nil {
		zipPath := filepath.Join(dir, piperEngine.Name)
		if err := download(ctx, piperEngine, zipPath, progress); err != nil {
			return err
		}
		progress("Đang giải nén Piper…")
		if err := unzip(zipPath, dir); err != nil {
			return fmt.Errorf("giải nén Piper: %w", err)
		}
		os.Remove(zipPath)
	}
	for _, f := range []piperFile{v.Config, v.Model} {
		p := filepath.Join(dir, f.Name)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if err := download(ctx, f, p, progress); err != nil {
			return err
		}
	}
	return nil
}

// download fetches f to path through a temp file and keeps it only when the
// SHA-256 matches.
func download(ctx context.Context, f piperFile, path string, progress func(string)) error {
	var lastErr error
	for _, url := range f.URLs {
		lastErr = downloadOne(ctx, url, f, path, progress)
		if lastErr == nil || ctx.Err() != nil {
			return lastErr
		}
	}
	return lastErr
}

func downloadOne(ctx context.Context, url string, f piperFile, path string, progress func(string)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Byrgenwerth/"+version)
	if strings.HasPrefix(url, "https://api.github.com/") {
		req.Header.Set("Accept", "application/octet-stream")
	}
	resp, err := (&http.Client{Timeout: 15 * time.Minute}).Do(req)
	if err != nil {
		return fmt.Errorf("tải %s: %w", f.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tải %s: máy chủ trả về %s", f.Name, resp.Status)
	}
	tmp := path + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	var done int64
	buf := make([]byte, 256<<10)
	lastShown := time.Time{}
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				os.Remove(tmp)
				return werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if time.Since(lastShown) > 300*time.Millisecond {
				lastShown = time.Now()
				progress(fmt.Sprintf("Đang tải %s: %d/%d MB", f.Name, done>>20, max(f.Size>>20, 1)))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			os.Remove(tmp)
			return fmt.Errorf("tải %s: %w", f.Name, rerr)
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if got := strings.ToUpper(hex.EncodeToString(h.Sum(nil))); got != f.SHA256 {
		os.Remove(tmp)
		return fmt.Errorf("file %s tải về không đúng (sai mã kiểm tra), đã xóa", f.Name)
	}
	return os.Rename(tmp, path)
}

// unzip extracts every file of the archive under dst, refusing paths that
// would land outside it.
func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	root := filepath.Clean(dst) + string(os.PathSeparator)
	for _, f := range r.File {
		target := filepath.Join(dst, f.Name)
		if !strings.HasPrefix(target, root) {
			return fmt.Errorf("đường dẫn lạ trong file nén: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// NarrateText tidies what will be read: OCR line breaks become spaces and very
// long text is cut at a sentence end.
func NarrateText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= MaxNarrateChars {
		return s
	}
	cut := string(r[:MaxNarrateChars])
	if i := strings.LastIndexAny(cut, ".!?…"); i > len(cut)/2 {
		cut = cut[:i+1]
	}
	return cut
}

// LengthScale is Piper's --length_scale for a speed in percent: +100 reads
// twice as fast, -50 half as fast.
func LengthScale(rate int) float64 {
	rate = max(-50, min(rate, 100))
	return 1 / (1 + float64(rate)/100)
}

// ScaleWavVolume lowers the volume of a 16-bit PCM WAV in place (volume 0..100).
// Windows' MCI cannot change the volume of WAV playback by itself.
func ScaleWavVolume(wav []byte, volume int) error {
	if volume >= 100 {
		return nil
	}
	if len(wav) < 12 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return errors.New("không phải file WAV")
	}
	bits := 0
	for i := 12; i+8 <= len(wav); {
		id, size := string(wav[i:i+4]), int(binary.LittleEndian.Uint32(wav[i+4:i+8]))
		body := i + 8
		end := min(body+size, len(wav))
		switch id {
		case "fmt ":
			if size >= 16 {
				bits = int(binary.LittleEndian.Uint16(wav[body+14 : body+16]))
			}
		case "data":
			if bits != 16 {
				return errors.New("chỉ chỉnh được âm lượng WAV 16-bit")
			}
			f := float64(max(volume, 0)) / 100
			for j := body; j+1 < end; j += 2 {
				s := float64(int16(binary.LittleEndian.Uint16(wav[j:])))
				binary.LittleEndian.PutUint16(wav[j:], uint16(int16(s*f)))
			}
			return nil
		}
		i = body + size + size%2
	}
	return errors.New("file WAV không có dữ liệu âm thanh")
}
