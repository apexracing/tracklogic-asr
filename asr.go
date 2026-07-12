package sensevoice

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

type Language string

const (
	LanguageAuto      Language = "auto"
	LanguageChinese   Language = "zh"
	LanguageEnglish   Language = "en"
	LanguageCantonese Language = "yue"
	LanguageJapanese  Language = "ja"
	LanguageKorean    Language = "ko"
	LanguageNoSpeech  Language = "nospeech"
)

var languageIDs = map[Language]int32{
	LanguageAuto: 0, LanguageChinese: 3, LanguageEnglish: 4,
	LanguageCantonese: 7, LanguageJapanese: 11, LanguageKorean: 12,
	LanguageNoSpeech: 13,
}

type Config struct {
	// ModelDir uses pre-downloaded files. When empty, the pinned INT8 model is
	// downloaded to CacheDir automatically.
	ModelDir string
	CacheDir string
	// RuntimePath is the path to onnxruntime.dll. When empty, the embedded
	// Microsoft CPU runtime is verified and extracted into the cache.
	RuntimePath     string
	RuntimeCacheDir string
	NumThreads      int
	Progress        func(name string, downloaded, total int64)
}

type Options struct {
	Language Language
	// WithoutITN disables inverse text normalization and punctuation.
	WithoutITN bool
}

type Result struct {
	Text     string
	Language string
	Emotion  string
	Events   []string
	Tokens   []string
	TokenIDs []int
}

type Recognizer struct {
	mu       sync.Mutex
	session  *ort.DynamicAdvancedSession
	frontend *frontend
	decoder  *decoder
	closed   bool
}

var runtimeState struct {
	sync.Mutex
	users int
	path  string
	owned bool
}

func New(ctx context.Context, cfg Config) (*Recognizer, error) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return nil, fmt.Errorf("this release supports windows/amd64 only")
	}
	var paths ModelPaths
	var err error
	if cfg.ModelDir == "" {
		paths, err = EnsureModel(ctx, cfg.CacheDir, cfg.Progress)
		if err != nil {
			return nil, err
		}
	} else {
		paths = modelPaths(cfg.ModelDir)
		for _, path := range []string{paths.Model, paths.Tokens, paths.CMVN} {
			if _, statErr := os.Stat(path); statErr != nil {
				return nil, fmt.Errorf("required model file %s: %w", path, statErr)
			}
		}
	}

	fe, err := newFrontend(paths.CMVN)
	if err != nil {
		return nil, err
	}
	dec, err := newDecoder(paths.Tokens)
	if err != nil {
		return nil, err
	}
	runtimePath := cfg.RuntimePath
	if runtimePath == "" {
		runtimePath, err = EnsureRuntime(ctx, cfg.RuntimeCacheDir, cfg.Progress)
		if err != nil {
			return nil, err
		}
	}
	if err = acquireRuntime(runtimePath); err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			releaseRuntime()
		}
	}()

	so, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("create ONNX session options: %w", err)
	}
	defer so.Destroy()
	if cfg.NumThreads > 0 {
		if err = so.SetIntraOpNumThreads(cfg.NumThreads); err != nil {
			return nil, fmt.Errorf("set ONNX threads: %w", err)
		}
	}
	session, err := ort.NewDynamicAdvancedSession(paths.Model,
		[]string{"speech", "speech_lengths", "language", "textnorm"},
		[]string{"ctc_logits", "encoder_out_lens"}, so)
	if err != nil {
		return nil, fmt.Errorf("load SenseVoice ONNX model: %w", err)
	}
	cleanup = false
	return &Recognizer{session: session, frontend: fe, decoder: dec}, nil
}

func acquireRuntime(path string) error {
	runtimeState.Lock()
	defer runtimeState.Unlock()
	if runtimeState.users > 0 {
		if runtimeState.path != path {
			return fmt.Errorf("ONNX Runtime already initialized from %s", runtimeState.path)
		}
		runtimeState.users++
		return nil
	}
	if ort.IsInitialized() {
		runtimeState.users = 1
		runtimeState.path = path
		runtimeState.owned = false
		return nil
	}
	ort.SetSharedLibraryPath(path)
	if err := ort.InitializeEnvironment(ort.WithLogLevelWarning()); err != nil {
		return fmt.Errorf("initialize ONNX Runtime from %s: %w", path, err)
	}
	runtimeState.users = 1
	runtimeState.path = path
	runtimeState.owned = true
	return nil
}

func releaseRuntime() {
	runtimeState.Lock()
	defer runtimeState.Unlock()
	if runtimeState.users == 0 {
		return
	}
	runtimeState.users--
	if runtimeState.users == 0 {
		if runtimeState.owned {
			_ = ort.DestroyEnvironment()
		}
		runtimeState.path = ""
		runtimeState.owned = false
	}
}

func (r *Recognizer) TranscribeFile(ctx context.Context, path string, opts Options) (Result, error) {
	samples, sampleRate, err := ReadWAV(path)
	if err != nil {
		return Result{}, err
	}
	return r.Transcribe(ctx, samples, sampleRate, opts)
}

func (r *Recognizer) Transcribe(ctx context.Context, samples []float32, sampleRate int, opts Options) (Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return Result{}, fmt.Errorf("recognizer is closed")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	lang := opts.Language
	if lang == "" {
		lang = LanguageAuto
	}
	langID, ok := languageIDs[lang]
	if !ok {
		return Result{}, fmt.Errorf("unsupported language %q", lang)
	}
	features, frames, err := r.frontend.extract(samples, sampleRate)
	if err != nil {
		return Result{}, err
	}
	textnorm := int32(14)
	if opts.WithoutITN {
		textnorm = 15
	}
	values := make([]ort.Value, 4)
	values[0], err = ort.NewTensor(ort.NewShape(1, int64(frames), featureDim), features)
	if err != nil {
		return Result{}, err
	}
	defer values[0].Destroy()
	values[1], err = ort.NewTensor(ort.NewShape(1), []int32{int32(frames)})
	if err != nil {
		return Result{}, err
	}
	defer values[1].Destroy()
	values[2], err = ort.NewTensor(ort.NewShape(1), []int32{langID})
	if err != nil {
		return Result{}, err
	}
	defer values[2].Destroy()
	values[3], err = ort.NewTensor(ort.NewShape(1), []int32{textnorm})
	if err != nil {
		return Result{}, err
	}
	defer values[3].Destroy()

	outputs := []ort.Value{nil, nil}
	if err = r.session.Run(values, outputs); err != nil {
		return Result{}, fmt.Errorf("run SenseVoice: %w", err)
	}
	for _, output := range outputs {
		if output != nil {
			defer output.Destroy()
		}
	}
	logits, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return Result{}, fmt.Errorf("unexpected logits type %T", outputs[0])
	}
	shape := logits.GetShape()
	if len(shape) != 3 {
		return Result{}, fmt.Errorf("unexpected logits shape %v", shape)
	}
	result, err := r.decoder.decode(logits.GetData(), int(shape[1]), int(shape[2]))
	if err != nil {
		return Result{}, err
	}
	if err = ctx.Err(); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (r *Recognizer) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	err := r.session.Destroy()
	releaseRuntime()
	return err
}
