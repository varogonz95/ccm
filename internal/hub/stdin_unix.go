//go:build !windows

package hub

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// cancelReader waits with select(2) on the file and a self-pipe, and only
// reads once the file is readable. select, not poll: macOS poll does not
// support tty devices.
type cancelReader struct {
	f      *os.File
	cr, cw *os.File // written to by Cancel
	once   sync.Once
}

func newCancelReader(f *os.File) (*cancelReader, error) {
	cr, cw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	limit := 8 * int(unsafe.Sizeof(unix.FdSet{}))
	if int(f.Fd()) >= limit || int(cr.Fd()) >= limit {
		cr.Close()
		cw.Close()
		return nil, fmt.Errorf("fd beyond select limit %d", limit)
	}
	return &cancelReader{f: f, cr: cr, cw: cw}, nil
}

func (r *cancelReader) Read(b []byte) (int, error) {
	fd, cfd := int(r.f.Fd()), int(r.cr.Fd())
	for {
		var set unix.FdSet
		set.Set(fd)
		set.Set(cfd)
		if _, err := unix.Select(max(fd, cfd)+1, &set, nil, nil, nil); err != nil {
			if err == unix.EINTR {
				continue
			}
			return 0, err
		}
		if set.IsSet(cfd) {
			return 0, errCanceled
		}
		if set.IsSet(fd) {
			return r.f.Read(b)
		}
	}
}

func (r *cancelReader) Cancel() {
	r.once.Do(func() { _, _ = r.cw.Write([]byte{0}) })
}

func (r *cancelReader) Close() error {
	r.cw.Close()
	return r.cr.Close()
}
