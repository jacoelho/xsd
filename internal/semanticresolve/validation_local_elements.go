package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
)

func validateLocalElementValueConstraints(sch *parser.Schema) []error {
	var errs []error

	seenLocal := make(map[*model.ElementDecl]bool)
	validateLocals := func(ct *model.ComplexType) {
		for _, elem := range traversal.CollectElementDeclsFromComplexType(sch, ct) {
			if elem == nil || elem.IsReference {
				continue
			}
			if seenLocal[elem] {
				continue
			}
			seenLocal[elem] = true
			if err := validateElementValueConstraints(sch, elem); err != nil {
				errs = append(errs, fmt.Errorf("local element %s: %w", elem.Name, err))
			}
		}
	}
	for _, qname := range traversal.SortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			validateLocals(ct)
		}
	}
	for _, qname := range traversal.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*model.ComplexType); ok {
			validateLocals(ct)
		}
	}

	return errs
}
