package voice

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/apexracing/tracklogic-voice/assets"
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

func TestIntegrationKokoro(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("bundled runtime is windows/amd64")
	}
	modelDir := os.Getenv("KOKORO_MODEL_DIR")
	if modelDir == "" {
		modelDir = filepath.FromSlash("Kokoro-82M-v1.1-zh-ONNX")
	}
	if _, err := os.Stat(filepath.Join(modelDir, "onnx", "model_quantized.onnx")); err != nil {
		t.Skipf("local Kokoro model unavailable: %s", modelDir)
	}
	s, err := NewSynthesizer(context.Background(), SynthesizerConfig{
		Assets:     assets.TTSConfig{ModelDir: modelDir, RuntimePath: os.Getenv("ONNXRUNTIME_DLL")},
		NumThreads: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(s.Voices()); got != 103 {
		t.Fatalf("Voices() returned %d entries, want 103", got)
	}
	modelVoices := make(map[string]struct{}, len(s.Voices()))
	for _, id := range s.Voices() {
		modelVoices[id] = struct{}{}
	}
	for _, option := range SelectableVoices() {
		if _, ok := modelVoices[option.ID]; !ok {
			t.Errorf("selectable voice %q (%s) is absent from the model", option.ID, option.Name)
		}
	}
	for name, text := range map[string]string{
		"Chinese": "千里之行，始于足下。",
		"English": "Tracklogic voice speaks clearly.",
		"Mixed":   "你好，Tracklogic voice!",
	} {
		t.Run(name, func(t *testing.T) {
			samples, synthErr := s.Synthesize(context.Background(), text, SynthesisOptions{TrimSilence: true})
			if synthErr != nil {
				t.Fatal(synthErr)
			}
			if len(samples) < SynthesisSampleRate/10 {
				t.Fatalf("unexpectedly short waveform: %d samples", len(samples))
			}
		})
	}
	path := filepath.Join(t.TempDir(), "voice.wav")
	if err = s.SynthesizeFile(context.Background(), "测试 WAV。", path, SynthesisOptions{TrimSilence: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, synthErr := s.Synthesize(context.Background(), "Hi.", SynthesisOptions{TrimSilence: true})
			errs <- synthErr
		}()
	}
	wg.Wait()
	close(errs)
	for synthErr := range errs {
		if synthErr != nil {
			t.Fatal(synthErr)
		}
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Synthesize(context.Background(), "closed", SynthesisOptions{}); err == nil {
		t.Fatal("expected closed-synthesizer error")
	}
}
