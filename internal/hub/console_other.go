//go:build !windows

package hub

import "os"

func prepareConsole(*os.File) (restore func()) { return func() {} }
