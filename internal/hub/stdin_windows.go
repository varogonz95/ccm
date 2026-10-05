//go:build windows

package hub

import (
	"fmt"
	"io"
	"os"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procPeekConsoleInputW = kernel32.NewProc("PeekConsoleInputW")
	procReadConsoleInputW = kernel32.NewProc("ReadConsoleInputW")
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
	keyEvent = 0x0001
	vkMenu   = 0x12
)

// yieldsChar reports whether ReadConsole would return a character for rec.
func (rec *inputRecord) yieldsChar() bool {
	if rec.EventType != keyEvent || rec.UnicodeChar == 0 {
		return false
	}
	// Alt+numpad input delivers its character on the Alt release.
	return rec.KeyDown != 0 || rec.VirtualKeyCode == vkMenu
}

// cancelReader waits on the console handle and a cancel event. The console
// handle is signaled by any input record (focus, key-up, resize...), and
// ReadConsole blocks until a character arrives, so records are peeked first
// and ones that yield no character are discarded; ReadConsole is only called
// when it will return at once. It does not touch the console mode, which
// term.MakeRaw and prepareConsole own.
type cancelReader struct {
	h      windows.Handle
	cancel windows.Handle // manual-reset event set by Cancel
	recs   [64]inputRecord
	u16    []uint16
	hi     uint16 // high surrogate held for the next read
}

func newCancelReader(f *os.File) (*cancelReader, error) {
	ev, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("create cancel event: %w", err)
	}
	return &cancelReader{h: windows.Handle(f.Fd()), cancel: ev}, nil
}

func (r *cancelReader) Read(b []byte) (int, error) {
	if len(b) < 6 {
		return 0, io.ErrShortBuffer
	}
	for {
		ev, err := windows.WaitForMultipleObjects([]windows.Handle{r.h, r.cancel}, false, windows.INFINITE)
		switch ev {
		case windows.WAIT_OBJECT_0:
		case windows.WAIT_OBJECT_0 + 1:
			return 0, errCanceled
		default:
			return 0, fmt.Errorf("wait for console input: %w", err)
		}
		ready, err := r.charReady()
		if err != nil {
			return 0, err
		}
		if !ready {
			continue
		}
		if n, err := r.readConsole(b); n > 0 || err != nil {
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
	var nr uint32
	if err := windows.ReadConsole(r.h, &u[start], uint32(units-start), &nr, nil); err != nil {
		return 0, err
	}
	u = u[:start+int(nr)]
	if l := len(u); l > 0 && utf16.IsSurrogate(rune(u[l-1])) && u[l-1] < 0xdc00 {
		r.hi, u = u[l-1], u[:l-1]
	}
	n := 0
	for _, c := range utf16.Decode(u) {
		n += utf8.EncodeRune(b[n:], c)
	}
	return n, nil
}

func (r *cancelReader) Cancel() { _ = windows.SetEvent(r.cancel) }

func (r *cancelReader) Close() error { return windows.CloseHandle(r.cancel) }

func consoleInput(p *windows.LazyProc, h windows.Handle, recs []inputRecord) (int, error) {
	var n uint32
	ok, _, err := p.Call(uintptr(h), uintptr(unsafe.Pointer(&recs[0])), uintptr(len(recs)), uintptr(unsafe.Pointer(&n)))
	if ok == 0 {
		return 0, err
	}
	return int(n), nil
}
