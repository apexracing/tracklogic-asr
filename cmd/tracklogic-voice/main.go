package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	voice "github.com/apexracing/tracklogic-voice"
	"github.com/apexracing/tracklogic-voice/assets"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "asr":
		err = runASR(os.Args[2:])
	case "tts":
		err = runTTS(os.Args[2:])
	case "fetch":
		err = runFetch(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: tracklogic-voice <asr|tts|fetch> [flags]")
	os.Exit(2)
}

func runASR(args []string) error {
	fs := flag.NewFlagSet("asr", flag.ContinueOnError)
	modelDir := fs.String("model-dir", "", "SenseVoice model directory")
	runtimePath := fs.String("runtime", "", "path to onnxruntime.dll")
	source := fs.String("model-source", "modelscope", "modelscope or huggingface")
	language := fs.String("language", "auto", "auto, zh, en, yue, ja, ko, or nospeech")
	threads := fs.Int("threads", 2, "ONNX intra-op CPU threads")
	withoutITN := fs.Bool("without-itn", false, "disable inverse text normalization")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: tracklogic-voice asr [flags] audio.wav")
	}
	ctx := context.Background()
	started := time.Now()
	r, err := voice.NewRecognizer(ctx, voice.RecognizerConfig{Assets: assets.ASRConfig{
		ModelDir: *modelDir, RuntimePath: *runtimePath, ModelSource: assets.ModelSource(*source), Progress: progress,
	}, NumThreads: *threads})
	if err != nil {
		return err
	}
	defer r.Close()
	result, err := r.TranscribeFile(ctx, fs.Arg(0), voice.TranscriptionOptions{
		Language: voice.Language(*language), WithoutITN: *withoutITN,
	})
	if err != nil {
		return err
	}
	fmt.Println(result.Text)
	fmt.Fprintf(os.Stderr, "language=%s emotion=%s elapsed=%s\n", result.Language, result.Emotion, time.Since(started).Round(time.Millisecond))
	return nil
}

func runTTS(args []string) error {
	fs := flag.NewFlagSet("tts", flag.ContinueOnError)
	modelDir := fs.String("model-dir", "", "Kokoro model directory")
	runtimePath := fs.String("runtime", "", "path to onnxruntime.dll")
	source := fs.String("model-source", "modelscope", "modelscope or huggingface")
	language := fs.String("language", "auto", "auto, zh, or en")
	voiceName := fs.String("voice", "", "voice name (empty selects by language)")
	speed := fs.Float64("speed", 1, "speech speed multiplier")
	output := fs.String("out", "voice.wav", "output WAV path")
	threads := fs.Int("threads", 2, "ONNX intra-op CPU threads")
	trim := fs.Bool("trim-silence", true, "trim leading and trailing silence")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: tracklogic-voice tts [flags] text")
	}
	text := strings.Join(fs.Args(), " ")
	ctx := context.Background()
	started := time.Now()
	s, err := voice.NewSynthesizer(ctx, voice.SynthesizerConfig{Assets: assets.TTSConfig{
		ModelDir: *modelDir, RuntimePath: *runtimePath, ModelSource: assets.ModelSource(*source), Progress: progress,
	}, NumThreads: *threads})
	if err != nil {
		return err
	}
	defer s.Close()
	if err = s.SynthesizeFile(ctx, text, *output, voice.SynthesisOptions{
		Voice: *voiceName, Speed: float32(*speed), Language: voice.Language(*language), TrimSilence: *trim,
	}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "saved=%s elapsed=%s\n", *output, time.Since(started).Round(time.Millisecond))
	return nil
}

func runFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	source := fs.String("model-source", "modelscope", "modelscope or huggingface")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: tracklogic-voice fetch [-model-source source] <asr|tts>")
	}
	ctx := context.Background()
	switch fs.Arg(0) {
	case "asr":
		paths, err := assets.PrepareASR(ctx, assets.ASRConfig{ModelSource: assets.ModelSource(*source), Progress: progress})
		if err == nil {
			fmt.Println(paths.Model.Directory)
		}
		return err
	case "tts":
		paths, err := assets.PrepareTTS(ctx, assets.TTSConfig{ModelSource: assets.ModelSource(*source), Progress: progress})
		if err == nil {
			fmt.Println(paths.Model.Directory)
		}
		return err
	default:
		return fmt.Errorf("unknown asset kind %q", fs.Arg(0))
	}
}

func progress(name string, completed, total int64) {
	if total > 0 {
		fmt.Fprintf(os.Stderr, "\r%-35s %3d%%", name, completed*100/total)
		if completed == total {
			fmt.Fprintln(os.Stderr)
		}
	}
}
