package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

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

func downloadFile(ctx context.Context, url, dst, wantHash string, progress ProgressFunc) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create asset directory: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
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
		return fmt.Errorf("create temporary asset: %w", err)
	}
	h := sha256.New()
	var written int64
	buf := make([]byte, 1<<20)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err = io.MultiWriter(f, h).Write(buf[:n]); err != nil {
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
		return err
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
