//go:build !darwin

package main

func setDockVisible(visible bool) {
	// Dock / ActivationPolicy is a macOS concept.
}
