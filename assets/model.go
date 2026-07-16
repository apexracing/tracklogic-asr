package assets

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

// ModelSource identifies a remote model repository provider.
type ModelSource string

const (
	// ModelSourceModelScope downloads from ModelScope. It is the default source.
	ModelSourceModelScope ModelSource = "modelscope"
	// ModelSourceHuggingFace downloads from Hugging Face.
	ModelSourceHuggingFace ModelSource = "huggingface"

	ModelScopeModelRepository  = "iic/SenseVoiceSmall-onnx"
	ModelScopeModelRevision    = "4e95991ddb34c70e9b94f026d62ac82a9d941ae1"
	HuggingFaceModelRepository = "DennisHuang648/SenseVoiceSmall-onnx"
	HuggingFaceModelRevision   = "0e0d0a81fe03a27c0d56329ff6fc2dcc3fe01f7e"
	defaultModelCacheRevision  = HuggingFaceModelRevision

	// DefaultModelRepository is retained for compatibility.
	// Deprecated: use HuggingFaceModelRepository or ModelScopeModelRepository.
	DefaultModelRepository = HuggingFaceModelRepository
	// DefaultModelRevision is retained for compatibility.
	// Deprecated: use HuggingFaceModelRevision or ModelScopeModelRevision.
	DefaultModelRevision = HuggingFaceModelRevision
)

var (
	modelScopeDownloadEndpoint  = "https://www.modelscope.cn"
	huggingFaceDownloadEndpoint = "https://huggingface.co"
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
	// Both providers serve byte-identical artifacts, so they intentionally share
	// the existing cache directory to avoid downloading a second model copy.
	return filepath.Join(base, "tracklogic-asr", "models", "sensevoice-small-int8", defaultModelCacheRevision), nil
}

// EnsureModel downloads the pinned INT8 model from ModelScope and verifies it.
func EnsureModel(ctx context.Context, cacheDir string, progress ProgressFunc) (ModelPaths, error) {
	return EnsureModelFrom(ctx, cacheDir, ModelSourceModelScope, progress)
}

// EnsureModelFrom downloads the pinned INT8 model from source and verifies it.
// An empty source selects ModelScope.
func EnsureModelFrom(ctx context.Context, cacheDir string, source ModelSource, progress ProgressFunc) (ModelPaths, error) {
	if err := ctx.Err(); err != nil {
		return ModelPaths{}, err
	}
	source, err := normalizeModelSource(source)
	if err != nil {
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
	for _, file := range defaultModelFiles {
		dst := filepath.Join(cacheDir, file.name)
		if validFile(dst, file.sha256) {
			continue
		}
		downloadURL, err := modelFileURL(source, file.name)
		if err != nil {
			return ModelPaths{}, err
		}
		if err := downloadFile(ctx, downloadURL, dst, file.sha256, progress); err != nil {
			return ModelPaths{}, err
		}
	}
	return modelPaths(cacheDir), nil
}

func normalizeModelSource(source ModelSource) (ModelSource, error) {
	if source == "" {
		return ModelSourceModelScope, nil
	}
	switch source {
	case ModelSourceModelScope, ModelSourceHuggingFace:
		return source, nil
	default:
		return "", fmt.Errorf("unsupported model source %q", source)
	}
}

func modelFileURL(source ModelSource, name string) (string, error) {
	source, err := normalizeModelSource(source)
	if err != nil {
		return "", err
	}
	switch source {
	case ModelSourceModelScope:
		query := url.Values{}
		query.Set("Revision", ModelScopeModelRevision)
		query.Set("FilePath", name)
		return modelScopeDownloadEndpoint + "/api/v1/models/" + ModelScopeModelRepository + "/repo?" + query.Encode(), nil
	case ModelSourceHuggingFace:
		return huggingFaceDownloadEndpoint + "/" + HuggingFaceModelRepository + "/resolve/" + HuggingFaceModelRevision + "/" + url.PathEscape(name), nil
	default:
		panic("unreachable model source")
	}
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
