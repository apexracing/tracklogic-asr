package asr

import (
	"context"
	"fmt"
	"runtime"

	"github.com/apexracing/tracklogic-asr/assets"
	"github.com/apexracing/tracklogic-asr/internal/audio"
	engine "github.com/apexracing/tracklogic-asr/internal/sensevoice"
)

// Recognizer owns a SenseVoice ONNX session. Close it when no longer needed.
type Recognizer struct{ engine *engine.Engine }

// New prepares assets and creates a recognizer.
func New(ctx context.Context, cfg Config) (*Recognizer, error) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return nil, fmt.Errorf("this release supports windows/amd64 only")
	}
	paths, err := assets.Prepare(ctx, cfg.Assets)
	if err != nil {
		return nil, err
	}
	e, err := engine.NewEngine(paths.Model, paths.RuntimePath, cfg.NumThreads)
	if err != nil {
		return nil, err
	}
	return &Recognizer{engine: e}, nil
}

// TranscribeFile recognizes an uncompressed WAV file.
func (r *Recognizer) TranscribeFile(ctx context.Context, path string, opts Options) (Result, error) {
	samples, sampleRate, err := audio.ReadWAV(path)
	if err != nil {
		return Result{}, err
	}
	return r.Transcribe(ctx, samples, sampleRate, opts)
}

// Transcribe recognizes mono float32 PCM. Non-16 kHz input is resampled.
func (r *Recognizer) Transcribe(ctx context.Context, samples []float32, sampleRate int, opts Options) (Result, error) {
	decoded, err := r.engine.Transcribe(ctx, samples, sampleRate, string(opts.Language), opts.WithoutITN)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Text:     decoded.Text,
		Language: decoded.Language,
		Emotion:  decoded.Emotion,
		Events:   decoded.Events,
		Tokens:   decoded.Tokens,
		TokenIDs: decoded.TokenIDs,
	}, nil
}

// Close releases the ONNX session and its runtime reference.
func (r *Recognizer) Close() error { return r.engine.Close() }
