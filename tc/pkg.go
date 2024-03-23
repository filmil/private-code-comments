// Package tc contains test-only code.
package tc

import "fmt"

// Must1 panics if the error is not nil. Used to wrap functions returning error
// to force simplify error handling.
func Must1(err error) {
	if err != nil {
		panic(fmt.Sprintf("Must1: error: %v", err))
	}
}

// Must panics if the error is not nil. Used to wrap functions returning error
// to force simplify error handling.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("Must error: %v", err))
	}
	return v
}

// Must3 panics if the error is not nil. Used to wrap functions returning error
// to force simplify error handling.
func Must3[T any, V any](t T, v V, err error) (T, V) {
	if err != nil {
		panic(fmt.Sprintf("Must3: error: %v", err))
	}
	return t, v
}
