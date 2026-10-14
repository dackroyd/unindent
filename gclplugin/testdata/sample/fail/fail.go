// Package fail holds a helper that never returns, so callers in other packages exit through it.
package fail

import "os"

func Exit(msg string) {
	println(msg)
	os.Exit(1)
}
