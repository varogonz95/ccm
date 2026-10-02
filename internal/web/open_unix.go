//go:build !windows

package web

import (
	"os/exec"
	"runtime"
)

// OpenBrowser opens url in the default browser without waiting for it.
func OpenBrowser(url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	cmd := exec.Command(name, url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }() // reap it
	return nil
}
