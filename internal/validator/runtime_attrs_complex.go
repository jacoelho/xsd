package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrs(ct *runtime.ComplexType, present []bool, attrs []StartAttr, resolver value.NSResolver, storeAttrs bool) ([]StartAttr, bool, error) {
	var validated []StartAttr
	if storeAttrs {
		validated = s.attrValidatedBuf[:0]
		if cap(validated) < len(attrs) {
			validated = make([]StartAttr, 0, len(attrs))
		}
	}
	seenID := false

	for _, attr := range attrs {
		if s.isUnknownXsiAttribute(&attr) {
			return nil, seenID, newValidationError(xsderrors.ErrAttributeNotDeclared, "unknown xsi attribute")
		}
		if s.isXsiAttribute(&attr) {
			validated = s.appendRawValidatedAttr(validated, attr, storeAttrs)
			continue
		}

		if attr.Sym != 0 {
			if use, idx, ok := lookupAttrUse(s.rt, ct.Attrs, attr.Sym); ok {
				if use.Use == runtime.AttrProhibited {
					return nil, seenID, newValidationError(xsderrors.ErrAttributeProhibited, "attribute prohibited")
				}
				canon, metrics, err := s.validateValueInternalWithMetrics(use.Validator, attr.Value, resolver, valueOptions{
					applyWhitespace:  true,
					trackIDs:         true,
					requireCanonical: use.Fixed.Present,
					storeValue:       storeAttrs,
					needKey:          use.Fixed.Present,
				})
				if err != nil {
					return nil, seenID, wrapValueError(err)
				}
				if s.isIDValidator(use.Validator) {
					if seenID {
						return nil, seenID, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
					}
					seenID = true
				}
				validated = s.appendValidatedAttr(validated, attr, storeAttrs, canon, metrics.keyKind, metrics.keyBytes)
				if use.Fixed.Present {
					match, err := s.fixedValueMatches(use.Validator, use.FixedMember, canon, metrics, resolver, use.Fixed, use.FixedKey)
					if err != nil {
						return nil, seenID, err
					}
					if !match {
						return nil, seenID, newValidationError(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
					}
				}
				if idx >= 0 && idx < len(present) {
					present[idx] = true
				}
				continue
			}
		}

		if s.isXMLAttribute(&attr) {
			validated = s.appendRawValidatedAttr(validated, attr, storeAttrs)
			continue
		}

		if ct.AnyAttr == 0 {
			return nil, seenID, newValidationError(xsderrors.ErrAttributeNotDeclared, "attribute not declared")
		}
		if !s.rt.WildcardAccepts(ct.AnyAttr, attr.NSBytes, attr.NS) {
			return nil, seenID, newValidationError(xsderrors.ErrAttributeNotDeclared, "attribute wildcard rejected namespace")
		}

		rule := s.rt.Wildcards[ct.AnyAttr]
		switch rule.PC {
		case runtime.PCSkip:
			validated = s.appendRawValidatedAttr(validated, attr, storeAttrs)
			continue
		case runtime.PCLax, runtime.PCStrict:
			if attr.Sym == 0 {
				if rule.PC == runtime.PCStrict {
					return nil, seenID, newValidationError(xsderrors.ErrValidateWildcardAttrStrictUnresolved, "attribute wildcard strict unresolved")
				}
				validated = s.appendRawValidatedAttr(validated, attr, storeAttrs)
				continue
			}
			id, ok := s.globalAttributeBySymbol(attr.Sym)
			if !ok {
				if rule.PC == runtime.PCStrict {
					return nil, seenID, newValidationError(xsderrors.ErrValidateWildcardAttrStrictUnresolved, "attribute wildcard strict unresolved")
				}
				validated = s.appendRawValidatedAttr(validated, attr, storeAttrs)
				continue
			}
			if int(id) >= len(s.rt.Attributes) {
				return nil, seenID, fmt.Errorf("attribute %d out of range", id)
			}
			globalAttr := s.rt.Attributes[id]
			canon, metrics, err := s.validateValueInternalWithMetrics(globalAttr.Validator, attr.Value, resolver, valueOptions{
				applyWhitespace:  true,
				trackIDs:         true,
				requireCanonical: globalAttr.Fixed.Present,
				storeValue:       storeAttrs,
				needKey:          globalAttr.Fixed.Present,
			})
			if err != nil {
				return nil, seenID, wrapValueError(err)
			}
			if s.isIDValidator(globalAttr.Validator) {
				if seenID {
					return nil, seenID, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
				}
				seenID = true
			}
			validated = s.appendValidatedAttr(validated, attr, storeAttrs, canon, metrics.keyKind, metrics.keyBytes)
			if globalAttr.Fixed.Present {
				match, err := s.fixedValueMatches(globalAttr.Validator, globalAttr.FixedMember, canon, metrics, resolver, globalAttr.Fixed, globalAttr.FixedKey)
				if err != nil {
					return nil, seenID, err
				}
				if !match {
					return nil, seenID, newValidationError(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
				}
			}
		default:
			return nil, seenID, fmt.Errorf("unknown wildcard processContents %d", rule.PC)
		}
	}

	return validated, seenID, nil
}

func (s *Session) appendRawValidatedAttr(validated []StartAttr, attr StartAttr, storeAttrs bool) []StartAttr {
	if !storeAttrs {
		return validated
	}
	s.ensureAttrNameStable(&attr)
	attr.Value = s.storeValue(attr.Value)
	attr.KeyKind = runtime.VKInvalid
	attr.KeyBytes = nil
	return append(validated, attr)
}

func (s *Session) appendValidatedAttr(validated []StartAttr, attr StartAttr, storeAttrs bool, canonical []byte, keyKind runtime.ValueKind, keyBytes []byte) []StartAttr {
	if !storeAttrs {
		return validated
	}
	s.ensureAttrNameStable(&attr)
	attr.Value = canonical
	attr.KeyKind = keyKind
	attr.KeyBytes = keyBytes
	return append(validated, attr)
}
