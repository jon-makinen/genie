//go:build darwin

package permissions

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework AppKit -framework ApplicationServices -framework AVFoundation

#import <AppKit/AppKit.h>
#import <ApplicationServices/ApplicationServices.h>
#import <AVFoundation/AVFoundation.h>

static const char* genie_bundle_id(void) {
    NSString* b = [[NSBundle mainBundle] bundleIdentifier];
    if (b == nil) return strdup("");
    return strdup([b UTF8String]);
}

static int genie_axtrusted_prompt(int prompt) {
    CFStringRef key = kAXTrustedCheckOptionPrompt;
    CFBooleanRef val = prompt ? kCFBooleanTrue : kCFBooleanFalse;
    CFDictionaryRef opts = CFDictionaryCreate(NULL,
        (const void**)&key, (const void**)&val, 1,
        &kCFCopyStringDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    Boolean trusted = AXIsProcessTrustedWithOptions(opts);
    if (opts) CFRelease(opts);
    return trusted ? 1 : 0;
}

static int genie_mic_authorized(void) {
    if (@available(macOS 10.14, *)) {
        AVAuthorizationStatus s = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
        return (s == AVAuthorizationStatusAuthorized) ? 1 : 0;
    }
    return 1;
}

static void genie_request_mic(void) {
    if (@available(macOS 10.14, *)) {
        [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio
                                  completionHandler:^(BOOL granted){ (void)granted; }];
    }
}

static void genie_open_accessibility(void) {
    NSURL* url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"];
    [[NSWorkspace sharedWorkspace] openURL:url];
}

static void genie_open_microphone(void) {
    NSURL* url = [NSURL URLWithString:@"x-apple.systempreferences:com.apple.preference.security?Privacy_Microphone"];
    [[NSWorkspace sharedWorkspace] openURL:url];
}
*/
import "C"

import (
	"fmt"
	"os/exec"
	"unsafe"
)

// Check returns the current state of both permissions without prompting.
func Check() Status {
	return Status{
		Microphone:    C.genie_mic_authorized() == 1,
		Accessibility: C.genie_axtrusted_prompt(0) == 1,
	}
}

// BundleID returns the running app's bundle identifier (e.g. com.jonmakinen.genie).
func BundleID() string {
	c := C.genie_bundle_id()
	defer C.free(unsafe.Pointer(c))
	return C.GoString(c)
}

// ResetAccessibility clears Genie's entry from the Accessibility TCC database
// and re-prompts. This is the fix for the common "checked in System Settings
// but AXIsProcessTrusted returns false" state that happens after a rebuild
// (the old cdhash is remembered but no longer matches). After reset the user
// gets a fresh allow-prompt and the trust is bound to the new binary.
func ResetAccessibility() error {
	id := BundleID()
	if id == "" {
		return fmt.Errorf("bundle identifier unavailable")
	}
	if out, err := exec.Command("tccutil", "reset", "Accessibility", id).CombinedOutput(); err != nil {
		return fmt.Errorf("tccutil reset: %v: %s", err, out)
	}
	C.genie_axtrusted_prompt(1)
	return nil
}

// RequestMicrophone triggers the system mic prompt asynchronously.
func RequestMicrophone() {
	C.genie_request_mic()
}

// PromptAccessibility opens the Accessibility prompt with the explanatory dialog.
func PromptAccessibility() {
	_ = C.genie_axtrusted_prompt(1)
}

// OpenAccessibilityPrefs jumps the user into System Settings.
func OpenAccessibilityPrefs() {
	C.genie_open_accessibility()
}

// OpenMicrophonePrefs jumps the user into System Settings.
func OpenMicrophonePrefs() {
	C.genie_open_microphone()
}
