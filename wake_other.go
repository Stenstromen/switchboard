//go:build !darwin

package main

func installWakeObserver(onWake func()) {
	// Sleep/wake recovery is macOS-specific.
}
