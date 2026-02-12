package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) tryValidateComplexDeclaredAttr(
	ct *runtime.ComplexType,
	present []bool,
	validated []StartAttr,
	attr StartAttr,
	resolver value.NSResolver,
	storeAttrs bool,
	seenID *bool,
) ([]StartAttr, bool, error) {
	if attr.Sym == 0 {
		return validated, false, nil
	}

	use, idx, ok := lookupAttrUse(s.rt, ct.Attrs, attr.Sym)
	if !ok {
		return validated, false, nil
	}
	if use.Use == runtime.AttrProhibited {
		return nil, true, newValidationError(xsderrors.ErrAttributeProhibited, "attribute prohibited")
	}

	out, err := s.validateComplexAttrValue(validated, attr, resolver, storeAttrs, attrValidationSpecFromAttrUse(use), seenID)
	if err != nil {
		return nil, true, err
	}
	if idx >= 0 && idx < len(present) {
		present[idx] = true
	}
	return out, true, nil
}
