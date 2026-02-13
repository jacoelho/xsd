package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

func validateLocalElementValueConstraints(sch *parser.Schema, index *iterationIndex) []error {
	var errs []error
	if index == nil {
		index = buildIterationIndex(sch)
	}

	seenLocal := make(map[*types.ElementDecl]bool)
	validateLocals := func(ct *types.ComplexType) {
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
	for _, qname := range index.elementQNames {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			validateLocals(ct)
		}
	}
	for _, qname := range index.typeQNames {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			validateLocals(ct)
		}
	}

	return errs
}
