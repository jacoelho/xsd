package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// ValidateSimple validates attributes on a simple-type element and optionally
// appends the accepted raw attributes through appendRaw.
func ValidateSimple(
	rt *runtime.Schema,
	input []Start,
	classes []Class,
	store bool,
	validated []Start,
	appendRaw func([]Start, Start, bool) []Start,
) ([]Start, error) {
	for i, attr := range input {
		switch classifyFor(rt, classes, i, attr) {
		case ClassXSIUnknown:
			return nil, xsderrors.New(xsderrors.ErrValidateSimpleTypeAttrNotAllowed, "unknown xsi attribute")
		case ClassXSIKnown, ClassXML:
			continue
		default:
			return nil, xsderrors.New(xsderrors.ErrValidateSimpleTypeAttrNotAllowed, "attribute not allowed on simple type")
		}
	}
	if !store {
		return nil, nil
	}
	for _, attr := range input {
		validated = appendRaw(validated, attr, true)
	}
	return validated, nil
}

func classifyFor(rt *runtime.Schema, classes []Class, index int, attr Start) Class {
	if index < len(classes) {
		return classes[index]
	}
	return ClassifyOne(rt, &attr)
}
