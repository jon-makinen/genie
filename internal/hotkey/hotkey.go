// Package hotkey registers a global system-wide keyboard shortcut and emits
// toggle events on the returned channel.
package hotkey

// Binding is a (macOS keycode + modifier flags) pair we ask Carbon to listen for.
type Binding struct {
	Code  uint16
	Cmd   bool
	Ctrl  bool
	Shift bool
	Alt   bool
}

// Toggler is the small surface used by the pipeline.
type Toggler interface {
	// Events emits one struct per keypress.
	Events() <-chan struct{}
	// Replace unregisters the current binding and registers a new one.
	Replace(b Binding) error
	Close()
}
