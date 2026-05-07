//go:build darwin

// macOS implementation of Injector: copy the text onto NSPasteboard, post a
// synthesized Cmd+V to the focused app, then restore the user's previous
// clipboard contents ~250 ms later.

package inject

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices -framework Carbon

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <Carbon/Carbon.h>

// genie_save_clipboard returns the current clipboard string content (or "").
// Caller must free() the returned char*.
static const char* genie_save_clipboard(void) {
    NSPasteboard* pb = [NSPasteboard generalPasteboard];
    NSString* s = [pb stringForType:NSPasteboardTypeString];
    if (s == nil) {
        return strdup("");
    }
    return strdup([s UTF8String]);
}

// genie_set_clipboard replaces the clipboard contents with the given UTF-8 string.
static void genie_set_clipboard(const char* utf8) {
    NSString* s = [NSString stringWithUTF8String:utf8];
    NSPasteboard* pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    [pb setString:s forType:NSPasteboardTypeString];
}

// genie_paste posts a Cmd+V keystroke to the system event tap.
static void genie_paste(void) {
    CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateCombinedSessionState);
    CGEventRef down = CGEventCreateKeyboardEvent(src, (CGKeyCode)kVK_ANSI_V, true);
    CGEventSetFlags(down, kCGEventFlagMaskCommand);
    CGEventRef up = CGEventCreateKeyboardEvent(src, (CGKeyCode)kVK_ANSI_V, false);
    CGEventSetFlags(up, kCGEventFlagMaskCommand);

    CGEventPost(kCGAnnotatedSessionEventTap, down);
    CGEventPost(kCGAnnotatedSessionEventTap, up);

    if (down) CFRelease(down);
    if (up) CFRelease(up);
    if (src) CFRelease(src);
}
*/
import "C"

import (
	"sync"
	"time"
	"unsafe"
)

type macInjector struct {
	mu sync.Mutex
	// gen is bumped on every Type call. The deferred restore goroutine only
	// touches the clipboard if its captured generation still matches — so back-
	// to-back Type calls can't race each other into corrupting the saved value.
	gen     uint64
	savedAt uint64 // gen at the time `saved` was captured; 0 means unset
	saved   string
}

// New returns a macOS clipboard-paste injector.
func New() Injector { return &macInjector{} }

func (m *macInjector) Type(text string) error {
	if text == "" {
		return nil
	}
	m.mu.Lock()

	m.gen++
	gen := m.gen

	// Only snapshot the clipboard when there isn't already a pending restore.
	// Otherwise the "previous" clipboard is the user's, not the value we just
	// pasted on a prior call.
	if m.savedAt == 0 {
		prevC := C.genie_save_clipboard()
		m.saved = C.GoString(prevC)
		C.free(unsafe.Pointer(prevC))
		m.savedAt = gen
	}

	cstr := C.CString(text)
	C.genie_set_clipboard(cstr)
	C.free(unsafe.Pointer(cstr))
	m.mu.Unlock()

	time.Sleep(20 * time.Millisecond)
	C.genie_paste()

	go m.restoreAfter(gen, 250*time.Millisecond)
	return nil
}

func (m *macInjector) restoreAfter(gen uint64, delay time.Duration) {
	time.Sleep(delay)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gen != gen {
		// A newer Type took over; let its goroutine handle the restore.
		return
	}
	s := C.CString(m.saved)
	C.genie_set_clipboard(s)
	C.free(unsafe.Pointer(s))
	m.saved = ""
	m.savedAt = 0
}

// Compile-time guarantee.
var _ Injector = (*macInjector)(nil)
