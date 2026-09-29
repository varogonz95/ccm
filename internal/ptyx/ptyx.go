// Package ptyx is a minimal cross-platform PTY abstraction:
// creack/pty on Unix, ConPTY on Windows.
package ptyx

import "io"

type Options struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
	Cols    int
	Rows    int
}

type PTY interface {
	io.ReadWriteCloser
	Resize(cols, rows int) error
	// Wait blocks until the process exits and returns its exit code.
	Wait() (int, error)
	// Kill asks the process to stop; force skips the graceful signal.
	Kill(force bool) error
	Pid() int
}
