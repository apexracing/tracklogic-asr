package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadFileFollowsRedirectAndReportsProgress(t *testing.T) {
	payload := []byte("model payload")
	hash := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/file", http.StatusFound)
			return
		}
		if r.URL.Path != "/file" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dst := filepath.Join(t.TempDir(), "model.bin")
	var completed, total int64
	err := downloadFile(context.Background(), server.URL+"/redirect", dst, hex.EncodeToString(hash[:]), func(_ string, done, size int64) {
		completed, total = done, size
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) || completed != int64(len(payload)) || total != int64(len(payload)) {
		t.Fatalf("payload=%q progress=%d/%d", got, completed, total)
	}
}

func TestDownloadFileCleansUpFailures(t *testing.T) {
	payload := []byte("wrong payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	goodHash := sha256.Sum256([]byte("expected payload"))
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "http-error", path: "/error"},
		{name: "hash-mismatch", path: "/wrong"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "model.bin")
			err := downloadFile(context.Background(), server.URL+test.path, dst, hex.EncodeToString(goodHash[:]), nil)
			if err == nil {
				t.Fatal("expected error")
			}
			for _, path := range []string{dst, dst + ".download"} {
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Fatalf("temporary artifact remains: %s", path)
				}
			}
		})
	}
}

func TestDownloadFileHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dst := filepath.Join(t.TempDir(), "model.bin")
	err := downloadFile(ctx, "https://example.invalid/model.bin", dst, strings.Repeat("0", 64), nil)
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("err=%v", err)
	}
	if _, statErr := os.Stat(dst + ".download"); !os.IsNotExist(statErr) {
		t.Fatalf("temporary artifact remains: %s.download", dst)
	}
}
