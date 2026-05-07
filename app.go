package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"genie/internal/audio"
	"genie/internal/config"
	"genie/internal/hotkey"
	"genie/internal/inject"
	"genie/internal/keycode"
	"genie/internal/launchatlogin"
	"genie/internal/modeldownload"
	"genie/internal/permissions"
	"genie/internal/pipeline"
	"genie/internal/transcribe"
	"genie/internal/tray"
)

// App is the Wails-bound application. It also owns the tray, hotkey, and
// pipeline lifecycles.
type App struct {
	ctx context.Context

	store    *config.Store
	whisper  *transcribe.Whisper
	pipeline *pipeline.Pipeline
	tray     tray.Tray
	hk       hotkey.Toggler
	dl       *modeldownload.Manager
	recorder *audio.Recorder
}

// NewApp wires construction-time dependencies that are safe to allocate
// before Wails is running. Heavy work (whisper load, tray, hotkey) happens in
// startup once the NSApp run loop is alive.
func NewApp() (*App, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &App{
		store: store,
		dl:    modeldownload.NewManager(),
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.tray = tray.New()
	go a.handleTrayEvents()

	hk, err := hotkey.New(bindingFromConfig(a.store.Get().Hotkey))
	if err != nil {
		log.Printf("hotkey register failed (Accessibility permission?): %v", err)
	}
	a.hk = hk
	if a.hk != nil {
		go a.handleHotkey()
	}

	rec, err := audio.NewRecorder()
	if err != nil {
		log.Printf("audio recorder init failed: %v", err)
	} else {
		a.recorder = rec
	}

	if err := a.tryLoadWhisper(); err != nil {
		log.Printf("whisper not loaded yet: %v", err)
	}
}

func (a *App) shutdown(_ context.Context) {
	if a.pipeline != nil {
		a.pipeline.Stop()
	}
	if a.hk != nil {
		a.hk.Close()
	}
	if a.tray != nil {
		a.tray.Close()
	}
	if a.whisper != nil {
		_ = a.whisper.Close()
	}
	if a.recorder != nil {
		a.recorder.Close()
	}
}

// tryLoadWhisper loads the configured model if it exists on disk and creates
// the pipeline. Safe to call repeatedly; later calls replace the previous
// pipeline.
func (a *App) tryLoadWhisper() error {
	cfg := a.store.Get()
	if !modeldownload.IsLocal(cfg.ModelName) {
		return fmt.Errorf("model %s not downloaded", cfg.ModelName)
	}
	if a.recorder == nil {
		return fmt.Errorf("audio recorder unavailable")
	}
	path, err := modeldownload.LocalPath(cfg.ModelName)
	if err != nil {
		return err
	}
	w, err := transcribe.New(path, cfg.Language)
	if err != nil {
		return err
	}
	if a.whisper != nil {
		_ = a.whisper.Close()
	}
	a.whisper = w
	a.pipeline = pipeline.New(a.store, w, inject.New(), a.recorder)
	a.pipeline.OnState(a.onPipelineState)
	a.pipeline.OnError(func(err error) {
		runtime.EventsEmit(a.ctx, "error", err.Error())
	})
	return nil
}

func (a *App) onPipelineState(recording bool) {
	if a.tray != nil {
		a.tray.SetRecording(recording)
	}
	runtime.EventsEmit(a.ctx, "recording_state", recording)
}

func (a *App) handleTrayEvents() {
	for {
		select {
		case <-a.tray.Toggles():
			a.toggleRecording()
		case <-a.tray.SettingsOpens():
			a.showSettings()
		case <-a.tray.Quits():
			runtime.Quit(a.ctx)
		}
	}
}

func (a *App) handleHotkey() {
	for range a.hk.Events() {
		// Always honor the hotkey to STOP an active recording, even if the user
		// disabled the hotkey afterwards — otherwise they can't end a session.
		if a.pipeline != nil && a.pipeline.Recording() {
			a.toggleRecording()
			continue
		}
		if !a.store.Get().HotkeyEnabled {
			continue
		}
		a.toggleRecording()
	}
}

func (a *App) toggleRecording() {
	if a.pipeline == nil {
		runtime.EventsEmit(a.ctx, "error", "Model not loaded — open Settings and download a model first.")
		a.showSettings()
		return
	}
	a.pipeline.Toggle()
}

// showSettings unhides the Wails window and yanks it to the front. Because
// LSUIElement apps have no Dock icon, WindowShow alone doesn't always reorder
// the window above other apps — toggling AlwaysOnTop forces a re-order without
// keeping the window pinned.
func (a *App) showSettings() {
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
}

// --- Methods bound to the frontend ---

// GetConfig returns the current settings.
func (a *App) GetConfig() config.Config { return a.store.Get() }

// SetConfig replaces the settings. Returns the persisted result.
func (a *App) SetConfig(in config.Config) (config.Config, error) {
	err := a.store.Update(func(c *config.Config) {
		c.ModelName = in.ModelName
		c.MagicWord = in.MagicWord
		c.HotkeyEnabled = in.HotkeyEnabled
		c.Language = in.Language
	})
	if err != nil {
		return config.Config{}, err
	}
	if a.whisper != nil {
		a.whisper.SetLanguage(in.Language)
	}
	return a.store.Get(), nil
}

// SetHotkey records a new global hotkey. eventCode is the W3C
// KeyboardEvent.code emitted by the frontend capture handler. Modifiers are
// the booleans from the same event. The hotkey is re-registered with macOS
// immediately and the resolved binding (with macOS keycode + label) is
// returned for the UI to display.
func (a *App) SetHotkey(eventCode string, cmd, ctrl, shift, alt bool) (config.Hotkey, error) {
	info, ok := keycode.FromEventCode(eventCode)
	if !ok {
		return config.Hotkey{}, fmt.Errorf("unsupported key: %s", eventCode)
	}
	hk := config.Hotkey{
		Code:  info.Code,
		Cmd:   cmd,
		Ctrl:  ctrl,
		Shift: shift,
		Alt:   alt,
		Label: formatHotkeyLabel(info.Label, cmd, ctrl, shift, alt),
	}
	if a.hk != nil {
		if err := a.hk.Replace(bindingFromConfig(hk)); err != nil {
			return config.Hotkey{}, fmt.Errorf("register: %w", err)
		}
	}
	if err := a.store.Update(func(c *config.Config) { c.Hotkey = hk }); err != nil {
		return config.Hotkey{}, err
	}
	return hk, nil
}

// bindingFromConfig converts the persisted Hotkey into a hotkey.Binding.
func bindingFromConfig(hk config.Hotkey) hotkey.Binding {
	return hotkey.Binding{
		Code:  hk.Code,
		Cmd:   hk.Cmd,
		Ctrl:  hk.Ctrl,
		Shift: hk.Shift,
		Alt:   hk.Alt,
	}
}

// formatHotkeyLabel renders e.g. "⌃⇧Space" or just "§".
func formatHotkeyLabel(key string, cmd, ctrl, shift, alt bool) string {
	prefix := ""
	if ctrl {
		prefix += "⌃"
	}
	if alt {
		prefix += "⌥"
	}
	if shift {
		prefix += "⇧"
	}
	if cmd {
		prefix += "⌘"
	}
	return prefix + key
}

// GetPermissions reports microphone + accessibility status.
func (a *App) GetPermissions() permissions.Status { return permissions.Check() }

// RequestMicrophone triggers the macOS microphone permission prompt.
func (a *App) RequestMicrophone() { permissions.RequestMicrophone() }

// PromptAccessibility opens the Accessibility prompt.
func (a *App) PromptAccessibility() { permissions.PromptAccessibility() }

// OpenAccessibilityPrefs jumps to System Settings > Privacy > Accessibility.
func (a *App) OpenAccessibilityPrefs() { permissions.OpenAccessibilityPrefs() }

// ResetAccessibility clears Genie's TCC accessibility entry and re-prompts.
// Used to recover from "checked in Settings but AXIsProcessTrusted returns
// false" after a rebuild changed the binary's cdhash. The settings UI surfaces
// any tccutil error so the user can see why nothing happened.
func (a *App) ResetAccessibility() error {
	if err := permissions.ResetAccessibility(); err != nil {
		runtime.EventsEmit(a.ctx, "error", err.Error())
		return err
	}
	return nil
}

// OpenMicrophonePrefs jumps to System Settings > Privacy > Microphone.
func (a *App) OpenMicrophonePrefs() { permissions.OpenMicrophonePrefs() }

// GetLaunchAtLogin reports the OS-level "Open at Login" state for this app.
func (a *App) GetLaunchAtLogin() launchatlogin.State { return launchatlogin.Get() }

// SetLaunchAtLogin toggles the OS-level "Open at Login" registration. The
// returned State reflects what the OS reports after the change (may show
// requires_approval=true the first time).
func (a *App) SetLaunchAtLogin(enable bool) (launchatlogin.State, error) {
	if err := launchatlogin.Set(enable); err != nil {
		return launchatlogin.Get(), err
	}
	return launchatlogin.Get(), nil
}

// ListModels returns the catalog with a per-row "downloaded" flag.
func (a *App) ListModels() []ModelEntry {
	out := make([]ModelEntry, 0, len(modeldownload.Catalog))
	for _, m := range modeldownload.Catalog {
		out = append(out, ModelEntry{
			ModelInfo:  m,
			Downloaded: modeldownload.IsLocal(m.Filename),
		})
	}
	return out
}

// ModelEntry combines catalog info with download state for the UI.
type ModelEntry struct {
	modeldownload.ModelInfo
	Downloaded bool `json:"downloaded"`
}

// DownloadModel fetches the named model in a goroutine and emits "model_progress" events.
func (a *App) DownloadModel(filename string) error {
	go func() {
		err := a.dl.Download(filename)
		if err != nil {
			log.Printf("download %s: %v", filename, err)
		}
		runtime.EventsEmit(a.ctx, "model_progress", a.dl.Progress())
		// Try to load whisper once the file lands.
		if err == nil {
			if loadErr := a.tryLoadWhisper(); loadErr != nil {
				log.Printf("post-download load: %v", loadErr)
			} else {
				runtime.EventsEmit(a.ctx, "model_loaded", filename)
			}
		}
	}()
	return nil
}

// ModelProgress lets the UI poll without waiting for an event.
func (a *App) ModelProgress() modeldownload.Progress { return a.dl.Progress() }

// CancelDownload aborts an in-flight download.
func (a *App) CancelDownload() { a.dl.Cancel() }

// ToggleRecording is the imperative version of the hotkey/tray toggle, exposed
// so the frontend can wire a button.
func (a *App) ToggleRecording() { a.toggleRecording() }

// IsRecording exposes pipeline state for the UI.
func (a *App) IsRecording() bool {
	if a.pipeline == nil {
		return false
	}
	return a.pipeline.Recording()
}

// HideWindow tucks the settings window away (tray is the primary surface).
func (a *App) HideWindow() { runtime.WindowHide(a.ctx) }

// QuitApp shuts the whole process down.
func (a *App) QuitApp() {
	a.shutdown(a.ctx)
	os.Exit(0)
}
