package sensevoice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

const (
	DefaultModelRepository = "DennisHuang648/SenseVoiceSmall-onnx"
	DefaultModelRevision   = "0e0d0a81fe03a27c0d56329ff6fc2dcc3fe01f7e"
)

type modelFile struct {
	name   string
	sha256 string
}

var defaultModelFiles = []modelFile{
	{"model_quant.onnx", "21dc965f689a78d1604717bf561e40d5a236087c85a95584567835750549e822"},
	{"tokens.json", "a2594fc1474e78973149cba8cd1f603ebed8c39c7decb470631f66e70ce58e97"},
	{"am.mvn", "29b3c740a2c0cfc6b308126d31d7f265fa2be74f3bb095cd2f143ea970896ae5"},
	{"config.yaml", "f71e239ba36705564b5bf2d2ffd07eece07b8e3f2bbf6d2c99d8df856339ac19"},
}

var downloadMu sync.Mutex

// ModelPaths contains the local files needed by the recognizer.
type ModelPaths struct {
	Directory string
	Model     string
	Tokens    string
	CMVN      string
	Config    string
}

// DefaultCacheDir returns the per-user model cache directory.
func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}
	return filepath.Join(base, "tracklogic-asr", "models", "sensevoice-small-int8", DefaultModelRevision), nil
}

// EnsureModel downloads and verifies the pinned INT8 model from Hugging Face.
// Existing valid files are reused. Downloads are written atomically.
func EnsureModel(ctx context.Context, cacheDir string, progress func(name string, downloaded, total int64)) (ModelPaths, error) {
	downloadMu.Lock()
	defer downloadMu.Unlock()

	if cacheDir == "" {
		var err error
		cacheDir, err = DefaultCacheDir()
		if err != nil {
			return ModelPaths{}, err
		}
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return ModelPaths{}, fmt.Errorf("create model cache: %w", err)
	}

	client := &http.Client{}
	base := "https://huggingface.co/" + DefaultModelRepository + "/resolve/" + DefaultModelRevision + "/"
	for _, file := range defaultModelFiles {
		dst := filepath.Join(cacheDir, file.name)
		if validFile(dst, file.sha256) {
			continue
		}
		if err := downloadFile(ctx, client, base+file.name, dst, file.sha256, progress); err != nil {
			return ModelPaths{}, err
		}
	}
	return modelPaths(cacheDir), nil
}

func modelPaths(dir string) ModelPaths {
	return ModelPaths{
		Directory: dir,
		Model:     filepath.Join(dir, "model_quant.onnx"),
		Tokens:    filepath.Join(dir, "tokens.json"),
		CMVN:      filepath.Join(dir, "am.mvn"),
		Config:    filepath.Join(dir, "config.yaml"),
	}
}

func validFile(path, want string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return false
	}
	return hex.EncodeToString(h.Sum(nil)) == want
}

func downloadFile(ctx context.Context, client *http.Client, url, dst, wantHash string, progress func(string, int64, int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create model request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", filepath.Base(dst), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: server returned %s", filepath.Base(dst), resp.Status)
	}

	tmp := dst + ".download"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create temporary model file: %w", err)
	}
	h := sha256.New()
	w := io.MultiWriter(f, h)
	var written int64
	buf := make([]byte, 1<<20)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err = w.Write(buf[:n]); err != nil {
				f.Close()
				os.Remove(tmp)
				return fmt.Errorf("write %s: %w", filepath.Base(dst), err)
			}
			written += int64(n)
			if progress != nil {
				progress(filepath.Base(dst), written, resp.ContentLength)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("read %s: %w", filepath.Base(dst), readErr)
		}
	}
	if err = f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close %s: %w", filepath.Base(dst), err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != wantHash {
		os.Remove(tmp)
		return fmt.Errorf("verify %s: sha256 %s, want %s", filepath.Base(dst), got, wantHash)
	}
	if err = os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("install %s: %w", filepath.Base(dst), err)
	}
	return nil
}
