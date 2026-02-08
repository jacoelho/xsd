package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

func validateGroupReferencesInSchema(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range sortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		if err := validateGroupReferences(sch, qname, group); err != nil {
			errs = append(errs, fmt.Errorf("group %s: %w", qname, err))
		}
	}

	return errs
}
