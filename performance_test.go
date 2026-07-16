package voice

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/apexracing/tracklogic-voice/assets"
	"github.com/apexracing/tracklogic-voice/internal/audio"
)

func BenchmarkASR2Threads(b *testing.B) {
	requirePerformanceBenchmark(b)
	old := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(old)
	modelDir := benchmarkPath("SENSEVOICE_MODEL_DIR", filepath.FromSlash("models/sensevoice-small-int8"))
	wav := benchmarkPath("SENSEVOICE_TEST_WAV", filepath.FromSlash("testdata/zh.wav"))
	if _, err := os.Stat(filepath.Join(modelDir, "model_quant.onnx")); err != nil {
		b.Skipf("missing ASR model: %s", modelDir)
	}
	samples, rate, err := audio.ReadWAV(wav)
	if err != nil {
		b.Fatal(err)
	}
	r, err := NewRecognizer(context.Background(), RecognizerConfig{Assets: assets.ASRConfig{ModelDir: modelDir}, NumThreads: 2})
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()
	for range 3 {
		if _, err = r.Transcribe(context.Background(), samples, rate, TranscriptionOptions{Language: LanguageChinese}); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = r.Transcribe(context.Background(), samples, rate, TranscriptionOptions{Language: LanguageChinese}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTTSChinese2Threads(b *testing.B) {
	benchmarkTTS(b, "千里之行，始于足下。")
}

func BenchmarkTTSEnglish2Threads(b *testing.B) {
	benchmarkTTS(b, "Tracklogic Voice provides clear, efficient speech synthesis for practical applications.")
}

func BenchmarkTTSMixed2Threads(b *testing.B) {
	benchmarkTTS(b, "Tracklogic Voice 支持中文、English 和中英混合语音合成。")
}

func benchmarkTTS(b *testing.B, text string) {
	b.Helper()
	requirePerformanceBenchmark(b)
	old := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(old)
	modelDir := benchmarkPath("KOKORO_MODEL_DIR", filepath.FromSlash("Kokoro-82M-v1.1-zh-ONNX"))
	if _, err := os.Stat(filepath.Join(modelDir, "onnx", "model_quantized.onnx")); err != nil {
		b.Skipf("missing TTS model: %s", modelDir)
	}
	s, err := NewSynthesizer(context.Background(), SynthesizerConfig{Assets: assets.TTSConfig{ModelDir: modelDir}, NumThreads: 2})
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	opts := SynthesisOptions{TrimSilence: true}
	for range 3 {
		if _, err = s.Synthesize(context.Background(), text, opts); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = s.Synthesize(context.Background(), text, opts); err != nil {
			b.Fatal(err)
		}
	}
}

func requirePerformanceBenchmark(b *testing.B) {
	b.Helper()
	if os.Getenv("TRACKLOGIC_BENCH") != "1" {
		b.Skip("set TRACKLOGIC_BENCH=1 to run model benchmarks")
	}
}

func benchmarkPath(environment, fallback string) string {
	if value := os.Getenv(environment); value != "" {
		return value
	}
	return fallback
}
