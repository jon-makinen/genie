package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/gen2brain/malgo"
)

const (
	// SampleRate is what whisper expects.
	SampleRate = 16000
	// FrameMs sized so 16kHz * 30ms / 1000 = 480 samples — the WebRTC VAD frame size.
	FrameMs = 30
	// SamplesPerFrame is convenience for downstream consumers.
	SamplesPerFrame = SampleRate * FrameMs / 1000
)

// Frame is a chunk of int16 PCM samples (mono, 16kHz, ~30ms).
type Frame []int16

// Recorder captures microphone audio and emits 30ms PCM int16 frames on Frames().
type Recorder struct {
	ctx     *malgo.AllocatedContext
	device  *malgo.Device
	frames  chan Frame
	mu      sync.Mutex
	buf     []int16
	running bool
}

// NewRecorder allocates a malgo context. Call Close when done.
func NewRecorder() (*Recorder, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {})
	if err != nil {
		return nil, fmt.Errorf("init malgo context: %w", err)
	}
	return &Recorder{ctx: ctx, frames: make(chan Frame, 64)}, nil
}

// Frames returns a read-only channel of mic frames. Closed when the recorder
// stops or fails.
func (r *Recorder) Frames() <-chan Frame { return r.frames }

// Start opens the default capture device and begins streaming frames.
func (r *Recorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return nil
	}

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = SampleRate
	cfg.Alsa.NoMMap = 1

	device, err := malgo.InitDevice(r.ctx.Context, cfg, malgo.DeviceCallbacks{
		Data: r.onData,
	})
	if err != nil {
		return fmt.Errorf("init capture device: %w", err)
	}
	if err := device.Start(); err != nil {
		device.Uninit()
		return fmt.Errorf("start capture device: %w", err)
	}
	r.device = device
	r.running = true
	return nil
}

// Stop halts capture, flushes any partial frame, and drains stale frames so
// the next Start begins from a clean buffer.
func (r *Recorder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return
	}
	r.device.Uninit()
	r.device = nil
	r.running = false
	r.buf = r.buf[:0]
	for {
		select {
		case <-r.frames:
		default:
			return
		}
	}
}

// Close releases all resources. The Frames channel is closed.
func (r *Recorder) Close() {
	r.Stop()
	if r.ctx != nil {
		_ = r.ctx.Uninit()
		r.ctx.Free()
		r.ctx = nil
	}
	close(r.frames)
}

// onData is the malgo capture callback: bytes are little-endian int16.
func (r *Recorder) onData(_, in []byte, frameCount uint32) {
	if frameCount == 0 || len(in) == 0 {
		return
	}
	samples := bytesToInt16(in)
	r.mu.Lock()
	r.buf = append(r.buf, samples...)
	for len(r.buf) >= SamplesPerFrame {
		frame := make(Frame, SamplesPerFrame)
		copy(frame, r.buf[:SamplesPerFrame])
		r.buf = r.buf[SamplesPerFrame:]
		select {
		case r.frames <- frame:
		default:
			// Drop frame on backpressure rather than block the audio thread.
		}
	}
	r.mu.Unlock()
}

func bytesToInt16(b []byte) []int16 {
	n := len(b) / 2
	out := make([]int16, n)
	for i := 0; i < n; i++ {
		out[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return out
}

// Int16ToFloat32 normalizes PCM samples to [-1, 1] for whisper.
func Int16ToFloat32(in []int16) []float32 {
	out := make([]float32, len(in))
	for i, s := range in {
		out[i] = float32(s) / float32(math.MaxInt16)
	}
	return out
}
