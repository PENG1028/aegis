// Package recovery provides panic recovery for background goroutines.
package core

import (
	"fmt"
	"log"
	"runtime/debug"
)

// Go runs fn in a new goroutine with panic recovery.
// If fn panics, the panic is logged with stack trace instead of crashing the process.
func Go(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] goroutine %q: %v\n%s", name, r, string(debug.Stack()))
			}
		}()
		fn()
	}()
}

// Wrap returns a function that calls fn with panic recovery.
// Use for goroutines that are started with `go fn()` directly.
func Wrap(name string, fn func()) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("[PANIC] goroutine %q: %v\n%s", name, r, string(debug.Stack()))
				log.Print(err)
			}
		}()
		fn()
	}
}
