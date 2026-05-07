//go:build darwin

package permissions

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework AppKit -framework ApplicationServices -framework AVFoundation

#import <AppKit/AppKit.h>
#import <ApplicationServices/ApplicationServices.h>
#import <AVFoundation/AVFoundation.h>

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

// Check returns the current state of both permissions without prompting.
func Check() Status {
	return Status{
		Microphone:    C.genie_mic_authorized() == 1,
		Accessibility: C.genie_axtrusted_prompt(0) == 1,
	}
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
