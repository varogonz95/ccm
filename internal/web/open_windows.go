//go:build windows

package web

import "os/exec"

// OpenBrowser opens url in the default browser without waiting for it. The
// URL is visible to other local users on the command line, so it must not
// carry the access key: ccm web passes a Redirect file URL.
func OpenBrowser(url string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
