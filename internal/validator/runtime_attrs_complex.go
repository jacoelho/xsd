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
				var err error
				validated, err = s.validateComplexAttrValue(validated, attr, resolver, storeAttrs, attrValidationSpec{
					validator:   use.Validator,
					fixed:       use.Fixed,
					fixedKey:    use.FixedKey,
					fixedMember: use.FixedMember,
				}, &seenID)
				if err != nil {
					return nil, seenID, err
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
			var err error
			validated, err = s.validateComplexAttrValue(validated, attr, resolver, storeAttrs, attrValidationSpec{
				validator:   globalAttr.Validator,
				fixed:       globalAttr.Fixed,
				fixedKey:    globalAttr.FixedKey,
				fixedMember: globalAttr.FixedMember,
			}, &seenID)
			if err != nil {
				return nil, seenID, err
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

type attrValidationSpec struct {
	validator   runtime.ValidatorID
	fixedMember runtime.ValidatorID
	fixed       runtime.ValueRef
	fixedKey    runtime.ValueKeyRef
}

func (s *Session) validateComplexAttrValue(
	validated []StartAttr,
	attr StartAttr,
	resolver value.NSResolver,
	storeAttrs bool,
	spec attrValidationSpec,
	seenID *bool,
) ([]StartAttr, error) {
	canon, metrics, err := s.validateValueInternalWithMetrics(spec.validator, attr.Value, resolver, valueOptions{
		applyWhitespace:  true,
		trackIDs:         true,
		requireCanonical: spec.fixed.Present,
		storeValue:       storeAttrs,
		needKey:          spec.fixed.Present,
	})
	if err != nil {
		return nil, wrapValueError(err)
	}
	if s.isIDValidator(spec.validator) {
		if *seenID {
			return nil, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
		}
		*seenID = true
	}
	validated = s.appendValidatedAttr(validated, attr, storeAttrs, canon, metrics.keyKind, metrics.keyBytes)
	if spec.fixed.Present {
		match, err := s.fixedValueMatches(spec.validator, spec.fixedMember, canon, metrics, resolver, spec.fixed, spec.fixedKey)
		if err != nil {
			return nil, err
		}
		if !match {
			return nil, newValidationError(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
		}
	}
	return validated, nil
}
