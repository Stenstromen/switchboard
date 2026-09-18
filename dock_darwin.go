//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

void switchboardSetActivationPolicy(int policy) {
	[NSApp setActivationPolicy:(NSApplicationActivationPolicy)policy];
}
*/
import "C"

const (
	activationPolicyRegular   = 0
	activationPolicyAccessory = 1
)

func setDockVisible(visible bool) {
	if visible {
		C.switchboardSetActivationPolicy(activationPolicyRegular)
	} else {
		C.switchboardSetActivationPolicy(activationPolicyAccessory)
	}
}
