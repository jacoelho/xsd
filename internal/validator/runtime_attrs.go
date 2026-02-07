package validator

import (
	"bytes"
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

type AttrApplied struct {
	KeyBytes []byte
	Value    runtime.ValueRef
	Name     runtime.SymbolID
	Fixed    bool
	KeyKind  runtime.ValueKind
}

type AttrResult struct {
	Applied []AttrApplied
	Attrs   []StartAttr
}

var (
	xsiLocalType                      = []byte("type")
	xsiLocalNil                       = []byte("nil")
	xsiLocalSchemaLocation            = []byte("schemaLocation")
	xsiLocalNoNamespaceSchemaLocation = []byte("noNamespaceSchemaLocation")
)

func (s *Session) ValidateAttributes(typeID runtime.TypeID, attrs []StartAttr, resolver value.NSResolver) (AttrResult, error) {
	if s == nil || s.rt == nil {
		return AttrResult{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}
	typ, ok := s.typeByID(typeID)
	if !ok {
		return AttrResult{}, fmt.Errorf("type %d not found", typeID)
	}
	storeAttrs := s.hasIdentityConstraints()

	if typ.Kind != runtime.TypeComplex {
		return s.validateSimpleTypeAttrs(attrs, storeAttrs)
	}

	ct := &s.rt.ComplexTypes[typ.Complex.ID]
	uses := s.attrUses(ct.Attrs)
	present := s.prepareAttrPresent(len(uses))

	if err := s.checkDuplicateAttrs(attrs); err != nil {
		return AttrResult{}, err
	}

	validated, seenID, err := s.validateComplexAttrs(ct, present, attrs, resolver, storeAttrs)
	if err != nil {
		return AttrResult{}, err
	}

	applied, err := s.applyDefaultAttrs(uses, present, storeAttrs, seenID)
	if err != nil {
		return AttrResult{}, err
	}

	result := AttrResult{Attrs: validated, Applied: applied}
	if storeAttrs {
		s.attrValidatedBuf = validated[:0]
	}
	s.attrAppliedBuf = applied[:0]
	return result, nil
}

func (s *Session) validateSimpleTypeAttrs(attrs []StartAttr, storeAttrs bool) (AttrResult, error) {
	for _, attr := range attrs {
		if s.isUnknownXsiAttribute(&attr) {
			return AttrResult{}, newValidationError(xsderrors.ErrValidateSimpleTypeAttrNotAllowed, "unknown xsi attribute")
		}
		if !s.isXsiAttribute(&attr) && !s.isXMLAttribute(&attr) {
			return AttrResult{}, newValidationError(xsderrors.ErrValidateSimpleTypeAttrNotAllowed, "attribute not allowed on simple type")
		}
	}
	if !storeAttrs {
		return AttrResult{}, nil
	}
	result := AttrResult{Attrs: make([]StartAttr, 0, len(attrs))}
	for _, attr := range attrs {
		s.ensureAttrNameStable(&attr)
		attr.Value = s.storeValue(attr.Value)
		attr.KeyKind = runtime.VKInvalid
		attr.KeyBytes = nil
		result.Attrs = append(result.Attrs, attr)
	}
	return result, nil
}

func (s *Session) prepareAttrPresent(size int) []bool {
	present := s.attrPresent
	if cap(present) < size {
		present = make([]bool, size)
	} else {
		present = present[:size]
		for i := range present {
			present[i] = false
		}
	}
	s.attrPresent = present
	return present
}

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
			if storeAttrs {
				s.ensureAttrNameStable(&attr)
				attr.Value = s.storeValue(attr.Value)
				attr.KeyKind = runtime.VKInvalid
				attr.KeyBytes = nil
				validated = append(validated, attr)
			}
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
				if storeAttrs {
					s.ensureAttrNameStable(&attr)
					attr.Value = canon
					attr.KeyKind = metrics.keyKind
					attr.KeyBytes = metrics.keyBytes
				}
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
				if storeAttrs {
					validated = append(validated, attr)
				}
				continue
			}
		}

		if s.isXMLAttribute(&attr) {
			if storeAttrs {
				s.ensureAttrNameStable(&attr)
				attr.Value = s.storeValue(attr.Value)
				attr.KeyKind = runtime.VKInvalid
				attr.KeyBytes = nil
				validated = append(validated, attr)
			}
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
			if storeAttrs {
				s.ensureAttrNameStable(&attr)
				attr.Value = s.storeValue(attr.Value)
				attr.KeyKind = runtime.VKInvalid
				attr.KeyBytes = nil
				validated = append(validated, attr)
			}
			continue
		case runtime.PCLax, runtime.PCStrict:
			if attr.Sym == 0 {
				if rule.PC == runtime.PCStrict {
					return nil, seenID, newValidationError(xsderrors.ErrValidateWildcardAttrStrictUnresolved, "attribute wildcard strict unresolved")
				}
				if storeAttrs {
					s.ensureAttrNameStable(&attr)
					attr.Value = s.storeValue(attr.Value)
					attr.KeyKind = runtime.VKInvalid
					attr.KeyBytes = nil
					validated = append(validated, attr)
				}
				continue
			}
			id, ok := s.globalAttributeBySymbol(attr.Sym)
			if !ok {
				if rule.PC == runtime.PCStrict {
					return nil, seenID, newValidationError(xsderrors.ErrValidateWildcardAttrStrictUnresolved, "attribute wildcard strict unresolved")
				}
				if storeAttrs {
					s.ensureAttrNameStable(&attr)
					attr.Value = s.storeValue(attr.Value)
					attr.KeyKind = runtime.VKInvalid
					attr.KeyBytes = nil
					validated = append(validated, attr)
				}
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
			if storeAttrs {
				s.ensureAttrNameStable(&attr)
				attr.Value = canon
				attr.KeyKind = metrics.keyKind
				attr.KeyBytes = metrics.keyBytes
				validated = append(validated, attr)
			}
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

func (s *Session) fixedValueMatches(
	validator runtime.ValidatorID,
	member runtime.ValidatorID,
	canonical []byte,
	metrics valueMetrics,
	resolver value.NSResolver,
	fixed runtime.ValueRef,
	fixedKey runtime.ValueKeyRef,
) (bool, error) {
	if fixedKey.Ref.Present {
		actualKind := metrics.keyKind
		actualKey := metrics.keyBytes
		if actualKind == runtime.VKInvalid {
			kind, key, err := s.keyForCanonicalValue(validator, canonical, resolver, member)
			if err != nil {
				return false, err
			}
			actualKind = kind
			actualKey = key
		}
		return actualKind == fixedKey.Kind && bytes.Equal(actualKey, valueKeyBytes(s.rt.Values, fixedKey)), nil
	}
	return bytes.Equal(canonical, valueBytes(s.rt.Values, fixed)), nil
}

func (s *Session) applyDefaultAttrs(uses []runtime.AttrUse, present []bool, storeAttrs, seenID bool) ([]AttrApplied, error) {
	applied := s.attrAppliedBuf[:0]
	if cap(applied) < len(uses) {
		applied = make([]AttrApplied, 0, len(uses))
	}

	for i := range uses {
		use := &uses[i]
		if use.Use == runtime.AttrProhibited {
			continue
		}
		if i < len(present) && present[i] {
			continue
		}
		if use.Use == runtime.AttrRequired {
			return nil, newValidationError(xsderrors.ErrRequiredAttributeMissing, "required attribute missing")
		}
		if use.Fixed.Present {
			if s.isIDValidator(use.Validator) {
				if seenID {
					return nil, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
				}
				seenID = true
			}
			fixedValue := valueBytes(s.rt.Values, use.Fixed)
			if err := s.trackDefaultValue(use.Validator, fixedValue, nil, use.FixedMember); err != nil {
				return nil, err
			}
			if storeAttrs {
				kind := use.FixedKey.Kind
				key := valueKeyBytes(s.rt.Values, use.FixedKey)
				if !use.FixedKey.Ref.Present {
					var err error
					kind, key, err = s.keyForCanonicalValue(use.Validator, fixedValue, nil, use.FixedMember)
					if err != nil {
						return nil, err
					}
				}
				applied = append(applied, AttrApplied{
					Name:     use.Name,
					Value:    use.Fixed,
					Fixed:    true,
					KeyKind:  kind,
					KeyBytes: s.storeKey(key),
				})
			} else {
				applied = append(applied, AttrApplied{Name: use.Name, Value: use.Fixed, Fixed: true})
			}
			continue
		}
		if use.Default.Present {
			if s.isIDValidator(use.Validator) {
				if seenID {
					return nil, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
				}
				seenID = true
			}
			defaultValue := valueBytes(s.rt.Values, use.Default)
			if err := s.trackDefaultValue(use.Validator, defaultValue, nil, use.DefaultMember); err != nil {
				return nil, err
			}
			if storeAttrs {
				kind := use.DefaultKey.Kind
				key := valueKeyBytes(s.rt.Values, use.DefaultKey)
				if !use.DefaultKey.Ref.Present {
					var err error
					kind, key, err = s.keyForCanonicalValue(use.Validator, defaultValue, nil, use.DefaultMember)
					if err != nil {
						return nil, err
					}
				}
				applied = append(applied, AttrApplied{
					Name:     use.Name,
					Value:    use.Default,
					KeyKind:  kind,
					KeyBytes: s.storeKey(key),
				})
			} else {
				applied = append(applied, AttrApplied{Name: use.Name, Value: use.Default})
			}
		}
	}

	return applied, nil
}
