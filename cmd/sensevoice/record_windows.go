//go:build windows

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var (
	winmm              = syscall.NewLazyDLL("winmm.dll")
	mciSendStringW     = winmm.NewProc("mciSendStringW")
	mciGetErrorStringW = winmm.NewProc("mciGetErrorStringW")
)

func recordWAV(ctx context.Context, duration time.Duration, output string) error {
	abs, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	_ = mci("close tracklogic_capture")
	if err = mci("open new type waveaudio alias tracklogic_capture"); err != nil {
		return fmt.Errorf("open microphone: %w", err)
	}
	defer mci("close tracklogic_capture")
	if err = mci("set tracklogic_capture bitspersample 16 channels 1 samplespersec 16000 bytespersec 32000 alignment 2"); err != nil {
		return fmt.Errorf("set recording format: %w", err)
	}
	if err = mci("record tracklogic_capture"); err != nil {
		return fmt.Errorf("start microphone: %w", err)
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		_ = mci("stop tracklogic_capture")
		return ctx.Err()
	case <-timer.C:
	}
	if err = mci("stop tracklogic_capture"); err != nil {
		return fmt.Errorf("stop microphone: %w", err)
	}
	if err = mci(fmt.Sprintf(`save tracklogic_capture "%s"`, abs)); err != nil {
		return fmt.Errorf("save recording: %w", err)
	}
	return nil
}

func mci(command string) error {
	p, err := syscall.UTF16PtrFromString(command)
	if err != nil {
		return err
	}
	code, _, _ := mciSendStringW.Call(uintptr(unsafe.Pointer(p)), 0, 0, 0)
	if code == 0 {
		return nil
	}
	buf := make([]uint16, 256)
	mciGetErrorStringW.Call(code, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return fmt.Errorf("MCI %d: %s", code, syscall.UTF16ToString(buf))
}
