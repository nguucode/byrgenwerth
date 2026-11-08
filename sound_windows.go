//go:build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	winmm              = syscall.NewLazyDLL("winmm.dll")
	procMciSendStringW = winmm.NewProc("mciSendStringW")
	procMciGetErrorW   = winmm.NewProc("mciGetErrorStringW")
)

func mci(cmd string) error {
	p, _ := syscall.UTF16PtrFromString(cmd)
	r, _, _ := procMciSendStringW.Call(uintptr(unsafe.Pointer(p)), 0, 0, 0)
	if r != 0 {
		buf := make([]uint16, 256)
		procMciGetErrorW.Call(r, uintptr(unsafe.Pointer(&buf[0])), 256)
		return fmt.Errorf("%s", syscall.UTF16ToString(buf))
	}
	return nil
}

// Sound plays a short file through MCI (handles mp3 and wav natively).
// All calls must come from the UI thread.
type Sound struct {
	Alias  string // MCI name; "" = the count sound
	open   bool
	path   string
	volume int
}

const soundAlias = "dcsound"

func (s *Sound) alias() string {
	if s.Alias == "" {
		return soundAlias
	}
	return s.Alias
}

func (s *Sound) Load(path string, volume int) error {
	s.Close()
	if strings.ContainsRune(path, '"') {
		return fmt.Errorf("đường dẫn không hợp lệ")
	}
	typ := "mpegvideo"
	if strings.HasSuffix(strings.ToLower(path), ".wav") {
		typ = "waveaudio"
	}
	if err := mci(fmt.Sprintf(`open "%s" type %s alias %s`, path, typ, s.alias())); err != nil {
		return err
	}
	s.open, s.path = true, path
	s.SetVolume(volume)
	return nil
}

func (s *Sound) SetVolume(v int) {
	s.volume = v
	if s.open {
		mci(fmt.Sprintf("setaudio %s volume to %d", s.alias(), v*10)) // 0..1000; not supported for waveaudio, ignored
	}
}

func (s *Sound) Loaded() bool { return s.open }

func (s *Sound) Play() error {
	if !s.open {
		return fmt.Errorf("chưa tải âm thanh")
	}
	return mci(fmt.Sprintf("play %s from 0", s.alias()))
}

func (s *Sound) Close() {
	if s.open {
		mci("close " + s.alias())
		s.open = false
	}
}

// Playing reports whether the sound is still being played.
func (s *Sound) Playing() bool {
	if !s.open {
		return false
	}
	buf := make([]uint16, 32)
	p, _ := syscall.UTF16PtrFromString("status " + s.alias() + " mode")
	r, _, _ := procMciSendStringW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	return r == 0 && syscall.UTF16ToString(buf) == "playing"
}
