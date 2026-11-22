// Command compilework compiles one schema document and reports the analysis
// and dependency steps the compilation charges. It is built with coverage
// counters by TestSequenceRestrictionOfAnAllGroupDoesWorkInProportionToChargedSteps,
// which reads the statements one default-options compilation executes.
//
// Usage:
//
//	compilework <schema.xsd>
//
// It prints "<analysis steps> <dependency steps>": for each budget, the least
// limit under which the schema compiles. The counters it leaves in GOCOVERDIR
// count only the final compilation under default options.
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/coverage"
	"slices"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

// searchCeiling is above the steps of every schema the test measures, all of
// which compile within the default limits.
const searchCeiling = 1 << 24

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: compilework <schema.xsd>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "compilework:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	data, readErr := os.ReadFile(path) //nolint:gosec // the path names the schema the caller asks to measure.
	if readErr != nil {
		return readErr
	}
	compile := func(opts xsd.CompileOptions) error {
		_, err := xsd.CompileWithOptions(opts, xsd.Bytes(path, data))
		return err
	}
	analysis, analysisErr := leastLimit(func(limit int) error {
		return compile(xsd.CompileOptions{MaxContentModelAnalysisSteps: limit})
	})
	if analysisErr != nil {
		return fmt.Errorf("analysis steps: %w", analysisErr)
	}
	dependency, dependencyErr := leastLimit(func(limit int) error {
		return compile(xsd.CompileOptions{MaxSchemaDependencySteps: limit})
	})
	if dependencyErr != nil {
		return fmt.Errorf("dependency steps: %w", dependencyErr)
	}
	if err := coverage.ClearCounters(); err != nil {
		return err
	}
	if err := compile(xsd.CompileOptions{}); err != nil {
		return err
	}
	_, err := fmt.Println(analysis, dependency)
	return err
}

// leastLimit returns the least limit under which compile succeeds. Spent
// steps are never refunded, so a compilation that fails at a limit fails at
// every lower one.
func leastLimit(compile func(limit int) error) (int, error) {
	if err := compile(searchCeiling); err != nil {
		return 0, err
	}
	low, high := 1, searchCeiling
	for low < high {
		mid := low + (high-low)/2
		err := compile(mid)
		switch {
		case err == nil:
			high = mid
		case isLimit(err):
			low = mid + 1
		default:
			return 0, fmt.Errorf("limit %d: %w", mid, err)
		}
	}
	return low, nil
}

func isLimit(err error) bool {
	return slices.ContainsFunc(xsderrors.Flatten(err), func(err error) bool {
		var diagnostic *xsderrors.Error
		return errors.As(err, &diagnostic) && diagnostic.Code() == xsderrors.CodeSchemaLimit
	})
}
