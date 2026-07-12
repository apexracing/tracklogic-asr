package sensevoice

import (
	"context"
	"runtime"
	"testing"
)

func TestEnsureRuntimeFromEmbeddedDLL(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("embedded runtime is windows/amd64")
	}
	path, err := EnsureRuntime(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !validFile(path, runtimeDLLHash) {
		t.Fatalf("extracted runtime failed SHA-256 verification: %s", path)
	}
}
