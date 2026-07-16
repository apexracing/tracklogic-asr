// Package assets prepares the model and native runtime required by tracklogic-asr.
package assets

import (
	"context"
	"fmt"
	"os"
)

// ProgressFunc receives download or extraction progress in bytes.
type ProgressFunc func(name string, completed, total int64)

// Config controls custom paths, cache locations, and progress reporting.
type Config struct {
	ModelDir        string
	ModelCacheDir   string
	ModelSource     ModelSource
	RuntimePath     string
	RuntimeCacheDir string
	Progress        ProgressFunc
}

// Paths contains verified model and runtime paths.
type Paths struct {
	Model       ModelPaths
	RuntimePath string
}

// Prepare resolves custom paths or prepares the default embedded runtime and
// downloadable model.
func Prepare(ctx context.Context, cfg Config) (Paths, error) {
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
