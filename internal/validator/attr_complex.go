package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// ComplexCallbacks supplies the session-bound behavior needed to validate one
// complex-type attribute set.
type ComplexCallbacks struct {
	AppendRaw   func(validated []Start, attr Start, storeAttrs bool) []Start
	ValidateUse func(
		validated []Start,
		attr Start,
		use runtime.AttrUse,
		storeAttrs bool,
		seenID *bool,
	) ([]Start, error)
	ValidateWildcard func(
		validated []Start,
		attr Start,
		wildcard runtime.WildcardID,
		storeAttrs bool,
		seenID *bool,
	) ([]Start, error)
}

// ValidateComplex validates one complex-type attribute set against runtime
// attribute uses and wildcard policy, delegating value checks through callbacks.
func ValidateComplex(
	rt *runtime.Schema,
	ct *runtime.ComplexType,
	present []bool,
	input []Start,
	classes []Class,
	storeAttrs bool,
	validated []Start,
	callbacks ComplexCallbacks,
) ([]Start, bool, error) {
	seenID := false

	for i, attr := range input {
		class := classifyFor(rt, classes, i, attr)

		switch class {
		case ClassXSIUnknown:
			return nil, seenID, xsderrors.New(xsderrors.ErrAttributeNotDeclared, "unknown xsi attribute")
		case ClassXSIKnown, ClassXML:
			validated = callbacks.AppendRaw(validated, attr, storeAttrs)
			continue
		}

		if attr.Sym != 0 {
			use, idx, ok := LookupUse(rt, ct.Attrs, attr.Sym)
			if ok {
				if use.Use == runtime.AttrProhibited {
					return nil, seenID, xsderrors.New(xsderrors.ErrAttributeProhibited, "attribute prohibited")
				}
				out, err := callbacks.ValidateUse(validated, attr, use, storeAttrs, &seenID)
				if err != nil {
					return nil, seenID, err
				}
				validated = out
				if idx >= 0 && idx < len(present) {
					present[idx] = true
				}
				continue
			}
		}

		if ct.AnyAttr == 0 {
			return nil, seenID, xsderrors.New(xsderrors.ErrAttributeNotDeclared, "attribute not declared")
		}
		out, err := callbacks.ValidateWildcard(validated, attr, ct.AnyAttr, storeAttrs, &seenID)
		if err != nil {
			return nil, seenID, err
		}
		validated = out
	}

	return validated, seenID, nil
}

func classifyFor(rt *runtime.Schema, classes []Class, index int, attr Start) Class {
	if index < len(classes) {
		return classes[index]
	}
	return ClassifyOne(rt, &attr)
}
