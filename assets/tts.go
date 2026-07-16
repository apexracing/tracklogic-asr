package assets

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	ModelScopeTTSRepository  = "huntsman/Kokoro-82M-v1.1-zh-ONNX"
	ModelScopeTTSRevision    = "80d48ea07e671ec7de0f3f59c32941d3a5c00e53"
	HuggingFaceTTSRepository = "tracklogic/Kokoro-82M-v1.1-zh-ONNX"
	HuggingFaceTTSRevision   = "f78bc7bcc7b3646cbf899829ca31cc5c852bbb31"
	ttsManifestRevision      = "manifest-v1"
)

var defaultTTSFiles = []modelFile{
	{"onnx/model_quantized.onnx", "a39469be791eeaa3089c1ed5e58b8731d1f2462ea0e7dae2bc44388e58f973d8"},
	{"voices/voices-v1.1-zh.bin", "14cb6186c99e4f6016871405f62046c5df863ae27465cbdc4ee08be7dd703acd"},
	{"tokenizer.json", "5715a60b09d5e4b9074435d68c6ccd5675b9d48b220e109fdea3cda681e23d15"},
	{"tokenizer_config.json", "be1cb066d6ef6b074b3f15e6a6dd21ac88ff3cdaedf325f0aaed686c70f75d20"},
	{"config.json", "df34b4f930b23447cd4dc410fabfb42eb3f24e803e6c3f97d618fb359380a36f"},
}

var ttsMu sync.Mutex

// TTSConfig controls Kokoro paths, cache locations, and progress reporting.
type TTSConfig struct {
	ModelDir        string
	ModelCacheDir   string
	ModelSource     ModelSource
	RuntimePath     string
	RuntimeCacheDir string
	Progress        ProgressFunc
}

// TTSModelPaths contains the files required by Kokoro.
type TTSModelPaths struct {
	Directory       string
	Model           string
	Voices          string
	Tokenizer       string
	TokenizerConfig string
	Config          string
}

// TTSPaths contains verified Kokoro and runtime paths.
type TTSPaths struct {
	Model       TTSModelPaths
	RuntimePath string
}

// PrepareTTS resolves custom paths or downloads the pinned Kokoro assets.
func PrepareTTS(ctx context.Context, cfg TTSConfig) (TTSPaths, error) {
	runtimePath := cfg.RuntimePath
	var err error
	if runtimePath == "" {
		runtimePath, err = EnsureRuntime(ctx, cfg.RuntimeCacheDir, cfg.Progress)
		if err != nil {
			return TTSPaths{}, err
		}
	} else if _, err = os.Stat(runtimePath); err != nil {
		return TTSPaths{}, fmt.Errorf("required runtime file %s: %w", runtimePath, err)
	}

	var model TTSModelPaths
	if cfg.ModelDir == "" {
		model, err = EnsureTTSModelFrom(ctx, cfg.ModelCacheDir, cfg.ModelSource, cfg.Progress)
		if err != nil {
			return TTSPaths{}, err
		}
	} else {
		model = ttsModelPaths(cfg.ModelDir)
		for _, path := range []string{model.Model, model.Voices, model.Tokenizer, model.TokenizerConfig, model.Config} {
			if _, err = os.Stat(path); err != nil {
				return TTSPaths{}, fmt.Errorf("required TTS file %s: %w", path, err)
			}
		}
	}
	return TTSPaths{Model: model, RuntimePath: runtimePath}, nil
}

// EnsureTTSModel downloads the pinned model from ModelScope.
func EnsureTTSModel(ctx context.Context, cacheDir string, progress ProgressFunc) (TTSModelPaths, error) {
	return EnsureTTSModelFrom(ctx, cacheDir, ModelSourceModelScope, progress)
}

// EnsureTTSModelFrom downloads and verifies the pinned model from source.
func EnsureTTSModelFrom(ctx context.Context, cacheDir string, source ModelSource, progress ProgressFunc) (TTSModelPaths, error) {
	if err := ctx.Err(); err != nil {
		return TTSModelPaths{}, err
	}
	source, err := normalizeModelSource(source)
	if err != nil {
		return TTSModelPaths{}, err
	}
	ttsMu.Lock()
	defer ttsMu.Unlock()
	if cacheDir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return TTSModelPaths{}, fmt.Errorf("find user cache directory: %w", err)
		}
		cacheDir = filepath.Join(base, "tracklogic-voice", "models", "kokoro-82m-v1.1-zh", ttsManifestRevision)
	}
	for _, file := range defaultTTSFiles {
		dst := filepath.Join(cacheDir, filepath.FromSlash(file.name))
		if validFile(dst, file.sha256) {
			continue
		}
		downloadURL, err := ttsFileURL(source, file.name)
		if err != nil {
			return TTSModelPaths{}, err
		}
		if err := downloadFile(ctx, downloadURL, dst, file.sha256, progress); err != nil {
			return TTSModelPaths{}, err
		}
	}
	return ttsModelPaths(cacheDir), nil
}

func ttsFileURL(source ModelSource, name string) (string, error) {
	source, err := normalizeModelSource(source)
	if err != nil {
		return "", err
	}
	switch source {
	case ModelSourceModelScope:
		query := url.Values{}
		query.Set("Revision", ModelScopeTTSRevision)
		query.Set("FilePath", filepath.ToSlash(name))
		return modelScopeDownloadEndpoint + "/api/v1/models/" + ModelScopeTTSRepository + "/repo?" + query.Encode(), nil
	case ModelSourceHuggingFace:
		parts := strings.Split(filepath.ToSlash(name), "/")
		for i := range parts {
			parts[i] = url.PathEscape(parts[i])
		}
		return huggingFaceDownloadEndpoint + "/" + HuggingFaceTTSRepository + "/resolve/" + HuggingFaceTTSRevision + "/" + strings.Join(parts, "/"), nil
	default:
		panic("unreachable model source")
	}
}

func ttsModelPaths(dir string) TTSModelPaths {
	return TTSModelPaths{
		Directory:       dir,
		Model:           filepath.Join(dir, "onnx", "model_quantized.onnx"),
		Voices:          filepath.Join(dir, "voices", "voices-v1.1-zh.bin"),
		Tokenizer:       filepath.Join(dir, "tokenizer.json"),
		TokenizerConfig: filepath.Join(dir, "tokenizer_config.json"),
		Config:          filepath.Join(dir, "config.json"),
	}
}
