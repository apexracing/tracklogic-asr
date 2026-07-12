//go:build !windows

package main

import (
	"context"
	"fmt"
	"time"
)

func recordWAV(context.Context, time.Duration, string) error {
	return fmt.Errorf("microphone recording is supported on Windows only")
}
