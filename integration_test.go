package sensevoice

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIntegrationSenseVoiceINT8(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("bundled runtime is windows/amd64")
	}
	wav := os.Getenv("SENSEVOICE_TEST_WAV")
	modelDir := os.Getenv("SENSEVOICE_MODEL_DIR")
	runtimePath := os.Getenv("ONNXRUNTIME_DLL")
	if wav == "" {
		wav = filepath.FromSlash("testdata/zh.wav")
	}
	if modelDir == "" {
		modelDir = filepath.FromSlash("models/sensevoice-small-int8")
	}
	if runtimePath == "" {
		runtimePath = filepath.FromSlash("runtime/windows-amd64/onnxruntime.dll")
	}
	for _, required := range []string{wav, filepath.Join(modelDir, "model_quant.onnx"), runtimePath} {
		if _, err := os.Stat(required); err != nil {
			t.Skipf("local integration asset unavailable: %s", required)
		}
	}
	r, err := New(context.Background(), Config{ModelDir: modelDir, RuntimePath: runtimePath, NumThreads: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	result, err := r.TranscribeFile(context.Background(), wav, Options{Language: LanguageChinese})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Text, "时间早上9点至下午5点") {
		t.Fatalf("unexpected transcript: %#v", result)
	}
}
