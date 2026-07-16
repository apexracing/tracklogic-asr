// Package ortutil coordinates the process-global ONNX Runtime environment.
package ortutil

import (
	"fmt"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var state struct {
	sync.Mutex
	users int
	path  string
	owned bool
}

// Acquire initializes or shares the runtime at path.
func Acquire(path string) error {
	state.Lock()
	defer state.Unlock()
	if state.users > 0 {
		if state.path != path {
			return fmt.Errorf("ONNX Runtime already initialized from %s", state.path)
		}
		state.users++
		return nil
	}
	if ort.IsInitialized() {
		state.users = 1
		state.path = path
		state.owned = false
		return nil
	}
	ort.SetSharedLibraryPath(path)
	if err := ort.InitializeEnvironment(ort.WithLogLevelWarning()); err != nil {
		return fmt.Errorf("initialize ONNX Runtime from %s: %w", path, err)
	}
	state.users = 1
	state.path = path
	state.owned = true
	return nil
}

// Release drops one runtime reference.
func Release() {
	state.Lock()
	defer state.Unlock()
	if state.users == 0 {
		return
	}
	state.users--
	if state.users == 0 {
		if state.owned {
			_ = ort.DestroyEnvironment()
		}
		state.path = ""
		state.owned = false
	}
}
