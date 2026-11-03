//go:build windows

package main

import (
	"fmt"
	"runtime"

	"github.com/lxn/win"
)

type HotkeyAction int

const (
	ActAdd HotkeyAction = iota + 1
	ActSub
	ActCapture
	ActToggle
	ActWindow
)

// HotkeyManager owns a dedicated OS thread: RegisterHotKey with a NULL window
// posts WM_HOTKEY to the registering thread's message queue.
type HotkeyManager struct {
	threadID uint32
	done     chan struct{}
}

const wmQuit = 0x0012

// StartHotkeys registers keys and calls onPress (from the hotkey thread) for each press.
// Returns per-key registration errors (e.g. key already used by another app).
func StartHotkeys(keys map[HotkeyAction]string, onPress func(HotkeyAction)) (*HotkeyManager, []string) {
	hm := &HotkeyManager{done: make(chan struct{})}
	ready := make(chan []string)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(hm.done)
		hm.threadID = win.GetCurrentThreadId()

		var errs []string
		var registered []HotkeyAction
		for act, spec := range keys {
			mods, vk, err := ParseHotkey(spec)
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}
			r, _, e := procRegisterHotKey.Call(0, uintptr(act), uintptr(mods|ModNoRepeat), uintptr(vk))
			if r == 0 {
				errs = append(errs, fmt.Sprintf("không đăng ký được %s (có app khác đang dùng?): %v", spec, e))
				continue
			}
			registered = append(registered, act)
		}
		ready <- errs

		var msg win.MSG
		for {
			r := win.GetMessage(&msg, 0, 0, 0)
			if r == 0 || r == -1 { // WM_QUIT or error
				break
			}
			if msg.Message == win.WM_HOTKEY {
				onPress(HotkeyAction(msg.WParam))
			}
		}
		for _, act := range registered {
			procUnregisterHotKey.Call(0, uintptr(act))
		}
	}()
	errs := <-ready
	return hm, errs
}

func (hm *HotkeyManager) Stop() {
	if hm == nil {
		return
	}
	procPostThreadMessageW.Call(uintptr(hm.threadID), wmQuit, 0, 0)
	<-hm.done
}
