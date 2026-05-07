// Package transcribe wraps the whisper.cpp Go bindings with a simple
// "transcribe this chunk of float32 PCM" API, plus model loading.
package transcribe

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// Whisper holds a loaded model and serializes inference calls.
type Whisper struct {
	mu       sync.Mutex
	model    whisper.Model
	path     string
	language string
}

// New loads the GGML model at path. language may be "auto" or an ISO code
// recognized by whisper (e.g. "en", "fi").
func New(path, language string) (*Whisper, error) {
	if path == "" {
		return nil, errors.New("model path is empty")
	}
	model, err := whisper.New(path)
	if err != nil {
		return nil, fmt.Errorf("load whisper model: %w", err)
	}
	return &Whisper{model: model, path: path, language: language}, nil
}

// SetLanguage changes the language used for subsequent transcribe calls.
func (w *Whisper) SetLanguage(lang string) {
	w.mu.Lock()
	w.language = lang
	w.mu.Unlock()
}

// Transcribe runs Whisper on a chunk of mono 16kHz float32 audio and returns
// the concatenated text. Empty input returns "" without invoking the model.
func (w *Whisper) Transcribe(samples []float32) (string, error) {
	if len(samples) == 0 {
		return "", nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	ctx, err := w.model.NewContext()
	if err != nil {
		return "", fmt.Errorf("new whisper context: %w", err)
	}
	if w.language != "" && w.language != "auto" {
		_ = ctx.SetLanguage(w.language)
	}
	ctx.SetTranslate(false)
	ctx.SetThreads(uint(threadCount()))

	if err := ctx.Process(samples, nil, nil, nil); err != nil {
		return "", fmt.Errorf("whisper process: %w", err)
	}

	var b strings.Builder
	for {
		seg, err := ctx.NextSegment()
		if err != nil {
			break
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strings.TrimSpace(seg.Text))
	}
	return b.String(), nil
}

// Close releases the model.
func (w *Whisper) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.model != nil {
		err := w.model.Close()
		w.model = nil
		return err
	}
	return nil
}

func threadCount() int {
	n := runtime.NumCPU()
	if n > 8 {
		return 8
	}
	if n < 1 {
		return 1
	}
	return n
}
