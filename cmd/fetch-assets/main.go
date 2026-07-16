package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/apexracing/tracklogic-voice/assets"
)

func main() {
	modelSource := flag.String("model-source", string(assets.ModelSourceModelScope), "model download source: modelscope or huggingface")
	kind := flag.String("kind", "all", "assets to prepare: asr, tts, or all")
	flag.Parse()
	lastPercent := map[string]int{}
	progress := func(name string, downloaded, total int64) {
		if total > 0 {
			percent := int(downloaded * 100 / total)
			if lastPercent[name] != percent || downloaded == total {
				lastPercent[name] = percent
				fmt.Fprintf(os.Stderr, "\r%-35s %3d%%", name, percent)
			}
		}
	}
	ctx := context.Background()
	if *kind == "asr" || *kind == "all" {
		paths, err := assets.PrepareASR(ctx, assets.ASRConfig{ModelSource: assets.ModelSource(*modelSource), Progress: progress})
		if err != nil {
			fmt.Fprintln(os.Stderr, "\nprepare ASR assets:", err)
			os.Exit(1)
		}
		fmt.Printf("\nASR model: %s\n", paths.Model.Directory)
	}
	if *kind == "tts" || *kind == "all" {
		paths, err := assets.PrepareTTS(ctx, assets.TTSConfig{ModelSource: assets.ModelSource(*modelSource), Progress: progress})
		if err != nil {
			fmt.Fprintln(os.Stderr, "\nprepare TTS assets:", err)
			os.Exit(1)
		}
		fmt.Printf("\nTTS model: %s\n", paths.Model.Directory)
	}
	if *kind != "asr" && *kind != "tts" && *kind != "all" {
		fmt.Fprintln(os.Stderr, "-kind must be asr, tts, or all")
		os.Exit(2)
	}
}
