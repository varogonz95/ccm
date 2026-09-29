//go:build windows

package ptyx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/UserExistsError/conpty"
)

type winPTY struct {
	c *conpty.ConPty
}

func Start(o Options) (PTY, error) {
	path, err := exec.LookPath(o.Command) // honours PATHEXT (.exe, .cmd, ...)
	if err != nil {
		return nil, err
	}
	parts := []string{syscall.EscapeArg(path)}
	for _, a := range o.Args {
		parts = append(parts, syscall.EscapeArg(a))
	}
	cmdline := strings.Join(parts, " ")
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cmd", ".bat":
		// npm installs claude as a .cmd shim, which CreateProcess can't launch directly.
		cmdline = `cmd.exe /d /s /c "` + cmdline + `"`
	}
	c, err := conpty.Start(cmdline,
		conpty.ConPtyDimensions(o.Cols, o.Rows),
		conpty.ConPtyWorkDir(o.Dir),
		conpty.ConPtyEnv(o.Env),
	)
	if err != nil {
		return nil, err
	}
	return &winPTY{c: c}, nil
}

func (p *winPTY) Read(b []byte) (int, error)  { return p.c.Read(b) }
func (p *winPTY) Write(b []byte) (int, error) { return p.c.Write(b) }
func (p *winPTY) Close() error                { return p.c.Close() }
func (p *winPTY) Pid() int                    { return p.c.Pid() }

func (p *winPTY) Resize(cols, rows int) error { return p.c.Resize(cols, rows) }

// TODO(M4): assign the process to a Job Object so claude's children die with it.
func (p *winPTY) Kill(force bool) error {
	proc, err := os.FindProcess(p.c.Pid())
	if err != nil {
		return err
	}
	return proc.Kill()
}

func (p *winPTY) Wait() (int, error) {
	code, err := p.c.Wait(context.Background())
	return int(code), err
}
