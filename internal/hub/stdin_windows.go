//go:build windows

package hub

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procPeekConsoleInputW   = kernel32.NewProc("PeekConsoleInputW")
	procReadConsoleInputW   = kernel32.NewProc("ReadConsoleInputW")
	procCancelSynchronousIo = kernel32.NewProc("CancelSynchronousIo")
)

// inputRecord is INPUT_RECORD with the KEY_EVENT_RECORD arm of the union.
type inputRecord struct {
	EventType       uint16
	_               uint16
	KeyDown         int32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	UnicodeChar     uint16
	ControlKeyState uint32
}

const (
	keyEvent  = 0x0001
	vkSpace   = 0x20
	vk2       = 0x32
	vkMenu    = 0x12
	ctrlState = 0x0004 | 0x0008 // RIGHT_CTRL_PRESSED | LEFT_CTRL_PRESSED
)

// yieldsChar reports whether ReadConsole should return a character for rec.
// A wrong "yes" only makes ReadConsole wait for the next key, and Cancel
// still interrupts it (see cancelRead), so this errs on the side of yes.
func (rec *inputRecord) yieldsChar() bool {
	if rec.EventType != keyEvent {
		return false
	}
	if rec.KeyDown == 0 {
		// Alt+numpad input delivers its character on the Alt release.
		return rec.VirtualKeyCode == vkMenu && rec.UnicodeChar != 0
	}
	if rec.UnicodeChar != 0 {
		return true
	}
	// Ctrl+Space and Ctrl+@ (Ctrl+2) send NUL.
	return rec.ControlKeyState&ctrlState != 0 && (rec.VirtualKeyCode == vkSpace || rec.VirtualKeyCode == vk2)
}

// cancelReader waits on the console handle and a cancel event. The console
// handle is signaled by any input record (focus, key-up, resize...), and
// ReadConsole blocks until a character arrives, so records are peeked first
// and ones that yield no character are discarded; ReadConsole is only called
// when it should return at once. If it blocks anyway, Cancel interrupts it
// with CancelSynchronousIo. It does not touch the console mode, which
// term.MakeRaw and prepareConsole own.
type cancelReader struct {
	h      windows.Handle
	cancel windows.Handle // manual-reset event set by Cancel
	recs   [64]inputRecord
	u16    []uint16
	hi     uint16 // high surrogate held for the next read

	mu      sync.Mutex
	thread  windows.Handle // thread inside ReadConsole, or 0
	stopped chan struct{}  // closed by Cancel
	once    sync.Once
}

func newCancelReader(f *os.File) (*cancelReader, error) {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("create cancel event: %w", err)
	}
	return &cancelReader{h: windows.Handle(f.Fd()), cancel: ev, stopped: make(chan struct{})}, nil
}

func (r *cancelReader) canceled() bool {
	select {
	case <-r.stopped:
		return true
	default:
		return false
	}
}

func (r *cancelReader) Read(b []byte) (int, error) {
	if len(b) < 6 {
		return 0, io.ErrShortBuffer
	}
	for {
		// Checked first: with input pending, the wait below reports the
		// console handle even when the cancel event is set too.
		if r.canceled() {
			return 0, errCanceled
		}
		ev, err := windows.WaitForMultipleObjects([]windows.Handle{r.h, r.cancel}, false, windows.INFINITE)
		switch ev {
		case windows.WAIT_OBJECT_0:
		case windows.WAIT_OBJECT_0 + 1:
			return 0, errCanceled
		case windows.WAIT_FAILED:
			return 0, fmt.Errorf("wait for console input: %w", err)
		default:
			return 0, fmt.Errorf("wait for console input: unexpected result %#x", ev)
		}
		ready, err := r.charReady()
		if err != nil {
			return 0, err
		}
		if !ready || r.canceled() {
			continue
		}
		n, err := r.readConsole(b)
		if r.canceled() {
			return 0, errCanceled
		}
		if n > 0 || err != nil {
			return n, err
		}
	}
}

// charReady peeks the pending records. If none yields a character it
// discards them, so the handle stops being signaled for them.
func (r *cancelReader) charReady() (bool, error) {
	n, err := consoleInput(procPeekConsoleInputW, r.h, r.recs[:])
	if err != nil {
		return false, fmt.Errorf("peek console input: %w", err)
	}
	if n == 0 {
		time.Sleep(10 * time.Millisecond) // signaled with nothing queued; don't spin
		return false, nil
	}
	for i := range r.recs[:n] {
		if r.recs[i].yieldsChar() {
			return true, nil
		}
	}
	if _, err := consoleInput(procReadConsoleInputW, r.h, r.recs[:n]); err != nil {
		return false, fmt.Errorf("discard console input: %w", err)
	}
	return false, nil
}

// readConsole reads UTF-16 and encodes it into b. It reads at most len(b)/3
// units so the UTF-8 always fits and nothing is left buffered here while the
// console handle is unsignaled.
func (r *cancelReader) readConsole(b []byte) (int, error) {
	units := len(b) / 3
	if cap(r.u16) < units {
		r.u16 = make([]uint16, units)
	}
	u := r.u16[:units]
	start := 0
	if r.hi != 0 {
		u[0], r.hi, start = r.hi, 0, 1
	}

	// Pin the thread and publish it so Cancel can interrupt this call.
	runtime.LockOSThread()
	th, err := windows.OpenThread(windows.THREAD_TERMINATE, false, windows.GetCurrentThreadId())
	if err != nil {
		runtime.UnlockOSThread()
		return 0, fmt.Errorf("open thread: %w", err)
	}
	r.mu.Lock()
	if r.canceled() { // Cancel ran after Read's last check
		r.mu.Unlock()
		windows.CloseHandle(th)
		runtime.UnlockOSThread()
		return 0, errCanceled
	}
	r.thread = th
	r.mu.Unlock()
	var nr uint32
	err = windows.ReadConsole(r.h, &u[start], uint32(units-start), &nr, nil)
	r.mu.Lock()
	r.thread = 0
	r.mu.Unlock()
	windows.CloseHandle(th)
	runtime.UnlockOSThread()
	if err != nil {
		return 0, err
	}

	u = u[:start+int(nr)]
	if l := len(u); l > 0 && u[l-1] >= 0xd800 && u[l-1] < 0xdc00 {
		r.hi, u = u[l-1], u[:l-1]
	}
	n := 0
	for i := 0; i < len(u); i++ {
		c := rune(u[i])
		if utf16.IsSurrogate(c) {
			c = utf8.RuneError
			if i+1 < len(u) {
				if d := utf16.DecodeRune(rune(u[i]), rune(u[i+1])); d != utf8.RuneError {
					c = d
					i++
				}
			}
		}
		n += utf8.EncodeRune(b[n:], c)
	}
	return n, nil
}

// Cancel makes pending and future Reads return errCanceled. A Read inside
// ReadConsole is interrupted with CancelSynchronousIo, retried until it
// leaves, since the call may not have started blocking yet.
func (r *cancelReader) Cancel() {
	r.once.Do(func() {
		close(r.stopped)
		_ = windows.SetEvent(r.cancel)
		go func() {
			for {
				r.mu.Lock()
				th := r.thread
				if th != 0 {
					procCancelSynchronousIo.Call(uintptr(th))
				}
				r.mu.Unlock()
				if th == 0 {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
	})
}

func (r *cancelReader) Close() error { return windows.CloseHandle(r.cancel) }

func consoleInput(p *windows.LazyProc, h windows.Handle, recs []inputRecord) (int, error) {
	var n uint32
	ok, _, err := p.Call(uintptr(h), uintptr(unsafe.Pointer(&recs[0])), uintptr(len(recs)), uintptr(unsafe.Pointer(&n)))
	if ok == 0 {
		return 0, err
	}
	return int(n), nil
}
