//go:build unindent_never

package buildstub

// Stop returns in this build.
func Stop() {}

// Stop returns in this build.
func (*Stopper) Stop() {}
