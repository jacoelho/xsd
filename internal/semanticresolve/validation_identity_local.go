package semanticresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/xpath"
)

func validateLocalIdentityConstraintKeyrefsWithIndex(sch *parser.Schema, index *iterationIndex, allConstraints []*model.IdentityConstraint) []error {
	var errs []error

	forEachLocalConstraintElement(sch, index, func(elem *model.ElementDecl) {
		if err := validateKeyrefConstraints(elem.Name, elem.Constraints, allConstraints); err != nil {
			errs = append(errs, err...)
		}
	})

	return errs
}

func validateLocalIdentityConstraintResolution(sch *parser.Schema, index *iterationIndex) []error {
	var errs []error

	forEachLocalConstraintElement(sch, index, func(elem *model.ElementDecl) {
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

func forEachLocalConstraintElement(sch *parser.Schema, index *iterationIndex, visit func(*model.ElementDecl)) {
	if sch == nil || visit == nil {
		return
	}
	if index == nil {
		index = buildIterationIndex(sch)
	}
	for _, elem := range index.localConstraintElems {
		visit(elem)
	}
}

func collectLocalConstraintElementsWithIndex(sch *parser.Schema, index *iterationIndex) []*model.ElementDecl {
	if sch == nil {
		return nil
	}
	if index == nil {
		index = buildIterationIndex(sch)
	}
	seen := make(map[*model.ElementDecl]bool)
	out := make([]*model.ElementDecl, 0)
	collect := func(content model.Content) {
		for _, elem := range collectConstraintElementsFromContent(content) {
			if elem == nil || elem.IsReference || len(elem.Constraints) == 0 {
				continue
			}
			if seen[elem] {
				continue
			}
			seen[elem] = true
			out = append(out, elem)
		}
	}
	for _, qname := range index.elementQNames {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*model.ComplexType); ok {
			collect(ct.Content())
		}
	}
	for _, qname := range index.typeQNames {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*model.ComplexType); ok {
			collect(ct.Content())
		}
	}
	return out
}
