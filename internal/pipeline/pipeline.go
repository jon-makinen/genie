// Package pipeline wires the recorder -> VAD -> whisper -> magic-word filter
// -> text injector together. It owns the recording lifecycle: Start opens the
// mic, Stop closes it, ToggleRequested flips between the two.
package pipeline

import (
	"log"
	"strings"
	"sync"

	"genie/internal/audio"
	"genie/internal/config"
	"genie/internal/inject"
	"genie/internal/transcribe"
	"genie/internal/vad"
)

// StateListener observes recording transitions for UI/tray glue.
type StateListener func(recording bool)

// ErrorListener observes user-visible errors from the running pipeline (e.g.
// inject failures). Listeners are invoked synchronously, so they must not
// block.
type ErrorListener func(err error)

// Pipeline is the top-level orchestrator. It is safe to call Toggle/Start/Stop
// from any goroutine.
type Pipeline struct {
	store    *config.Store
	whisper  *transcribe.Whisper
	injector inject.Injector
	recorder *audio.Recorder

	mu        sync.Mutex
	recording bool
	stopping  bool
	stopCh    chan struct{}
	doneCh    chan struct{}

	listenersMu sync.Mutex
	listeners   []StateListener
	errListeners []ErrorListener
}

// New constructs a Pipeline. The whisper model and recorder must already be
// initialised; the recorder is reused across recording sessions.
func New(store *config.Store, whisper *transcribe.Whisper, injector inject.Injector, recorder *audio.Recorder) *Pipeline {
	return &Pipeline{store: store, whisper: whisper, injector: injector, recorder: recorder}
}

// OnState registers a state listener. Listeners are invoked synchronously.
func (p *Pipeline) OnState(fn StateListener) {
	p.listenersMu.Lock()
	defer p.listenersMu.Unlock()
	p.listeners = append(p.listeners, fn)
}

// OnError registers a listener for non-fatal user-visible errors.
func (p *Pipeline) OnError(fn ErrorListener) {
	p.listenersMu.Lock()
	defer p.listenersMu.Unlock()
	p.errListeners = append(p.errListeners, fn)
}

// Recording reports whether the pipeline is currently capturing audio.
func (p *Pipeline) Recording() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.recording
}

// Toggle flips between recording and idle.
func (p *Pipeline) Toggle() {
	if p.Recording() {
		p.Stop()
	} else {
		_ = p.Start()
	}
}

// Start opens the mic and begins transcribing. Idempotent.
func (p *Pipeline) Start() error {
	p.mu.Lock()
	if p.recording {
		p.mu.Unlock()
		return nil
	}

	if err := p.recorder.Start(); err != nil {
		p.mu.Unlock()
		return err
	}
	splitter, err := vad.NewSplitter(500)
	if err != nil {
		p.recorder.Stop()
		p.mu.Unlock()
		return err
	}

	p.recording = true
	p.stopCh = make(chan struct{})
	p.doneCh = make(chan struct{})
	p.mu.Unlock()

	p.notify(true)
	go p.run(splitter)
	return nil
}

// Stop halts the pipeline and waits for the worker to exit. Safe to call
// multiple times; concurrent callers all wait on the same teardown.
func (p *Pipeline) Stop() {
	p.mu.Lock()
	if !p.recording {
		p.mu.Unlock()
		return
	}
	first := !p.stopping
	p.stopping = true
	stop := p.stopCh
	done := p.doneCh
	p.mu.Unlock()

	if first {
		close(stop)
	}
	<-done
}

func (p *Pipeline) run(splitter *vad.Splitter) {
	defer func() {
		p.recorder.Stop()
		splitter.Close()
		p.mu.Lock()
		p.recording = false
		p.stopping = false
		p.mu.Unlock()
		p.notify(false)
		close(p.doneCh)
	}()

	frames := p.recorder.Frames()
	utterances := splitter.Utterances()

	for {
		select {
		case <-p.stopCh:
			// Flush whatever's buffered in the splitter and drain any pending
			// utterances so the trailing speech still gets transcribed.
			splitter.Flush()
			for {
				select {
				case u, ok := <-utterances:
					if !ok {
						return
					}
					p.handleUtterance(u.Samples)
				default:
					return
				}
			}
		case f, ok := <-frames:
			if !ok {
				return
			}
			splitter.Push(f)
		case u, ok := <-utterances:
			if !ok {
				return
			}
			p.handleUtterance(u.Samples)
		}
	}
}

func (p *Pipeline) handleUtterance(samples []int16) {
	text, err := p.whisper.Transcribe(audio.Int16ToFloat32(samples))
	if err != nil {
		log.Printf("whisper transcribe: %v", err)
		return
	}
	text = cleanWhisper(text)
	if text == "" {
		return
	}

	cfg := p.store.Get()
	prefix, magicHit := splitOnMagic(text, cfg.MagicWord)
	if prefix != "" {
		if err := p.injector.Type(prefix + " "); err != nil {
			log.Printf("inject: %v", err)
			p.notifyErr(err)
		}
	}
	if magicHit {
		go p.Stop()
	}
}

func (p *Pipeline) notify(recording bool) {
	p.listenersMu.Lock()
	listeners := append([]StateListener(nil), p.listeners...)
	p.listenersMu.Unlock()
	for _, fn := range listeners {
		fn(recording)
	}
}

func (p *Pipeline) notifyErr(err error) {
	p.listenersMu.Lock()
	listeners := append([]ErrorListener(nil), p.errListeners...)
	p.listenersMu.Unlock()
	for _, fn := range listeners {
		fn(err)
	}
}

// cleanWhisper drops bracketed markers and surrounding whitespace.
func cleanWhisper(text string) string {
	text = strings.TrimSpace(text)
	for _, marker := range []string{"[BLANK_AUDIO]", "[MUSIC]", "(silence)"} {
		text = strings.ReplaceAll(text, marker, "")
	}
	return strings.TrimSpace(text)
}

// splitOnMagic returns the prefix to inject and whether the magic word was hit.
// Comparison is case-insensitive on word boundaries.
func splitOnMagic(text, magic string) (string, bool) {
	if magic == "" {
		return text, false
	}
	lowered := strings.ToLower(text)
	target := strings.ToLower(magic)
	idx := indexWord(lowered, target)
	if idx < 0 {
		return text, false
	}
	return strings.TrimSpace(text[:idx]), true
}

// indexWord finds target as a whole word (separated by non-letter chars) in
// hay. -1 if not present.
func indexWord(hay, target string) int {
	if target == "" {
		return -1
	}
	start := 0
	for {
		i := strings.Index(hay[start:], target)
		if i < 0 {
			return -1
		}
		i += start
		left := i == 0 || !isWordChar(rune(hay[i-1]))
		end := i + len(target)
		right := end == len(hay) || !isWordChar(rune(hay[end]))
		if left && right {
			return i
		}
		start = i + 1
	}
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
