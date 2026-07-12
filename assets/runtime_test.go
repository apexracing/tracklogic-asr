package assets

import (
	"context"
	"os"
	"runtime"
	"testing"
)

func TestEnsureRuntimeFromEmbeddedDLL(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("embedded runtime is windows/amd64")
	}
	dir := t.TempDir()
	path, err := EnsureRuntime(context.Background(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !validFile(path, runtimeDLLHash) {
		t.Fatalf("extracted runtime failed verification: %s", path)
	}
	if err = os.WriteFile(path, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = EnsureRuntime(context.Background(), dir, nil); err != nil {
		t.Fatal(err)
	}
	if !validFile(path, runtimeDLLHash) {
		t.Fatal("corrupt runtime was not replaced")
	}
}

func TestEnsureModelCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := EnsureModel(ctx, t.TempDir(), nil); err == nil {
		t.Fatal("expected cancellation error")
	}
}
