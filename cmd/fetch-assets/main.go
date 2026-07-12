package main

import (
	"context"
	"fmt"
	"os"

	sensevoice "github.com/apexracing/tracklogic-asr"
)

func main() {
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
	runtimePath, err := sensevoice.EnsureRuntime(ctx, "", progress)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nruntime:", err)
		os.Exit(1)
	}
	model, err := sensevoice.EnsureModel(ctx, "", progress)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nmodel:", err)
		os.Exit(1)
	}
	fmt.Printf("\nONNX Runtime: %s\nModel: %s\n", runtimePath, model.Directory)
}
