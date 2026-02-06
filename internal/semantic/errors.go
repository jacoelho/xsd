package semantic

import (
	"errors"
	"slices"
	"strings"
)

// FormatValidationErrors returns deterministic schema validation output.
func FormatValidationErrors(validationErrors []error) error {
	if len(validationErrors) == 0 {
		return nil
	}
	errs := validationErrors
	if len(validationErrors) > 1 {
		errs = make([]error, len(validationErrors))
		copy(errs, validationErrors)
		slices.SortStableFunc(errs, func(a, b error) int {
			return strings.Compare(a.Error(), b.Error())
		})
	}
	var errMsg strings.Builder
	errMsg.WriteString("schema validation failed:")
	for _, err := range errs {
		errMsg.WriteString("\n  - ")
		errMsg.WriteString(err.Error())
	}
	return errors.New(errMsg.String())
}
