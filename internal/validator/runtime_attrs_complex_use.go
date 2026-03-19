package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrUse(
	validated []attrs.Start,
	attr attrs.Start,
	resolver value.NSResolver,
	storeAttrs bool,
	use runtime.AttrUse,
	seenID *bool,
) ([]attrs.Start, error) {
	return s.validateComplexAttrValue(validated, attr, resolver, storeAttrs, attrValidationSpecFromAttrUse(use), seenID)
}
