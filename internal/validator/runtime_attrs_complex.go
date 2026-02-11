package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrs(ct *runtime.ComplexType, present []bool, attrs []StartAttr, resolver value.NSResolver, storeAttrs bool) ([]StartAttr, bool, error) {
	classified, err := s.classifyAttrs(attrs, false)
	if err != nil {
		return nil, false, err
	}
	return s.validateComplexAttrsClassified(ct, present, attrs, classified.classes, resolver, storeAttrs)
}

func (s *Session) validateComplexAttrsClassified(ct *runtime.ComplexType, present []bool, attrs []StartAttr, classes []attrClass, resolver value.NSResolver, storeAttrs bool) ([]StartAttr, bool, error) {
	var validated []StartAttr
	if storeAttrs {
		validated = s.attrValidatedBuf[:0]
		if cap(validated) < len(attrs) {
			validated = make([]StartAttr, 0, len(attrs))
		}
	}
	seenID := false

	for i, attr := range attrs {
		class := attrClassOther
		if i < len(classes) {
			class = classes[i]
		} else {
			class, _ = s.classifyAttr(&attr)
		}

		if class == attrClassXsiUnknown {
			return nil, seenID, newValidationError(xsderrors.ErrAttributeNotDeclared, "unknown xsi attribute")
		}
		if class == attrClassXsiKnown {
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

		if class == attrClassXML {
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
		var wildcardAttr runtime.AttrID
		resolved, err := resolveWildcardSymbol(rule.PC, attr.Sym, func(symbol runtime.SymbolID) bool {
			id, ok := s.globalAttributeBySymbol(symbol)
			if !ok {
				return false
			}
			wildcardAttr = id
			return true
		}, func() error {
			return newValidationError(xsderrors.ErrValidateWildcardAttrStrictUnresolved, "attribute wildcard strict unresolved")
		})
		if err != nil {
			return nil, seenID, err
		}
		if !resolved {
			validated = s.appendRawValidatedAttr(validated, attr, storeAttrs)
			continue
		}
		if int(wildcardAttr) >= len(s.rt.Attributes) {
			return nil, seenID, fmt.Errorf("attribute %d out of range", wildcardAttr)
		}
		globalAttr := s.rt.Attributes[wildcardAttr]
		validated, err = s.validateComplexAttrValue(validated, attr, resolver, storeAttrs, attrValidationSpec{
			validator:   globalAttr.Validator,
			fixed:       globalAttr.Fixed,
			fixedKey:    globalAttr.FixedKey,
			fixedMember: globalAttr.FixedMember,
		}, &seenID)
		if err != nil {
			return nil, seenID, err
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
