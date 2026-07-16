package assets

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareRequiresModelCacheDirForDownload(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func() error
	}{
		{
			name: "ASR",
			prepare: func() error {
				_, err := PrepareASR(context.Background(), ASRConfig{})
				return err
			},
		},
		{
			name: "TTS",
			prepare: func() error {
				_, err := PrepareTTS(context.Background(), TTSConfig{})
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.prepare()
			if err == nil || !strings.Contains(err.Error(), "cache directory is required") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestPrepareCustomPaths(t *testing.T) {
	dir := t.TempDir()
	modelDir := filepath.Join(dir, "model")
	if err := os.Mkdir(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model_quant.onnx", "tokens.json", "am.mvn", "config.yaml"} {
		if err := os.WriteFile(filepath.Join(modelDir, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runtimePath := filepath.Join(dir, "onnxruntime.dll")
	if err := os.WriteFile(runtimePath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := Prepare(context.Background(), Config{ModelDir: modelDir, RuntimePath: runtimePath})
	if err != nil {
		t.Fatal(err)
	}
	if paths.Model.Directory != modelDir || paths.RuntimePath != runtimePath {
		t.Fatalf("paths=%+v", paths)
	}
}

func TestPrepareRejectsIncompleteModel(t *testing.T) {
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "onnxruntime.dll")
	_ = os.WriteFile(runtimePath, []byte("test"), 0o644)
	if _, err := Prepare(context.Background(), Config{ModelDir: dir, RuntimePath: runtimePath}); err == nil {
		t.Fatal("expected missing-model error")
	}
}

func TestPrepareCustomModelIgnoresModelSource(t *testing.T) {
	dir := t.TempDir()
	modelDir := filepath.Join(dir, "model")
	if err := os.Mkdir(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model_quant.onnx", "tokens.json", "am.mvn", "config.yaml"} {
		if err := os.WriteFile(filepath.Join(modelDir, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runtimePath := filepath.Join(dir, "onnxruntime.dll")
	if err := os.WriteFile(runtimePath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Prepare(context.Background(), Config{
		ModelDir:    modelDir,
		ModelSource: ModelSource("unsupported-but-unused"),
		RuntimePath: runtimePath,
	})
	if err != nil {
		t.Fatal(err)
	}
}
