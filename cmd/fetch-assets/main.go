package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/apexracing/tracklogic-asr/assets"
)

func main() {
	modelSource := flag.String("model-source", string(assets.ModelSourceModelScope), "model download source: modelscope or huggingface")
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
	paths, err := assets.Prepare(ctx, assets.Config{
		ModelSource: assets.ModelSource(*modelSource),
		Progress:    progress,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nprepare assets:", err)
		os.Exit(1)
	}
	fmt.Printf("\nONNX Runtime: %s\nModel: %s\n", paths.RuntimePath, paths.Model.Directory)
}
