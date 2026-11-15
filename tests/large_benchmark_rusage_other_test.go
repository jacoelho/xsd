//go:build !unix

package tests_test

import "os"

func processMaxRSSBytes(*os.ProcessState) (uint64, bool) {
	return 0, false
}
