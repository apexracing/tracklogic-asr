package voice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apexracing/tracklogic-voice/assets"
	"github.com/apexracing/tracklogic-voice/internal/audio"
	"github.com/apexracing/tracklogic-voice/internal/g2p"
	"github.com/apexracing/tracklogic-voice/internal/kokoro"
)

// Synthesizer owns a Kokoro ONNX session. Close it when no longer needed.
type Synthesizer struct {
	engine   *kokoro.Engine
	frontend *g2p.Frontend
}

// NewSynthesizer prepares assets and creates a Chinese and US English
// synthesizer. This release performs CPU inference on windows/amd64.
func NewSynthesizer(ctx context.Context, cfg SynthesizerConfig) (*Synthesizer, error) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return nil, fmt.Errorf("this release supports windows/amd64 only")
	}
	paths, err := assets.PrepareTTS(ctx, cfg.Assets)
	if err != nil {
		return nil, err
	}
	engine, err := kokoro.NewEngine(paths.Model, paths.RuntimePath, cfg.NumThreads)
	if err != nil {
		return nil, err
	}
	return &Synthesizer{engine: engine, frontend: g2p.New()}, nil
}

// Synthesize converts Chinese, US English, or mixed text to 24 kHz mono
// float32 PCM.
func (s *Synthesizer) Synthesize(ctx context.Context, text string, opts SynthesisOptions) ([]float32, error) {
	if s == nil || s.engine == nil || s.frontend == nil {
		return nil, fmt.Errorf("synthesizer is not initialized")
	}
	language := string(opts.Language)
	phonemes, detected, err := s.frontend.Phonemize(text, language)
	if err != nil {
		return nil, err
	}
	return s.synthesizePhonemes(ctx, phonemes, detected, opts)
}

// SynthesizePhonemes converts already prepared Kokoro phonemes to 24 kHz mono
// float32 PCM. Language is used only for choosing the default voice.
func (s *Synthesizer) SynthesizePhonemes(ctx context.Context, phonemes string, opts SynthesisOptions) ([]float32, error) {
	phonemes = strings.TrimSpace(phonemes)
	if phonemes == "" {
		return nil, fmt.Errorf("phonemes are empty")
	}
	detected := string(opts.Language)
	switch detected {
	case "", g2p.LanguageAuto:
		detected = g2p.LanguageChinese
	case g2p.LanguageChinese, g2p.LanguageEnglish:
	default:
		return nil, fmt.Errorf("unsupported synthesis language %q", detected)
	}
	return s.synthesizePhonemes(ctx, phonemes, detected, opts)
}

func (s *Synthesizer) synthesizePhonemes(ctx context.Context, phonemes, detected string, opts SynthesisOptions) ([]float32, error) {
	if s == nil || s.engine == nil {
		return nil, fmt.Errorf("synthesizer is not initialized")
	}
	speed := opts.Speed
	if speed == 0 {
		speed = 1
	}
	if speed <= 0 || speed > 4 {
		return nil, fmt.Errorf("speed must be greater than 0 and at most 4")
	}
	voice := opts.Voice
	if voice == "" {
		voice = "zf_001"
		if detected == g2p.LanguageEnglish {
			voice = "af_maple"
		}
	}
	return s.engine.Synthesize(ctx, phonemes, voice, speed, opts.TrimSilence)
}

// SynthesizeFile writes synthesized text as a 24 kHz mono 16-bit PCM WAV.
func (s *Synthesizer) SynthesizeFile(ctx context.Context, text, path string, opts SynthesisOptions) error {
	samples, err := s.Synthesize(ctx, text, opts)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}
	return audio.WriteWAV(path, samples, SynthesisSampleRate)
}

// Voices lists the 103 voice names available in the aggregate voice archive.
func (s *Synthesizer) Voices() []string {
	if s == nil || s.engine == nil {
		return nil
	}
	return s.engine.Voices()
}

// Close releases the ONNX session, voice archive, and runtime reference.
func (s *Synthesizer) Close() error {
	if s == nil || s.engine == nil {
		return nil
	}
	return s.engine.Close()
}
