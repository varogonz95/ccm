//go:build !windows

package web

import (
	"os/exec"
	"runtime"
)

// OpenBrowser opens url in the default browser without waiting for it. The
// URL is visible to other local users on the command line, so it must not
// carry the access key: ccm web passes a Redirect file URL.
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
