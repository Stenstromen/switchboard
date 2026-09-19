//go:build darwin

package main

/*
void switchboardInstallWakeObserver(void);
*/
import "C"
import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	wakeHandler    atomic.Pointer[func()]
	wakeDebounce   sync.Mutex
	wakePending    bool
	wakeDelay      = 1500 * time.Millisecond
	wakeSecondPass = 5 * time.Second
)

//export switchboardGoWake
func switchboardGoWake() {
	fn := wakeHandler.Load()
	if fn == nil || *fn == nil {
		return
	}
	wakeDebounce.Lock()
	if wakePending {
		wakeDebounce.Unlock()
		return
	}
	wakePending = true
	wakeDebounce.Unlock()

	handler := *fn
	go func() {
		time.Sleep(wakeDelay)
		handler()
		// TCP / keepalive state often settles a few seconds later.
		time.Sleep(wakeSecondPass)
		handler()
		wakeDebounce.Lock()
		wakePending = false
		wakeDebounce.Unlock()
	}()
}

func installWakeObserver(onWake func()) {
	h := onWake
	wakeHandler.Store(&h)
	C.switchboardInstallWakeObserver()
}
