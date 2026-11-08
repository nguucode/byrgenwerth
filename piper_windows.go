//go:build windows

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const createNoWindow = 0x08000000

// PiperSpeak turns text into a WAV file with the chosen voice. The engine and
// voice must already be downloaded (EnsurePiper).
func PiperSpeak(ctx context.Context, dir, voice, text string, rate, volume int, out string) error {
	v, ok := piperVoices[voice]
	if !ok {
		return fmt.Errorf("không có giọng %q", voice)
	}
	text = NarrateText(text)
	if text == "" {
		return fmt.Errorf("không có chữ để đọc")
	}
	args := []string{"--model", filepath.Join(dir, v.Model.Name), "--output_file", out,
		"--length_scale", fmt.Sprintf("%.3f", LengthScale(rate))}
	if v.Speaker >= 0 {
		args = append(args, "--speaker", fmt.Sprint(v.Speaker))
	}
	cmd := exec.CommandContext(ctx, piperExe(dir), args...)
	cmd.Dir = filepath.Dir(piperExe(dir)) // espeak-ng-data sits next to the exe
	cmd.Stdin = strings.NewReader(text + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if i := strings.LastIndex(msg, "\n"); i >= 0 {
			msg = msg[i+1:]
		}
		return fmt.Errorf("Piper lỗi: %v %s", err, msg)
	}
	wav, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	if err := ScaleWavVolume(wav, volume); err != nil {
		return err
	}
	return os.WriteFile(out, wav, 0o644)
}
