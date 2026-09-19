//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit -framework Foundation
#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>

void switchboardSetActivationPolicy(int policy) {
	[NSApp setActivationPolicy:(NSApplicationActivationPolicy)policy];
}

extern void switchboardGoWake(void);

static id switchboardWakeToken = nil;
static id switchboardUnlockToken = nil;

void switchboardInstallWakeObserver(void) {
	if (switchboardWakeToken != nil) {
		return;
	}

	NSNotificationCenter *wsnc = [[NSWorkspace sharedWorkspace] notificationCenter];
	switchboardWakeToken = [wsnc addObserverForName:NSWorkspaceDidWakeNotification
	                                         object:nil
	                                          queue:nil
	                                     usingBlock:^(NSNotification *note) {
		switchboardGoWake();
	}];

	NSDistributedNotificationCenter *dnc = [NSDistributedNotificationCenter defaultCenter];
	switchboardUnlockToken = [dnc addObserverForName:@"com.apple.screenIsUnlocked"
	                                          object:nil
	                                           queue:nil
	                                      usingBlock:^(NSNotification *note) {
		switchboardGoWake();
	}];
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
