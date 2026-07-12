package sensevoice

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
	ONNXRuntimeVersion = "1.26.0"
	runtimeDLLHash     = "b2ba7ca16e0e4fe71ad5148744ab885a2f5809e52a0c3de4d9ba3853a03977f9"
)

var runtimeInstallMu sync.Mutex

// DefaultRuntimeCacheDir returns the per-user embedded runtime cache directory.
func DefaultRuntimeCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}
	return filepath.Join(base, "tracklogic-asr", "runtime", "onnxruntime", ONNXRuntimeVersion, "windows-amd64"), nil
}

// EnsureRuntime verifies the embedded Microsoft ONNX Runtime CPU DLL and
// atomically installs it into the user cache. It performs no network access.
func EnsureRuntime(ctx context.Context, cacheDir string, progress func(name string, downloaded, total int64)) (string, error) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return "", fmt.Errorf("embedded ONNX Runtime supports windows/amd64 only")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	runtimeInstallMu.Lock()
	defer runtimeInstallMu.Unlock()

	if cacheDir == "" {
		var err error
		cacheDir, err = DefaultRuntimeCacheDir()
		if err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create runtime cache: %w", err)
	}
	dllPath := filepath.Join(cacheDir, "onnxruntime.dll")
	if validFile(dllPath, runtimeDLLHash) {
		return dllPath, nil
	}
	if len(embeddedRuntimeDLL) == 0 {
		return "", fmt.Errorf("embedded onnxruntime.dll is unavailable for this platform")
	}
	hash := sha256.Sum256(embeddedRuntimeDLL)
	if got := hex.EncodeToString(hash[:]); got != runtimeDLLHash {
		return "", fmt.Errorf("verify embedded onnxruntime.dll: sha256 %s, want %s", got, runtimeDLLHash)
	}
	if err := writeEmbeddedFile(ctx, dllPath, embeddedRuntimeDLL, progress); err != nil {
		return "", err
	}
	licensePath := filepath.Join(cacheDir, "LICENSE-onnxruntime.txt")
	if len(embeddedRuntimeLicense) > 0 {
		if err := writeEmbeddedFile(ctx, licensePath, embeddedRuntimeLicense, nil); err != nil {
			return "", err
		}
	}
	return dllPath, nil
}

func writeEmbeddedFile(ctx context.Context, destination string, data []byte, progress func(string, int64, int64)) error {
	tmp := destination + ".extract"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	const chunkSize = 1 << 20
	for offset := 0; offset < len(data); {
		if err = ctx.Err(); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}
		if _, err = f.Write(data[offset:end]); err != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("write %s: %w", destination, err)
		}
		offset = end
		if progress != nil {
			progress(filepath.Base(destination), int64(offset), int64(len(data)))
		}
	}
	if err = f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close %s: %w", destination, err)
	}
	if err = os.Rename(tmp, destination); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("install %s: %w", destination, err)
	}
	return nil
}
