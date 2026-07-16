//go:build windows

package processmetrics

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	psapi                = syscall.NewLazyDLL("psapi.dll")
	getCurrentProcess    = kernel32.NewProc("GetCurrentProcess")
	getProcessTimes      = kernel32.NewProc("GetProcessTimes")
	getProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
)

type filetime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

type processMemoryCountersEx struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

// Read captures GetProcessTimes and GetProcessMemoryInfo for this process.
func Read() (Snapshot, error) {
	handle, _, _ := getCurrentProcess.Call()
	var creation, exit, kernel, user filetime
	ok, _, callErr := getProcessTimes.Call(handle,
		uintptr(unsafe.Pointer(&creation)), uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ok == 0 {
		return Snapshot{}, fmt.Errorf("GetProcessTimes: %w", callErr)
	}
	memory := processMemoryCountersEx{CB: uint32(unsafe.Sizeof(processMemoryCountersEx{}))}
	ok, _, callErr = getProcessMemoryInfo.Call(handle, uintptr(unsafe.Pointer(&memory)), uintptr(memory.CB))
	if ok == 0 {
		return Snapshot{}, fmt.Errorf("GetProcessMemoryInfo: %w", callErr)
	}
	units100ns := func(value filetime) uint64 { return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime) }
	return Snapshot{
		CPUTime:        time.Duration(units100ns(kernel)+units100ns(user)) * 100,
		WorkingSet:     uint64(memory.WorkingSetSize),
		PeakWorkingSet: uint64(memory.PeakWorkingSetSize),
		PrivateBytes:   uint64(memory.PrivateUsage),
	}, nil
}
