//go:build !windows

package processmetrics

import "fmt"

// Read is currently implemented only on Windows.
func Read() (Snapshot, error) { return Snapshot{}, fmt.Errorf("process metrics require Windows") }
