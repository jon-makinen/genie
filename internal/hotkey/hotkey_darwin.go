//go:build darwin

package hotkey

import (
	"fmt"
	"sync"

	"golang.design/x/hotkey"
)

type macHotkey struct {
	mu      sync.Mutex
	hk      *hotkey.Hotkey
	events  chan struct{}
	stop    chan struct{}
	current Binding
}

// New registers the given binding as a macOS global hotkey. Pass an empty
// Binding (Code == 0) to start without a binding — callers can register one
// later via Replace.
func New(initial Binding) (Toggler, error) {
	m := &macHotkey{
		events: make(chan struct{}, 4),
		stop:   make(chan struct{}),
	}
	if initial.Code != 0 {
		if err := m.Replace(initial); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *macHotkey) Events() <-chan struct{} { return m.events }

func (m *macHotkey) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-m.stop:
	default:
		close(m.stop)
	}
	if m.hk != nil {
		_ = m.hk.Unregister()
		m.hk = nil
	}
}

// Replace unregisters the current hotkey (if any), registers a new one, and
// starts a fresh listener goroutine. Idempotent if the binding is unchanged.
func (m *macHotkey) Replace(b Binding) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if b == m.current && m.hk != nil {
		return nil
	}

	if m.hk != nil {
		_ = m.hk.Unregister()
		m.hk = nil
		// Tell the previous listener goroutine to exit; we'll spawn a new one.
		close(m.stop)
		m.stop = make(chan struct{})
	}

	mods := bindingMods(b)
	hk := hotkey.New(mods, hotkey.Key(b.Code))
	if err := hk.Register(); err != nil {
		return fmt.Errorf("register hotkey: %w", err)
	}
	m.hk = hk
	m.current = b

	go m.loop(hk, m.stop)
	return nil
}

func (m *macHotkey) loop(hk *hotkey.Hotkey, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case _, ok := <-hk.Keydown():
			if !ok {
				return
			}
			select {
			case m.events <- struct{}{}:
			default:
			}
		}
	}
}

func bindingMods(b Binding) []hotkey.Modifier {
	var out []hotkey.Modifier
	if b.Cmd {
		out = append(out, hotkey.ModCmd)
	}
	if b.Ctrl {
		out = append(out, hotkey.ModCtrl)
	}
	if b.Shift {
		out = append(out, hotkey.ModShift)
	}
	if b.Alt {
		out = append(out, hotkey.ModOption)
	}
	return out
}
