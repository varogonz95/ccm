package web

import "testing"

func TestCheckLoopback(t *testing.T) {
	ok := []string{"127.0.0.1:7421", "localhost:7421", "[::1]:7421", "127.0.0.1:0"}
	bad := []string{":7421", "0.0.0.0:7421", "192.168.1.20:7421", "[::]:7421", "example.com:7421", "7421"}
	for _, a := range ok {
		if err := CheckLoopback(a); err != nil {
			t.Errorf("%s: unexpected error %v", a, err)
		}
	}
	for _, a := range bad {
		if err := CheckLoopback(a); err == nil {
			t.Errorf("%s: expected an error", a)
		}
	}
}
