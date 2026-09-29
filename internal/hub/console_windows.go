//go:build windows

package hub

import (
	"os"

	"golang.org/x/sys/windows"
)

// prepareConsole enables VT output and UTF-8 so claude's TUI renders correctly
// in the local Windows console. Raw VT *input* is handled by term.MakeRaw.
func prepareConsole() (restore func()) {
	out := windows.Handle(os.Stdout.Fd())
	var mode uint32
	haveMode := windows.GetConsoleMode(out, &mode) == nil
	if haveMode {
		_ = windows.SetConsoleMode(out, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
	inCP, _ := windows.GetConsoleCP()
	outCP, _ := windows.GetConsoleOutputCP()
	_ = windows.SetConsoleCP(65001)
	_ = windows.SetConsoleOutputCP(65001)
	return func() {
		if haveMode {
			_ = windows.SetConsoleMode(out, mode)
		}
		if inCP != 0 {
			_ = windows.SetConsoleCP(inCP)
		}
		if outCP != 0 {
			_ = windows.SetConsoleOutputCP(outCP)
		}
	}
}
