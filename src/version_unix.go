//go:build !windows

package main

// GetVersion returns the version of the application
func GetVersion() uint32 {
	return 0
}
