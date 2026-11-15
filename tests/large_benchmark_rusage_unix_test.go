//go:build unix

package tests_test

import (
	"os"
	"runtime"
	"syscall"
)

// processMaxRSSBytes reports the peak resident set size of an exited process.
// Darwin reports ru_maxrss in bytes; other Unix kernels report KiB.
func processMaxRSSBytes(state *os.ProcessState) (uint64, bool) {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || usage.Maxrss <= 0 {
		return 0, false
	}
	if runtime.GOOS == "darwin" {
		return uint64(usage.Maxrss), true
	}
	return uint64(usage.Maxrss) * 1024, true
}
