package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestTTSFileURL(t *testing.T) {
	for _, test := range []struct {
		name   string
		source ModelSource
	}{
		{name: "default"},
		{name: "modelscope", source: ModelSourceModelScope},
		{name: "huggingface", source: ModelSourceHuggingFace},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := ttsFileURL(test.source, "onnx/model_quantized.onnx")
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if test.source == ModelSourceHuggingFace {
				want := "/" + HuggingFaceTTSRepository + "/resolve/" + HuggingFaceTTSRevision + "/onnx/model_quantized.onnx"
				if u.Host != "huggingface.co" || u.Path != want {
					t.Fatalf("URL=%s want host/path huggingface.co%s", raw, want)
				}
				return
			}
			if u.Host != "www.modelscope.cn" || u.Path != "/api/v1/models/"+ModelScopeTTSRepository+"/repo" {
				t.Fatalf("unexpected ModelScope URL: %s", raw)
			}
			if u.Query().Get("Revision") != ModelScopeTTSRevision || u.Query().Get("FilePath") != "onnx/model_quantized.onnx" {
				t.Fatalf("unexpected ModelScope query: %v", u.Query())
			}
		})
	}
}

func TestEnsureTTSModelSourcesShareManifestCache(t *testing.T) {
	payloads := map[string][]byte{
		"onnx/model_quantized.onnx": []byte("model"),
		"voices/voices-v1.1-zh.bin": []byte("voices"),
		"tokenizer.json":            []byte("tokenizer"),
		"tokenizer_config.json":     []byte("tokenizer-config"),
		"config.json":               []byte("config"),
	}
	oldFiles := defaultTTSFiles
	defaultTTSFiles = nil
	for name, payload := range payloads {
		hash := sha256.Sum256(payload)
		defaultTTSFiles = append(defaultTTSFiles, modelFile{name: name, sha256: hex.EncodeToString(hash[:])})
	}
	t.Cleanup(func() { defaultTTSFiles = oldFiles })

	var mu sync.Mutex
	requests := 0
	methods := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		methods[r.Method]++
		mu.Unlock()
		name := r.URL.Query().Get("FilePath")
		if name == "" {
			prefix := "/" + HuggingFaceTTSRepository + "/resolve/" + HuggingFaceTTSRevision + "/"
			name = strings.TrimPrefix(r.URL.Path, prefix)
		}
		payload, ok := payloads[filepath.ToSlash(name)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	oldMS, oldHF := modelScopeDownloadEndpoint, huggingFaceDownloadEndpoint
	modelScopeDownloadEndpoint, huggingFaceDownloadEndpoint = server.URL, server.URL
	t.Cleanup(func() { modelScopeDownloadEndpoint, huggingFaceDownloadEndpoint = oldMS, oldHF })

	dir := t.TempDir()
	if _, err := EnsureTTSModelFrom(context.Background(), dir, ModelSourceModelScope, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureTTSModelFrom(context.Background(), dir, ModelSourceHuggingFace, nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if requests != len(payloads) {
		t.Fatalf("requests=%d want %d; sources did not share cache", requests, len(payloads))
	}
	if methods[http.MethodGet] != len(payloads) || len(methods) != 1 {
		t.Fatalf("download methods=%v; ModelScope LFS must use GET only", methods)
	}
}
