package kokoro

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/apexracing/tracklogic-voice/assets"
	"github.com/apexracing/tracklogic-voice/internal/ortutil"
	ort "github.com/yalue/onnxruntime_go"
)

const SampleRate = 24000

// Engine owns a Kokoro ONNX session and voice archive.
type Engine struct {
	mu        sync.Mutex
	session   *ort.DynamicAdvancedSession
	tokenizer *tokenizer
	voices    *voiceArchive
	closed    bool
}

func NewEngine(model assets.TTSModelPaths, runtimePath string, numThreads int) (*Engine, error) {
	tok, err := loadTokenizer(model.Tokenizer)
	if err != nil {
		return nil, err
	}
	voices, err := openVoiceArchive(model.Voices)
	if err != nil {
		return nil, err
	}
	if err = ortutil.Acquire(runtimePath); err != nil {
		voices.close()
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			ortutil.Release()
			voices.close()
		}
	}()
	so, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("create Kokoro session options: %w", err)
	}
	defer so.Destroy()
	if numThreads > 0 {
		if err = so.SetIntraOpNumThreads(numThreads); err != nil {
			return nil, fmt.Errorf("set Kokoro threads: %w", err)
		}
	}
	if err = so.SetInterOpNumThreads(1); err != nil {
		return nil, fmt.Errorf("set Kokoro inter-op threads: %w", err)
	}
	if err = so.SetExecutionMode(ort.ExecutionModeSequential); err != nil {
		return nil, fmt.Errorf("set Kokoro execution mode: %w", err)
	}
	session, err := ort.NewDynamicAdvancedSession(model.Model,
		[]string{"input_ids", "style", "speed"}, []string{"waveform"}, so)
	if err != nil {
		return nil, fmt.Errorf("load Kokoro ONNX model: %w", err)
	}
	cleanup = false
	return &Engine{session: session, tokenizer: tok, voices: voices}, nil
}

func (e *Engine) Voices() []string { return e.voices.names() }

func (e *Engine) Synthesize(ctx context.Context, phonemes, voice string, speed float32, trim bool) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, fmt.Errorf("synthesizer is closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids, err := e.tokenizer.tokenize(phonemes)
	if err != nil {
		return nil, err
	}
	chunks := chunkTokens(ids, voiceRows-1)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("text produced no Kokoro tokens")
	}
	var output []float32
	for _, chunk := range chunks {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		style, err := e.voices.style(voice, len(chunk))
		if err != nil {
			return nil, err
		}
		inputData := make([]int64, len(chunk)+2)
		copy(inputData[1:], chunk)
		input, err := ort.NewTensor(ort.NewShape(1, int64(len(inputData))), inputData)
		if err != nil {
			return nil, err
		}
		styleTensor, err := ort.NewTensor(ort.NewShape(1, styleDim), style)
		if err != nil {
			input.Destroy()
			return nil, err
		}
		speedTensor, err := ort.NewTensor(ort.NewShape(1), []float32{speed})
		if err != nil {
			input.Destroy()
			styleTensor.Destroy()
			return nil, err
		}
		values := []ort.Value{input, styleTensor, speedTensor}
		outputs := []ort.Value{nil}
		err = e.session.Run(values, outputs)
		input.Destroy()
		styleTensor.Destroy()
		speedTensor.Destroy()
		if err != nil {
			return nil, fmt.Errorf("run Kokoro: %w", err)
		}
		wave, ok := outputs[0].(*ort.Tensor[float32])
		if !ok {
			if outputs[0] != nil {
				outputs[0].Destroy()
			}
			return nil, fmt.Errorf("unexpected Kokoro waveform type %T", outputs[0])
		}
		part := append([]float32(nil), wave.GetData()...)
		wave.Destroy()
		if trim {
			part = trimSilence(part)
		}
		output = append(output, part...)
	}
	return output, nil
}

func trimSilence(samples []float32) []float32 {
	const threshold = float32(0.001)
	const padding = 120
	start, end := 0, len(samples)
	for start < end && float32(math.Abs(float64(samples[start]))) < threshold {
		start++
	}
	for end > start && float32(math.Abs(float64(samples[end-1]))) < threshold {
		end--
	}
	if start == end {
		return nil
	}
	start = max(0, start-padding)
	end = min(len(samples), end+padding)
	return append([]float32(nil), samples[start:end]...)
}

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	err := e.session.Destroy()
	if closeErr := e.voices.close(); err == nil {
		err = closeErr
	}
	ortutil.Release()
	return err
}
