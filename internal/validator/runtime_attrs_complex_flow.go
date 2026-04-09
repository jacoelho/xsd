package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrsClassified(ct *runtime.ComplexType, present []bool, inputAttrs []Start, classes []Class, resolver value.NSResolver, storeAttrs, storeValues bool, validated []Start) ([]Start, bool, error) {
	seenID := false

	for i, attr := range inputAttrs {
		class := classifyFor(s.rt, classes, i, attr)
		switch class {
		case ClassXSIUnknown:
			return nil, seenID, xsderrors.New(xsderrors.ErrAttributeNotDeclared, "unknown xsi attribute")
		case ClassXSIKnown, ClassXML:
			if storeValues {
				validated = StoreRaw(validated, attr, storeAttrs, s.ensureAttrNameStable, s.storeValue)
			} else {
				validated = StoreRawIdentity(validated, attr, storeAttrs, s.ensureAttrNameStable)
			}
			continue
		}

		if attr.Sym != 0 {
			use, idx, ok := LookupUse(s.rt, ct.Attrs, attr.Sym)
			if ok {
				if use.Use == runtime.AttrProhibited {
					return nil, seenID, xsderrors.New(xsderrors.ErrAttributeProhibited, "attribute prohibited")
				}
				out, err := s.validateComplexAttrUse(validated, attr, resolver, storeAttrs, storeValues, use, &seenID)
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
		out, err := s.validateComplexWildcardAttr(validated, attr, resolver, storeAttrs, storeValues, ct.AnyAttr, &seenID)
		if err != nil {
			return nil, seenID, err
		}
		validated = out
	}

	return validated, seenID, nil
}
