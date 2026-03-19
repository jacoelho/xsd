package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrsClassified(ct *runtime.ComplexType, present []bool, inputAttrs []attrs.Start, classes []attrs.Class, resolver value.NSResolver, storeAttrs bool, validated []attrs.Start) ([]attrs.Start, bool, error) {
	return attrs.ValidateComplex(
		s.rt,
		ct,
		present,
		inputAttrs,
		classes,
		storeAttrs,
		validated,
		attrs.ComplexCallbacks{
			AppendRaw: s.appendRawValidatedAttr,
			ValidateUse: func(
				validated []attrs.Start,
				attr attrs.Start,
				use runtime.AttrUse,
				storeAttrs bool,
				seenID *bool,
			) ([]attrs.Start, error) {
				return s.validateComplexAttrUse(validated, attr, resolver, storeAttrs, use, seenID)
			},
			ValidateWildcard: func(
				validated []attrs.Start,
				attr attrs.Start,
				anyAttr runtime.WildcardID,
				storeAttrs bool,
				seenID *bool,
			) ([]attrs.Start, error) {
				return s.validateComplexWildcardAttr(validated, attr, resolver, storeAttrs, anyAttr, seenID)
			},
		},
	)
}
