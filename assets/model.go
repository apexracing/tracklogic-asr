package assets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	// DefaultModelRepository is the pinned Hugging Face model repository.
	DefaultModelRepository = "DennisHuang648/SenseVoiceSmall-onnx"
	// DefaultModelRevision is the immutable model revision downloaded by default.
	DefaultModelRevision = "0e0d0a81fe03a27c0d56329ff6fc2dcc3fe01f7e"
)

type modelFile struct{ name, sha256 string }

var defaultModelFiles = []modelFile{
	{"model_quant.onnx", "21dc965f689a78d1604717bf561e40d5a236087c85a95584567835750549e822"},
	{"tokens.json", "a2594fc1474e78973149cba8cd1f603ebed8c39c7decb470631f66e70ce58e97"},
	{"am.mvn", "29b3c740a2c0cfc6b308126d31d7f265fa2be74f3bb095cd2f143ea970896ae5"},
	{"config.yaml", "f71e239ba36705564b5bf2d2ffd07eece07b8e3f2bbf6d2c99d8df856339ac19"},
}
var modelMu sync.Mutex

// ModelPaths contains the files required by the SenseVoice frontend and model.
type ModelPaths struct {
	Directory string
	Model     string
	Tokens    string
	CMVN      string
	Config    string
}

func defaultModelCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}
	return filepath.Join(base, "tracklogic-asr", "models", "sensevoice-small-int8", DefaultModelRevision), nil
}

// EnsureModel downloads and verifies the pinned INT8 model when necessary.
func EnsureModel(ctx context.Context, cacheDir string, progress ProgressFunc) (ModelPaths, error) {
	if err := ctx.Err(); err != nil {
		return ModelPaths{}, err
	}
	modelMu.Lock()
	defer modelMu.Unlock()
	if cacheDir == "" {
		var err error
		cacheDir, err = defaultModelCacheDir()
		if err != nil {
			return ModelPaths{}, err
		}
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return ModelPaths{}, fmt.Errorf("create model cache: %w", err)
	}
	base := "https://huggingface.co/" + DefaultModelRepository + "/resolve/" + DefaultModelRevision + "/"
	for _, file := range defaultModelFiles {
		dst := filepath.Join(cacheDir, file.name)
		if validFile(dst, file.sha256) {
			continue
		}
		if err := downloadFile(ctx, base+file.name, dst, file.sha256, progress); err != nil {
			return ModelPaths{}, err
		}
	}
	return modelPaths(cacheDir), nil
}

func modelPaths(dir string) ModelPaths {
	return ModelPaths{
		Directory: dir,
		Model:     filepath.Join(dir, "model_quant.onnx"),
		Tokens:    filepath.Join(dir, "tokens.json"),
		CMVN:      filepath.Join(dir, "am.mvn"),
		Config:    filepath.Join(dir, "config.yaml"),
	}
}
