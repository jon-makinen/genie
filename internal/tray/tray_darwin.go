//go:build darwin

package tray

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdint.h>

void genie_tray_create(void);
void genie_tray_set_recording(int on);
*/
import "C"

type macTray struct {
	toggles  chan struct{}
	settings chan struct{}
	quits    chan struct{}
}

var instance *macTray

// New creates the NSStatusItem on the main thread. May only be called once.
func New() Tray {
	if instance != nil {
		return instance
	}
	instance = &macTray{
		toggles:  make(chan struct{}, 4),
		settings: make(chan struct{}, 4),
		quits:    make(chan struct{}, 4),
	}
	C.genie_tray_create()
	return instance
}

func (m *macTray) SetRecording(on bool) {
	if on {
		C.genie_tray_set_recording(1)
	} else {
		C.genie_tray_set_recording(0)
	}
}

func (m *macTray) Toggles() <-chan struct{}       { return m.toggles }
func (m *macTray) SettingsOpens() <-chan struct{} { return m.settings }
func (m *macTray) Quits() <-chan struct{}         { return m.quits }

func (m *macTray) Close() {}

//export genieTrayToggle
func genieTrayToggle() {
	if instance == nil {
		return
	}
	select {
	case instance.toggles <- struct{}{}:
	default:
	}
}

//export genieTraySettings
func genieTraySettings() {
	if instance == nil {
		return
	}
	select {
	case instance.settings <- struct{}{}:
	default:
	}
}

//export genieTrayQuit
func genieTrayQuit() {
	if instance == nil {
		return
	}
	select {
	case instance.quits <- struct{}{}:
	default:
	}
}
