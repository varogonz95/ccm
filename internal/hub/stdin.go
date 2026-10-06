package hub

import "errors"

// errCanceled is returned by cancelReader.Read once Cancel has been called.
var errCanceled = errors.New("read canceled")

// cancelReader (stdin_unix.go, stdin_windows.go) reads a terminal without
// ever blocking in a read that has no input, so Cancel always unblocks a
// pending Read. Cancel is safe to call concurrently with Read; Close must
// not be called until Read has returned.
