//go:build ignore

// A generator in another package, which is never built with buildstub: its Halt is not a counterpart of buildstub's.
package main

func Halt() {}

func main() {}
