//go:build !unindent_never

// Package buildstub has functions that never return only in builds without the unindent_never tag, as the runtime's
// race detector stubs only throw in builds without -race.
package buildstub

// Stop is a stub that panics in this build.
func Stop() { panic("stub") }

// Stop is a stub that panics in this build.
func (*Stopper) Stop() { panic("stub") }

// Halt is only declared in this build, so every build with a call to it has this one.
func Halt() { panic("halt") }
