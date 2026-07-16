// Package processmetrics captures operating-system process CPU and memory counters.
package processmetrics

import "time"

// Snapshot contains cumulative CPU time and current/peak process memory.
type Snapshot struct {
	CPUTime        time.Duration
	WorkingSet     uint64
	PeakWorkingSet uint64
	PrivateBytes   uint64
}
