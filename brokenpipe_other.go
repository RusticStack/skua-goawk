//go:build !windows

package main

import (
	"errors"
	"syscall"
)

// isBrokenPipe reports whether err is a write to a pipe whose reader has gone.
// (Go's runtime already dies of SIGPIPE for stdout and stderr on Unix.)
func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
