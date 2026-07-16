package sensevoice

import (
	"context"
	"fmt"
	"sync"

	"github.com/apexracing/tracklogic-voice/assets"
	"github.com/apexracing/tracklogic-voice/internal/ortutil"
	ort "github.com/yalue/onnxruntime_go"
)

var languageIDs = map[string]int32{
	"auto": 0, "zh": 3, "en": 4, "yue": 7,
	"ja": 11, "ko": 12, "nospeech": 13,
}

type Engine struct {
	mu       sync.Mutex
	session  *ort.DynamicAdvancedSession
	frontend *frontend
	decoder  *decoder
	closed   bool
}

func NewEngine(model assets.ModelPaths, runtimePath string, numThreads int) (*Engine, error) {
	fe, err := newFrontend(model.CMVN)
	if err != nil {
		return nil, err
	}
	dec, err := newDecoder(model.Tokens)
	if err != nil {
		return nil, err
	}
	if err = ortutil.Acquire(runtimePath); err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			ortutil.Release()
		}
	}()
	so, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("create ONNX session options: %w", err)
	}
	defer so.Destroy()
	if numThreads > 0 {
		if err = so.SetIntraOpNumThreads(numThreads); err != nil {
			return nil, fmt.Errorf("set ONNX threads: %w", err)
		}
	}
	if err = so.SetInterOpNumThreads(1); err != nil {
		return nil, fmt.Errorf("set ONNX inter-op threads: %w", err)
	}
	if err = so.SetExecutionMode(ort.ExecutionModeSequential); err != nil {
		return nil, fmt.Errorf("set ONNX execution mode: %w", err)
	}
	session, err := ort.NewDynamicAdvancedSession(model.Model,
		[]string{"speech", "speech_lengths", "language", "textnorm"},
		[]string{"ctc_logits", "encoder_out_lens"}, so)
	if err != nil {
		return nil, fmt.Errorf("load SenseVoice ONNX model: %w", err)
	}
	cleanup = false
	return &Engine{session: session, frontend: fe, decoder: dec}, nil
}

func (e *Engine) Transcribe(ctx context.Context, samples []float32, sampleRate int, language string, withoutITN bool) (DecodedResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return DecodedResult{}, fmt.Errorf("recognizer is closed")
	}
	if err := ctx.Err(); err != nil {
		return DecodedResult{}, err
	}
	if language == "" {
		language = "auto"
	}
	langID, ok := languageIDs[language]
	if !ok {
		return DecodedResult{}, fmt.Errorf("unsupported language %q", language)
	}
	features, frames, err := e.frontend.extract(samples, sampleRate)
	if err != nil {
		return DecodedResult{}, err
	}
	textnorm := int32(14)
	if withoutITN {
		textnorm = 15
	}
	values := make([]ort.Value, 4)
	values[0], err = ort.NewTensor(ort.NewShape(1, int64(frames), featureDim), features)
	if err != nil {
		return DecodedResult{}, err
	}
	defer values[0].Destroy()
	values[1], err = ort.NewTensor(ort.NewShape(1), []int32{int32(frames)})
	if err != nil {
		return DecodedResult{}, err
	}
	defer values[1].Destroy()
	values[2], err = ort.NewTensor(ort.NewShape(1), []int32{langID})
	if err != nil {
		return DecodedResult{}, err
	}
	defer values[2].Destroy()
	values[3], err = ort.NewTensor(ort.NewShape(1), []int32{textnorm})
	if err != nil {
		return DecodedResult{}, err
	}
	defer values[3].Destroy()
	outputs := []ort.Value{nil, nil}
	if err = e.session.Run(values, outputs); err != nil {
		return DecodedResult{}, fmt.Errorf("run SenseVoice: %w", err)
	}
	for _, output := range outputs {
		if output != nil {
			defer output.Destroy()
		}
	}
	logits, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return DecodedResult{}, fmt.Errorf("unexpected logits type %T", outputs[0])
	}
	shape := logits.GetShape()
	if len(shape) != 3 {
		return DecodedResult{}, fmt.Errorf("unexpected logits shape %v", shape)
	}
	result, err := e.decoder.decode(logits.GetData(), int(shape[1]), int(shape[2]))
	if err != nil {
		return DecodedResult{}, err
	}
	if err = ctx.Err(); err != nil {
		return DecodedResult{}, err
	}
	return result, nil
}

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	err := e.session.Destroy()
	ortutil.Release()
	return err
}
