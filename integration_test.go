package asr

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/apexracing/tracklogic-asr/assets"
)

func TestIntegrationSenseVoiceINT8(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("bundled runtime is windows/amd64")
	}
	wav := os.Getenv("SENSEVOICE_TEST_WAV")
	if wav == "" {
		wav = filepath.FromSlash("testdata/zh.wav")
	}
	modelDir := os.Getenv("SENSEVOICE_MODEL_DIR")
	if modelDir == "" {
		modelDir = filepath.FromSlash("models/sensevoice-small-int8")
	}
	for _, required := range []string{wav, filepath.Join(modelDir, "model_quant.onnx")} {
		if _, err := os.Stat(required); err != nil {
			t.Skipf("local integration asset unavailable: %s", required)
		}
	}
	runtimePath := os.Getenv("ONNXRUNTIME_DLL")
	r, err := New(context.Background(), Config{Assets: assets.Config{ModelDir: modelDir, RuntimePath: runtimePath}, NumThreads: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Transcribe(context.Background(), make([]float32, 16000), 16000, Options{Language: "invalid"}); err == nil {
		t.Fatal("expected invalid-language error")
	}
	result, err := r.TranscribeFile(context.Background(), wav, Options{Language: LanguageChinese})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Text, "时间早上9点至下午5点") {
		t.Fatalf("unexpected transcript: %#v", result)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := r.TranscribeFile(context.Background(), wav, Options{Language: LanguageChinese})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = r.TranscribeFile(context.Background(), wav, Options{}); err == nil {
		t.Fatal("expected closed-recognizer error")
	}
}
