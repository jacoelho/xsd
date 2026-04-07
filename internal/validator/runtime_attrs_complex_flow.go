package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrsClassified(ct *runtime.ComplexType, present []bool, inputAttrs []Start, classes []Class, resolver value.NSResolver, storeAttrs bool, validated []Start) ([]Start, bool, error) {
	return ValidateComplex(
		s.rt,
		ct,
		present,
		inputAttrs,
		classes,
		storeAttrs,
		validated,
		ComplexCallbacks{
			AppendRaw: func(validated []Start, attr Start, storeAttrs bool) []Start {
				return StoreRaw(validated, attr, storeAttrs, s.ensureAttrNameStable, s.storeValue)
			},
			ValidateUse: func(
				validated []Start,
				attr Start,
				use runtime.AttrUse,
				storeAttrs bool,
				seenID *bool,
			) ([]Start, error) {
				return s.validateComplexAttrUse(validated, attr, resolver, storeAttrs, use, seenID)
			},
			ValidateWildcard: func(
				validated []Start,
				attr Start,
				anyAttr runtime.WildcardID,
				storeAttrs bool,
				seenID *bool,
			) ([]Start, error) {
				return s.validateComplexWildcardAttr(validated, attr, resolver, storeAttrs, anyAttr, seenID)
			},
		},
	)
}
