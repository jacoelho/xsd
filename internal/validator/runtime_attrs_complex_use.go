package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrUse(
	validated []Start,
	attr Start,
	resolver value.NSResolver,
	storeAttrs bool,
	storeValues bool,
	use runtime.AttrUse,
	seenID *bool,
) ([]Start, error) {
	return s.validateComplexAttrValue(validated, attr, resolver, storeAttrs, storeValues, attrValidationSpecFromAttrUse(use), seenID)
}
