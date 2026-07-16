package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestModelFileURL(t *testing.T) {
	tests := []struct {
		name       string
		source     ModelSource
		host       string
		path       string
		revision   string
		queryFile  string
		pathSuffix string
	}{
		{
			name:      "default-modelscope",
			host:      "www.modelscope.cn",
			path:      "/api/v1/models/" + ModelScopeModelRepository + "/repo",
			revision:  ModelScopeModelRevision,
			queryFile: "model_quant.onnx",
		},
		{
			name:      "modelscope",
			source:    ModelSourceModelScope,
			host:      "www.modelscope.cn",
			path:      "/api/v1/models/" + ModelScopeModelRepository + "/repo",
			revision:  ModelScopeModelRevision,
			queryFile: "model_quant.onnx",
		},
		{
			name:       "huggingface",
			source:     ModelSourceHuggingFace,
			host:       "huggingface.co",
			pathSuffix: "/" + HuggingFaceModelRepository + "/resolve/" + HuggingFaceModelRevision + "/model_quant.onnx",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := modelFileURL(test.source, "model_quant.onnx")
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := url.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Host != test.host {
				t.Fatalf("host=%q want %q", parsed.Host, test.host)
			}
			if test.path != "" {
				if parsed.Path != test.path {
					t.Fatalf("path=%q want %q", parsed.Path, test.path)
				}
				if parsed.Query().Get("Revision") != test.revision || parsed.Query().Get("FilePath") != test.queryFile {
					t.Fatalf("query=%v", parsed.Query())
				}
			}
			if test.pathSuffix != "" && !strings.HasSuffix(parsed.Path, test.pathSuffix) {
				t.Fatalf("path=%q want suffix %q", parsed.Path, test.pathSuffix)
			}
		})
	}
}

func TestEnsureModelFromSelectsSource(t *testing.T) {
	payloads := installTestModelFiles(t)
	var mu sync.Mutex
	var hits []ModelSource
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var source ModelSource
		var name string
		switch {
		case r.URL.Path == "/api/v1/models/"+ModelScopeModelRepository+"/repo":
			source = ModelSourceModelScope
			name = r.URL.Query().Get("FilePath")
			if r.URL.Query().Get("Revision") != ModelScopeModelRevision {
				http.Error(w, "bad modelscope revision", http.StatusBadRequest)
				return
			}
		case strings.HasPrefix(r.URL.Path, "/"+HuggingFaceModelRepository+"/resolve/"+HuggingFaceModelRevision+"/"):
			source = ModelSourceHuggingFace
			name = filepath.Base(r.URL.Path)
		default:
			http.NotFound(w, r)
			return
		}
		payload, ok := payloads[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		hits = append(hits, source)
		mu.Unlock()
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	oldModelScopeEndpoint := modelScopeDownloadEndpoint
	oldHuggingFaceEndpoint := huggingFaceDownloadEndpoint
	modelScopeDownloadEndpoint = server.URL
	huggingFaceDownloadEndpoint = server.URL
	t.Cleanup(func() {
		modelScopeDownloadEndpoint = oldModelScopeEndpoint
		huggingFaceDownloadEndpoint = oldHuggingFaceEndpoint
	})

	tests := []struct {
		name string
		src  ModelSource
		want ModelSource
	}{
		{name: "default", want: ModelSourceModelScope},
		{name: "modelscope", src: ModelSourceModelScope, want: ModelSourceModelScope},
		{name: "huggingface", src: ModelSourceHuggingFace, want: ModelSourceHuggingFace},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mu.Lock()
			hits = nil
			mu.Unlock()
			if _, err := EnsureModelFrom(context.Background(), t.TempDir(), test.src, nil); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(hits) != len(defaultModelFiles) {
				t.Fatalf("hits=%v", hits)
			}
			for _, hit := range hits {
				if hit != test.want {
					t.Fatalf("hit source=%q want %q", hit, test.want)
				}
			}
		})
	}
}

func TestEnsureModelSourcesShareCache(t *testing.T) {
	payloads := installTestModelFiles(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		name := r.URL.Query().Get("FilePath")
		if name == "" {
			name = filepath.Base(r.URL.Path)
		}
		payload, ok := payloads[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	oldModelScopeEndpoint := modelScopeDownloadEndpoint
	oldHuggingFaceEndpoint := huggingFaceDownloadEndpoint
	modelScopeDownloadEndpoint = server.URL
	huggingFaceDownloadEndpoint = server.URL
	t.Cleanup(func() {
		modelScopeDownloadEndpoint = oldModelScopeEndpoint
		huggingFaceDownloadEndpoint = oldHuggingFaceEndpoint
	})

	dir := t.TempDir()
	if _, err := EnsureModelFrom(context.Background(), dir, ModelSourceModelScope, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureModelFrom(context.Background(), dir, ModelSourceHuggingFace, nil); err != nil {
		t.Fatal(err)
	}
	if requests != len(defaultModelFiles) {
		t.Fatalf("requests=%d want %d; second source did not reuse cache", requests, len(defaultModelFiles))
	}
}

func TestEnsureModelFromRejectsUnknownSource(t *testing.T) {
	_, err := EnsureModelFrom(context.Background(), t.TempDir(), ModelSource("unknown"), nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported model source") {
		t.Fatalf("err=%v", err)
	}
}

func installTestModelFiles(t *testing.T) map[string][]byte {
	t.Helper()
	payloads := map[string][]byte{
		"model_quant.onnx": []byte("model"),
		"tokens.json":      []byte("tokens"),
		"am.mvn":           []byte("cmvn"),
		"config.yaml":      []byte("config"),
	}
	old := defaultModelFiles
	defaultModelFiles = nil
	for _, name := range []string{"model_quant.onnx", "tokens.json", "am.mvn", "config.yaml"} {
		hash := sha256.Sum256(payloads[name])
		defaultModelFiles = append(defaultModelFiles, modelFile{name: name, sha256: hex.EncodeToString(hash[:])})
	}
	t.Cleanup(func() { defaultModelFiles = old })
	return payloads
}

func TestDefaultModelCacheDirRemainsCompatible(t *testing.T) {
	dir, err := defaultModelCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != HuggingFaceModelRevision {
		t.Fatalf("cache dir=%q", dir)
	}
	if _, err := os.Stat(filepath.Dir(dir)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
