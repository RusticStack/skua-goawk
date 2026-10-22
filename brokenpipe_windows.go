package main

import (
	"errors"
	"syscall"
)

// isBrokenPipe reports whether err is a write to a pipe whose reader has gone
// (ERROR_BROKEN_PIPE, or ERROR_NO_DATA: "The pipe is being closed").
func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.ERROR_BROKEN_PIPE) || errors.Is(err, syscall.Errno(232))
}
