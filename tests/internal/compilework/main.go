// Command compilework compiles one schema document and reports the analysis
// and dependency steps the compilation charges. It is built with coverage
// counters by TestContentModelCompileWorkGrowsWithChargedSteps,
// TestAttributeDerivationAuditDoesWorkInProportionToChargedSteps and
// TestTypesSharingAnAttributeGroupReadItsSetOnce, which read the statements
// one default-options compilation executes.
//
// Usage:
//
//	compilework <schema.xsd>
//
// It prints "<analysis steps> <dependency steps> <outcome>": for each budget,
// the least limit under which compilation ends other than at that limit, and
// "compiled" or "rejected" for how a final compilation under default options
// ends. It clears the counters before that compilation, so the internal/schema
// counters it leaves in GOCOVERDIR count that compilation only. A final
// compilation that ends at a limit is an error.
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
// which end within the default limits.
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
	outcome := "compiled"
	if err := compile(xsd.CompileOptions{}); isLimit(err) {
		return err
	} else if err != nil {
		outcome = "rejected"
	}
	_, err := fmt.Println(analysis, dependency, outcome)
	return err
}

// leastLimit returns the least limit under which compile ends other than at
// the limit. Spent steps are never refunded, so a compilation that ends at a
// limit ends there at every lower one. Above that limit every compilation
// ends as the one at searchCeiling does: compiled, or rejected with the same
// diagnostic codes. Any other end is an error.
func leastLimit(compile func(limit int) error) (int, error) {
	reference := compile(searchCeiling)
	if isLimit(reference) {
		return 0, reference
	}
	want := diagnosticCodes(reference)
	low, high := 1, searchCeiling
	for low < high {
		mid := low + (high-low)/2
		err := compile(mid)
		switch {
		case isLimit(err):
			low = mid + 1
		case !slices.Equal(diagnosticCodes(err), want):
			return 0, fmt.Errorf("limit %d ends with codes %v, not %v as at limit %d: %w", mid, diagnosticCodes(err), want, searchCeiling, err)
		default:
			high = mid
		}
	}
	return low, nil
}

// diagnosticCodes returns the sorted codes of err's diagnostics, an empty
// code for each entry that is not a diagnostic, and none for nil.
func diagnosticCodes(err error) []xsderrors.Code {
	var codes []xsderrors.Code
	for _, entry := range xsderrors.Flatten(err) {
		var diagnostic *xsderrors.Error
		if errors.As(entry, &diagnostic) {
			codes = append(codes, diagnostic.Code())
		} else {
			codes = append(codes, "")
		}
	}
	slices.Sort(codes)
	return codes
}

func isLimit(err error) bool {
	return slices.ContainsFunc(xsderrors.Flatten(err), func(err error) bool {
		var diagnostic *xsderrors.Error
		return errors.As(err, &diagnostic) && diagnostic.Code() == xsderrors.CodeSchemaLimit
	})
}
