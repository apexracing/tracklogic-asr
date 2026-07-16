package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	// ONNXRuntimeVersion is the embedded Microsoft CPU runtime version.
	ONNXRuntimeVersion = "1.26.0"
	runtimeDLLHash     = "b2ba7ca16e0e4fe71ad5148744ab885a2f5809e52a0c3de4d9ba3853a03977f9"
)

var runtimeMu sync.Mutex

func defaultRuntimeCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}
	return filepath.Join(base, "tracklogic-voice", "runtime", "onnxruntime", ONNXRuntimeVersion, "windows-amd64"), nil
}

// EnsureRuntime verifies and installs the embedded Windows x64 runtime.
func EnsureRuntime(ctx context.Context, cacheDir string, progress ProgressFunc) (string, error) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return "", fmt.Errorf("embedded ONNX Runtime supports windows/amd64 only")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if cacheDir == "" {
		var err error
		cacheDir, err = defaultRuntimeCacheDir()
		if err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	dll := filepath.Join(cacheDir, "onnxruntime.dll")
	if validFile(dll, runtimeDLLHash) {
		return dll, nil
	}
	if len(embeddedRuntimeDLL) == 0 {
		return "", fmt.Errorf("embedded onnxruntime.dll is unavailable")
	}
	h := sha256.Sum256(embeddedRuntimeDLL)
	if hex.EncodeToString(h[:]) != runtimeDLLHash {
		return "", fmt.Errorf("embedded onnxruntime.dll SHA-256 mismatch")
	}
	if err := writeEmbedded(ctx, dll, embeddedRuntimeDLL, progress); err != nil {
		return "", err
	}
	if len(embeddedRuntimeLicense) > 0 {
		if err := writeEmbedded(ctx, filepath.Join(cacheDir, "LICENSE-onnxruntime.txt"), embeddedRuntimeLicense, nil); err != nil {
			return "", err
		}
	}
	return dll, nil
}

func writeEmbedded(ctx context.Context, dst string, data []byte, progress ProgressFunc) error {
	tmp := dst + ".extract"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	for off := 0; off < len(data); {
		if err = ctx.Err(); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
		end := off + (1 << 20)
		if end > len(data) {
			end = len(data)
		}
		if _, err = f.Write(data[off:end]); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
		off = end
		if progress != nil {
			progress(filepath.Base(dst), int64(off), int64(len(data)))
		}
	}
	if err = f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err = os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
