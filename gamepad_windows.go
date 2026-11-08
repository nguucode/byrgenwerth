//go:build windows

package main

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Xbox controllers (and anything Steam Input or DS4Windows presents as one)
// through XInput. A background goroutine polls the four slots.

var (
	xinputDLL      *syscall.LazyDLL
	procXInputGet  *syscall.LazyProc
	xinputLoadOnce sync.Once
)

func xinputGetState() *syscall.LazyProc {
	xinputLoadOnce.Do(func() {
		for _, name := range []string{"xinput1_4.dll", "xinput9_1_0.dll", "xinput1_3.dll"} {
			d := syscall.NewLazyDLL(name)
			if d.Load() == nil {
				xinputDLL, procXInputGet = d, d.NewProc("XInputGetState")
				return
			}
		}
	})
	return procXInputGet
}

type xinputState struct {
	Packet       uint32
	Buttons      uint16
	LeftTrigger  uint8
	RightTrigger uint8
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

// padButtonsNow returns the buttons held on any connected controller. Empty
// slots are asked again only every 2 seconds: asking them is slow.
func (g *Gamepads) padButtonsNow() uint32 {
	proc := xinputGetState()
	if proc == nil {
		return 0
	}
	var all uint32
	now := time.Now()
	for i := 0; i < 4; i++ {
		if !g.connected[i] && now.Before(g.retry[i]) {
			continue
		}
		var st xinputState
		r, _, _ := proc.Call(uintptr(i), uintptr(unsafe.Pointer(&st)))
		if r != 0 {
			g.connected[i] = false
			g.retry[i] = now.Add(2 * time.Second)
			continue
		}
		g.connected[i] = true
		b := uint32(st.Buttons)
		if st.LeftTrigger > 128 {
			b |= PadLT
		}
		if st.RightTrigger > 128 {
			b |= PadRT
		}
		all |= b
	}
	return all
}

// Gamepads watches for one button combination and, while the setup dialog is
// open, learns a new one. Callbacks run on the polling goroutine.
type Gamepads struct {
	mu        sync.Mutex
	combo     PadCombo
	onPress   func()
	capture   *PadCapture
	onCapture func(mask uint32)
	connected [4]bool
	retry     [4]time.Time
	quit      chan struct{}
	done      chan struct{}
}

func StartGamepads(onPress func()) *Gamepads {
	g := &Gamepads{onPress: onPress, quit: make(chan struct{}), done: make(chan struct{})}
	go g.run()
	return g
}

// SetCombo changes the combination that fires onPress (0 = none).
func (g *Gamepads) SetCombo(mask uint32) {
	g.mu.Lock()
	g.combo = PadCombo{Mask: mask, held: true} // a combination held right now must be let go first
	g.mu.Unlock()
}

// Capture reports the next combination pressed and let go; nil stops learning.
func (g *Gamepads) Capture(f func(mask uint32)) {
	g.mu.Lock()
	if f == nil {
		g.capture = nil
	} else {
		g.capture = &PadCapture{}
	}
	g.onCapture = f
	g.mu.Unlock()
}

func (g *Gamepads) Stop() {
	if g == nil {
		return
	}
	close(g.quit)
	<-g.done
}

func (g *Gamepads) run() {
	defer close(g.done)
	for {
		g.mu.Lock()
		active := g.combo.Mask != 0 || g.capture != nil
		g.mu.Unlock()
		wait := 30 * time.Millisecond
		if !active {
			wait = 500 * time.Millisecond
		}
		select {
		case <-g.quit:
			return
		case <-time.After(wait):
		}
		if !active {
			continue
		}
		buttons := g.padButtonsNow()
		g.mu.Lock()
		var fire func()
		if g.capture != nil {
			if mask, ok := g.capture.Feed(buttons); ok {
				f := g.onCapture
				fire = func() { f(mask) }
			}
		} else if g.combo.Feed(buttons) {
			fire = g.onPress
		}
		g.mu.Unlock()
		if fire != nil {
			fire()
		}
	}
}
