// Package tray manages the macOS menu bar (NSStatusItem) icon and menu.
package tray

// Tray is the small surface the rest of the app uses.
type Tray interface {
	// SetRecording flips the icon between idle (mic) and recording (filled dot).
	SetRecording(on bool)
	// Toggles emits one event per "Toggle Recording" click.
	Toggles() <-chan struct{}
	// SettingsOpens emits when the user picks "Settings…".
	SettingsOpens() <-chan struct{}
	// Quits emits when the user picks "Quit".
	Quits() <-chan struct{}
	Close()
}
