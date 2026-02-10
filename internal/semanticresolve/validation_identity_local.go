package semanticresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/xpath"
)

func validateLocalIdentityConstraintKeyrefs(sch *parser.Schema, allConstraints []*model.IdentityConstraint) []error {
	var errs []error

	forEachLocalConstraintElement(sch, func(elem *model.ElementDecl) {
		if err := validateKeyrefConstraints(elem.Name, elem.Constraints, allConstraints); err != nil {
			errs = append(errs, err...)
		}
	})

	return errs
}

func validateLocalIdentityConstraintResolution(sch *parser.Schema) []error {
	var errs []error

	forEachLocalConstraintElement(sch, func(elem *model.ElementDecl) {
		for _, constraint := range elem.Constraints {
			if err := validateIdentityConstraintResolution(sch, constraint, elem); err != nil {
				if errors.Is(err, xpath.ErrInvalidXPath) {
					continue
				}
				errs = append(errs, fmt.Errorf("element %s local identity constraint '%s': %w", elem.Name, constraint.Name, err))
			}
		}
	})

	return errs
}

func forEachLocalConstraintElement(sch *parser.Schema, visit func(*model.ElementDecl)) {
	if sch == nil || visit == nil {
		return
	}
	seen := make(map[*model.ElementDecl]bool)
	validateLocals := func(ct *model.ComplexType) {
		for _, elem := range collectConstraintElementsFromContent(ct.Content()) {
			if elem == nil || elem.IsReference || len(elem.Constraints) == 0 {
				continue
			}
			if seen[elem] {
				continue
			}
			seen[elem] = true
			visit(elem)
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
}
