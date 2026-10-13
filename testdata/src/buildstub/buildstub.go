package buildstub

import "os"

// Wraps never returns only because Stop doesn't.
func Wraps() { Stop() }

// WrapsTwice never returns only because Wraps doesn't.
func WrapsTwice() { Wraps() }

// Panics never returns in any build: it panics after Stop.
func Panics() {
	Stop()
	panic("always")
}

// Exits never returns in any build.
func Exits() {
	os.Exit(1)
}

// MayStop returns when stop is false.
func MayStop(stop bool) {
	if stop {
		Stop()
	}
}

// Stopper has a Stop method that never returns only in some builds.
type Stopper struct{}
