package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Hotkey is a persisted global keyboard shortcut. Code is the macOS virtual
// keycode (kVK_* from HIToolbox); the booleans are modifier flags.
type Hotkey struct {
	Code  uint16 `json:"code"`
	Cmd   bool   `json:"cmd"`
	Ctrl  bool   `json:"ctrl"`
	Shift bool   `json:"shift"`
	Alt   bool   `json:"alt"`
	Label string `json:"label"`
}

// Config holds user-tweakable settings persisted to disk.
type Config struct {
	ModelName     string `json:"model_name"`
	MagicWord     string `json:"magic_word"`
	HotkeyEnabled bool   `json:"hotkey_enabled"`
	Hotkey        Hotkey `json:"hotkey"`
	Language      string `json:"language"`
}

// Default values used when a fresh config is created.
func defaults() Config {
	return Config{
		ModelName:     "ggml-large-v3-turbo-q5_0.bin",
		MagicWord:     "genie",
		HotkeyEnabled: true,
		Hotkey:        Hotkey{Code: 0x0A, Label: "§"},
		Language:      "auto",
	}
}

// Store wraps a Config with thread-safe load/save backed by a JSON file under
// ~/Library/Application Support/Genie/config.json.
type Store struct {
	path string
	mu   sync.RWMutex
	cfg  Config
}

// NewStore loads (or seeds) the config file and returns an initialized Store.
func NewStore() (*Store, error) {
	dir, err := AppSupportDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	s := &Store{path: filepath.Join(dir, "config.json"), cfg: defaults()}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// AppSupportDir returns the per-user data directory used by Genie.
func AppSupportDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "Genie"), nil
}

// ModelsDir returns the directory where Whisper model files live.
func ModelsDir() (string, error) {
	base, err := AppSupportDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "models"), nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s.save()
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	// Unmarshal onto the defaults so missing JSON keys keep their zero-friendly
	// default values (e.g. HotkeyEnabled stays true if the key isn't present).
	cfg := defaults()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	s.cfg = cfg
	return nil
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// Get returns a copy of the current config.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Update applies a mutator and persists the result.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.cfg)
	return s.save()
}
