package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateLocalElementValueConstraints(sch *parser.Schema, index *iterationIndex) []error {
	if index == nil {
		index = buildIterationIndex(sch)
	}

	seenLocal := make(map[*model.ElementDecl]bool)

	var errs []error
	errs = append(errs, validateLocalElementValueConstraintsFromElements(sch, index, seenLocal)...)
	errs = append(errs, validateLocalElementValueConstraintsFromTypes(sch, index, seenLocal)...)
	return errs
}

func validateLocalElementValueConstraintsFromElements(sch *parser.Schema, index *iterationIndex, seenLocal map[*model.ElementDecl]bool) []error {
	var errs []error
	for _, qname := range index.elementQNames {
		decl := sch.ElementDecls[qname]
		ct, ok := decl.Type.(*model.ComplexType)
		if !ok {
			continue
		}
		errs = append(errs, validateLocalElementValueConstraintsInComplexType(sch, ct, seenLocal)...)
	}
	return errs
}

func validateLocalElementValueConstraintsFromTypes(sch *parser.Schema, index *iterationIndex, seenLocal map[*model.ElementDecl]bool) []error {
	var errs []error
	for _, qname := range index.typeQNames {
		typ := sch.TypeDefs[qname]
		ct, ok := typ.(*model.ComplexType)
		if !ok {
			continue
		}
		errs = append(errs, validateLocalElementValueConstraintsInComplexType(sch, ct, seenLocal)...)
	}
	return errs
}

func validateLocalElementValueConstraintsInComplexType(sch *parser.Schema, ct *model.ComplexType, seenLocal map[*model.ElementDecl]bool) []error {
	var errs []error
	for _, elem := range CollectElementDeclsFromComplexType(sch, ct) {
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
	return errs
}
