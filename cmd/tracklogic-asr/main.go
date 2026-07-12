package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	asr "github.com/apexracing/tracklogic-asr"
	"github.com/apexracing/tracklogic-asr/assets"
)

func main() {
	runtimePath := flag.String("runtime", "", "path to onnxruntime.dll (empty uses the embedded runtime)")
	modelDir := flag.String("model-dir", "", "model directory (empty downloads automatically)")
	language := flag.String("language", "auto", "auto, zh, en, yue, ja, ko, or nospeech")
	withoutITN := flag.Bool("without-itn", false, "disable punctuation and inverse text normalization")
	threads := flag.Int("threads", 0, "ONNX CPU thread count (0 uses runtime default)")
	recordDuration := flag.Duration("record", 0, "record from the default microphone, for example 5s")
	recordOutput := flag.String("record-out", filepath.FromSlash("testdata/local-recording.wav"), "recorded WAV output path")
	flag.Parse()
	if *recordDuration <= 0 && flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: tracklogic-asr [flags] audio.wav")
		fmt.Fprintln(os.Stderr, "   or: tracklogic-asr -record 5s [flags]")
		flag.PrintDefaults()
		os.Exit(2)
	}

	ctx := context.Background()
	audioPath := ""
	if *recordDuration > 0 {
		if flag.NArg() != 0 {
			fmt.Fprintln(os.Stderr, "do not provide audio.wav together with -record")
			os.Exit(2)
		}
		if err := os.MkdirAll(filepath.Dir(*recordOutput), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "create recording directory:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Recording for %s...\n", recordDuration.String())
		if err := recordWAV(ctx, *recordDuration, *recordOutput); err != nil {
			fmt.Fprintln(os.Stderr, "record:", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Saved:", *recordOutput)
		audioPath = *recordOutput
	} else {
		audioPath = flag.Arg(0)
	}

	started := time.Now()
	lastPercent := map[string]int{}
	r, err := asr.New(ctx, asr.Config{
		Assets: assets.Config{
			RuntimePath: *runtimePath,
			ModelDir:    *modelDir,
			Progress: func(name string, downloaded, total int64) {
				if total > 0 {
					percent := int(downloaded * 100 / total)
					if lastPercent[name] != percent || downloaded == total {
						lastPercent[name] = percent
						fmt.Fprintf(os.Stderr, "\r%-35s %3d%%", name, percent)
					}
				}
			},
		},
		NumThreads: *threads,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "initialize:", err)
		os.Exit(1)
	}
	defer r.Close()
	result, err := r.TranscribeFile(ctx, audioPath, asr.Options{
		Language:   asr.Language(*language),
		WithoutITN: *withoutITN,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "transcribe:", err)
		os.Exit(1)
	}
	fmt.Println(result.Text)
	fmt.Fprintf(os.Stderr, "language=%s emotion=%s events=%v elapsed=%s\n", result.Language, result.Emotion, result.Events, time.Since(started).Round(time.Millisecond))
}
