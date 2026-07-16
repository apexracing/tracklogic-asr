// Package assets prepares the models and native runtime required by tracklogic-voice.
package assets

import (
	"context"
	"fmt"
	"os"
)

// ProgressFunc receives download or extraction progress in bytes.
type ProgressFunc func(name string, completed, total int64)

// ASRConfig controls custom ASR paths, cache locations, and progress reporting.
// ModelCacheDir is required when ModelDir is empty.
type ASRConfig struct {
	ModelDir        string
	ModelCacheDir   string
	ModelSource     ModelSource
	RuntimePath     string
	RuntimeCacheDir string
	Progress        ProgressFunc
}

// Config is retained for source compatibility.
// Deprecated: use ASRConfig.
type Config = ASRConfig

// Paths contains verified model and runtime paths.
type Paths struct {
	Model       ModelPaths
	RuntimePath string
}

// PrepareASR resolves custom paths or prepares the embedded runtime and the
// downloadable model in the caller-provided cache directory.
func PrepareASR(ctx context.Context, cfg ASRConfig) (Paths, error) {
	if cfg.ModelDir == "" && cfg.ModelCacheDir == "" {
		return Paths{}, fmt.Errorf("ASR model cache directory is required when model directory is not set")
	}
	runtimePath := cfg.RuntimePath
	var err error
	if runtimePath == "" {
		runtimePath, err = EnsureRuntime(ctx, cfg.RuntimeCacheDir, cfg.Progress)
		if err != nil {
			return Paths{}, err
		}
	} else if _, err = os.Stat(runtimePath); err != nil {
		return Paths{}, fmt.Errorf("required runtime file %s: %w", runtimePath, err)
	}

	var model ModelPaths
	if cfg.ModelDir == "" {
		model, err = EnsureModelFrom(ctx, cfg.ModelCacheDir, cfg.ModelSource, cfg.Progress)
		if err != nil {
			return Paths{}, err
		}
	} else {
		model = modelPaths(cfg.ModelDir)
		for _, path := range []string{model.Model, model.Tokens, model.CMVN, model.Config} {
			if _, err = os.Stat(path); err != nil {
				return Paths{}, fmt.Errorf("required model file %s: %w", path, err)
			}
		}
	}
	return Paths{Model: model, RuntimePath: runtimePath}, nil
}

// Prepare is retained for source compatibility.
// Deprecated: use PrepareASR.
func Prepare(ctx context.Context, cfg Config) (Paths, error) { return PrepareASR(ctx, cfg) }
