package vad

import (
	"encoding/binary"
	"fmt"

	"genie/internal/audio"

	webrtcvad "github.com/maxhawkins/go-webrtcvad"
)

// Mode controls VAD aggressiveness (0=loosest, 3=tightest).
const Mode = 2

// Utterance is a contiguous run of voiced frames flanked by silence.
type Utterance struct {
	Samples []int16
}

// Splitter buffers voiced frames and emits Utterance values when trailing
// silence exceeds SilenceMs. Hands-free mode: keep calling Push for every
// 30ms frame from the recorder, then read Utterances().
type Splitter struct {
	vad         *webrtcvad.VAD
	utterances  chan Utterance
	buf         []int16
	silenceCnt  int
	voicedCnt   int
	silenceMax  int // frames of silence that closes an utterance
	minVoiced   int // minimum voiced frames to bother emitting
	maxBuffered int // hard cap on buffered samples per utterance
	hasVoice    bool
}

// NewSplitter builds a splitter. silenceMs controls how much trailing silence
// closes an utterance (e.g. 500). Internally rounds to whole frames.
func NewSplitter(silenceMs int) (*Splitter, error) {
	v, err := webrtcvad.New()
	if err != nil {
		return nil, fmt.Errorf("init webrtc vad: %w", err)
	}
	if err := v.SetMode(Mode); err != nil {
		return nil, fmt.Errorf("set vad mode: %w", err)
	}
	if silenceMs < audio.FrameMs {
		silenceMs = audio.FrameMs
	}
	return &Splitter{
		vad:         v,
		utterances:  make(chan Utterance, 8),
		silenceMax:  silenceMs / audio.FrameMs,
		minVoiced:   8,                               // ~240ms minimum
		maxBuffered: audio.SampleRate * 30,           // 30s safety cap
	}, nil
}

// Utterances exposes the output channel.
func (s *Splitter) Utterances() <-chan Utterance { return s.utterances }

// Push feeds a 30ms int16 frame through VAD. Frames that don't match the
// expected size are dropped silently to keep the pipeline robust.
func (s *Splitter) Push(frame audio.Frame) {
	if len(frame) != audio.SamplesPerFrame {
		return
	}
	active, err := s.vad.Process(audio.SampleRate, int16ToBytes(frame))
	if err != nil {
		return
	}
	if active {
		s.silenceCnt = 0
		s.voicedCnt++
		s.hasVoice = true
		s.buf = append(s.buf, frame...)
	} else if s.hasVoice {
		s.buf = append(s.buf, frame...)
		s.silenceCnt++
		if s.silenceCnt >= s.silenceMax {
			s.flush()
		}
	}
	if len(s.buf) > s.maxBuffered {
		s.flush()
	}
}

// Flush emits any pending utterance. Useful when the recorder stops.
func (s *Splitter) Flush() { s.flush() }

// Close releases resources and closes the output channel.
func (s *Splitter) Close() {
	s.flush()
	close(s.utterances)
}

func (s *Splitter) flush() {
	if s.voicedCnt < s.minVoiced {
		s.reset()
		return
	}
	out := make([]int16, len(s.buf))
	copy(out, s.buf)
	select {
	case s.utterances <- Utterance{Samples: out}:
	default:
		// Drop on backpressure rather than block the recorder.
	}
	s.reset()
}

func (s *Splitter) reset() {
	s.buf = s.buf[:0]
	s.silenceCnt = 0
	s.voicedCnt = 0
	s.hasVoice = false
}

func int16ToBytes(in []int16) []byte {
	out := make([]byte, len(in)*2)
	for i, v := range in {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(v))
	}
	return out
}
