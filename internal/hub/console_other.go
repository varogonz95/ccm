//go:build !windows

package hub

func prepareConsole() (restore func()) { return func() {} }
