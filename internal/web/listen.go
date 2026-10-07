package web

import (
	"fmt"
	"net"
)

// CheckLoopback refuses listen addresses other than loopback. Serving the UI
// to the LAN needs its own login and TLS (issue #6).
func CheckLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("--listen %s: %w", addr, err)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("--listen %s: clawsh web only listens on loopback (e.g. 127.0.0.1:7421); LAN access is issue #6", addr)
}
