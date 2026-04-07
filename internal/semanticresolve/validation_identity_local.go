package semanticresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/semantics"
)

func validateLocalIdentityConstraintKeyrefsWithIndex(sch *parser.Schema, index *iterationIndex, allConstraints []*model.IdentityConstraint) []error {
	var errs []error

	forEachLocalConstraintElement(sch, index, func(elem *model.ElementDecl) {
		if err := semantics.ValidateKeyrefConstraints(elem.Name, elem.Constraints, allConstraints); err != nil {
			errs = append(errs, err...)
		}
	})

	return errs
}

func validateLocalIdentityConstraintResolution(sch *parser.Schema, index *iterationIndex) []error {
	var errs []error

	forEachLocalConstraintElement(sch, index, func(elem *model.ElementDecl) {
		for _, constraint := range elem.Constraints {
			if err := semantics.ValidateIdentityConstraintResolution(sch, constraint, elem); err != nil {
				if errors.Is(err, runtime.ErrInvalidXPath) {
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
