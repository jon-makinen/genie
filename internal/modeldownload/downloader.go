// Package modeldownload fetches Whisper GGML model files from Hugging Face.
package modeldownload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"genie/internal/config"
)

// Catalog enumerates the models the UI exposes. URLs point to Hugging Face's
// official ggerganov/whisper.cpp repository.
var Catalog = []ModelInfo{
	{
		Filename: "ggml-large-v3-turbo-q5_0.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo-q5_0.bin",
		Label:    "large-v3-turbo (q5_0, ~870 MB)",
	},
	{
		Filename: "ggml-medium.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
		Label:    "medium (1.5 GB)",
	},
	{
		Filename: "ggml-small.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
		Label:    "small (466 MB)",
	},
}

// ModelInfo describes one downloadable model.
type ModelInfo struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Label    string `json:"label"`
}

// Progress is what the UI polls to draw a progress bar.
type Progress struct {
	Filename   string `json:"filename"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Done       bool   `json:"done"`
	Err        string `json:"error,omitempty"`
}

// Manager coordinates downloads with progress reporting and cancellation.
type Manager struct {
	mu       sync.Mutex
	progress atomic.Pointer[Progress]
	cancel   context.CancelFunc
}

// NewManager constructs a Manager with no active download.
func NewManager() *Manager { return &Manager{} }

// Progress returns a copy of the latest progress report (zero-valued if none).
func (m *Manager) Progress() Progress {
	p := m.progress.Load()
	if p == nil {
		return Progress{}
	}
	return *p
}

// LocalPath returns the on-disk path for a model filename, regardless of
// whether it exists yet.
func LocalPath(filename string) (string, error) {
	dir, err := config.ModelsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filename), nil
}

// IsLocal reports whether the file exists on disk and is non-empty.
func IsLocal(filename string) bool {
	p, err := LocalPath(filename)
	if err != nil {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.Size() > 0
}

// Cancel aborts an in-flight download (if any).
func (m *Manager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}

// Download fetches the named model and reports progress via Progress(). It is
// safe to call again after completion; calling while a download is in progress
// returns an error.
func (m *Manager) Download(filename string) error {
	info := lookup(filename)
	if info == nil {
		return fmt.Errorf("unknown model %q", filename)
	}

	dir, err := config.ModelsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, filename)
	tmp := dest + ".part"

	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return errors.New("download already in progress")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.cancel = nil
		m.mu.Unlock()
	}()

	m.progress.Store(&Progress{Filename: filename})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	if err != nil {
		m.fail(filename, err)
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		m.fail(filename, err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("http %d", resp.StatusCode)
		m.fail(filename, err)
		return err
	}

	out, err := os.Create(tmp)
	if err != nil {
		m.fail(filename, err)
		return err
	}

	pw := &progressWriter{m: m, filename: filename, total: resp.ContentLength}
	if _, err := io.Copy(out, io.TeeReader(resp.Body, pw)); err != nil {
		out.Close()
		os.Remove(tmp)
		m.fail(filename, err)
		return err
	}
	if err := out.Close(); err != nil {
		m.fail(filename, err)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		m.fail(filename, err)
		return err
	}

	m.progress.Store(&Progress{Filename: filename, Downloaded: pw.total, Total: pw.total, Done: true})
	return nil
}

func (m *Manager) fail(filename string, err error) {
	m.progress.Store(&Progress{Filename: filename, Err: err.Error(), Done: true})
}

func lookup(filename string) *ModelInfo {
	for i := range Catalog {
		if Catalog[i].Filename == filename {
			return &Catalog[i]
		}
	}
	return nil
}

type progressWriter struct {
	m        *Manager
	filename string
	written  int64
	total    int64
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.written += int64(n)
	p.m.progress.Store(&Progress{
		Filename:   p.filename,
		Downloaded: p.written,
		Total:      p.total,
	})
	return n, nil
}
