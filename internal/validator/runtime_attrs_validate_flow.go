package validator

import (
	"bytes"
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// AttrResult holds validated input attributes and applied default/fixed attributes.
type AttrResult struct {
	Applied []Applied
	Attrs   []Start
}

// ValidateAttributes validates attributes against the resolved runtime type.
func (s *Session) ValidateAttributes(typeID runtime.TypeID, inputAttrs []Start, resolver value.NSResolver) (AttrResult, error) {
	if s == nil || s.rt == nil {
		return AttrResult{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}
	classified, err := s.classifyAttrs(inputAttrs, true)
	if err != nil {
		return AttrResult{}, err
	}
	return s.validateAttributesClassified(typeID, inputAttrs, resolver, classified)
}

func (s *Session) validateAttributesClassified(typeID runtime.TypeID, inputAttrs []Start, resolver value.NSResolver, classified Classification) (AttrResult, error) {
	return s.validateAttributesClassifiedWithStorage(typeID, inputAttrs, resolver, classified, s.hasIdentityConstraints(), true)
}

func (s *Session) validateAttributesClassifiedWithStorage(typeID runtime.TypeID, inputAttrs []Start, resolver value.NSResolver, classified Classification, storeAttrs, storeValues bool) (AttrResult, error) {
	if classified.DuplicateErr != nil {
		return AttrResult{}, classified.DuplicateErr
	}

	typ, ok := s.typeByID(typeID)
	if !ok {
		return AttrResult{}, fmt.Errorf("type %d not found", typeID)
	}

	validated := s.attrState.PrepareValidated(storeAttrs, len(inputAttrs))
	var (
		applied []Applied
		err     error
	)
	if typ.Kind != runtime.TypeComplex {
		validated, err = s.validateSimpleAttrs(inputAttrs, classified.Classes, storeAttrs, storeValues, validated)
	} else {
		if int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
			return AttrResult{}, fmt.Errorf("complex type %d not found", typ.Complex.ID)
		}
		ct := &s.rt.ComplexTypes[typ.Complex.ID]
		uses := Uses(s.rt.AttrIndex.Uses, ct.Attrs)
		present := s.attrState.PreparePresent(len(uses))

		seenID := false
		validated, seenID, err = s.validateComplexAttrsClassified(ct, present, inputAttrs, classified.Classes, resolver, storeAttrs, storeValues, validated)
		if err == nil {
			applied, err = s.applyDefaultAttrs(uses, present, storeAttrs, seenID)
		}
	}
	if err != nil {
		return AttrResult{}, err
	}

	if storeAttrs {
		s.attrState.Validated = validated[:0]
	}
	if cap(applied) > 0 {
		s.attrAppliedBuf = applied[:0]
	}
	return AttrResult{Attrs: validated, Applied: applied}, nil
}

func (s *Session) classifyAttrs(input []Start, checkDuplicates bool) (Classification, error) {
	if s == nil {
		return Classification{}, nil
	}
	return s.attrState.Classify(s.rt, input, checkDuplicates)
}

func (s *Session) validateSimpleAttrs(input []Start, classes []Class, storeAttrs, storeValues bool, validated []Start) ([]Start, error) {
	for i, attr := range input {
		switch classifyFor(s.rt, classes, i, attr) {
		case ClassXSIUnknown:
			return nil, xsderrors.New(xsderrors.ErrValidateSimpleTypeAttrNotAllowed, "unknown xsi attribute")
		case ClassXSIKnown, ClassXML:
			continue
		default:
			return nil, xsderrors.New(xsderrors.ErrValidateSimpleTypeAttrNotAllowed, "attribute not allowed on simple type")
		}
	}
	if !storeAttrs {
		return nil, nil
	}
	for _, attr := range input {
		if storeValues {
			validated = StoreRaw(validated, attr, true, s.ensureAttrNameStable, s.storeValue)
			continue
		}
		validated = StoreRawIdentity(validated, attr, true, s.ensureAttrNameStable)
	}
	return validated, nil
}

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

func (s *Session) validateComplexAttrUse(
	validated []Start,
	attr Start,
	resolver value.NSResolver,
	storeAttrs bool,
	storeValues bool,
	use runtime.AttrUse,
	seenID *bool,
) ([]Start, error) {
	return s.validateComplexAttrValue(validated, attr, resolver, storeAttrs, storeValues, SpecFromUse(use), seenID)
}

func (s *Session) validateComplexAttrValue(
	validated []Start,
	attr Start,
	resolver value.NSResolver,
	storeAttrs bool,
	storeValues bool,
	spec ValueSpec,
	seenID *bool,
) ([]Start, error) {
	opts := valueOptions{
		ApplyWhitespace:  true,
		TrackIDs:         true,
		RequireCanonical: spec.Fixed.Present,
		StoreValue:       storeAttrs && storeValues,
		NeedKey:          spec.Fixed.Present || storeAttrs,
	}
	var metrics *ValueMetrics
	if storeAttrs || spec.Fixed.Present {
		metrics = s.acquireMetricsState()
		defer s.releaseMetricsState()
	}
	canonical, err := s.validateValueCore(spec.Validator, attr.Value, resolver, opts, metrics)
	if err != nil {
		return nil, err
	}

	if s.isIDValidator(spec.Validator) {
		if *seenID {
			return nil, xsderrors.New(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
		}
		*seenID = true
	}

	keyKind := runtime.VKInvalid
	var keyBytes []byte
	hasKey := false
	if metrics != nil {
		keyKind, keyBytes, hasKey = metrics.State.Key()
	}
	if storeAttrs {
		if storeValues {
			validated = StoreCanonical(validated, attr, true, s.ensureAttrNameStable, canonical, keyKind, keyBytes)
		} else {
			if len(keyBytes) > 0 {
				keyBytes = s.storeKey(keyBytes)
			}
			validated = StoreCanonicalIdentity(validated, attr, true, s.ensureAttrNameStable, keyKind, keyBytes)
		}
	}
	if !spec.Fixed.Present {
		return validated, nil
	}

	if spec.FixedKey.Ref.Present {
		actualKind := keyKind
		actualKey := keyBytes
		if !hasKey {
			actualKind, actualKey, err = s.keyForCanonicalValue(spec.Validator, canonical, resolver, spec.FixedMember)
			if err != nil {
				return nil, err
			}
		}
		if actualKind != spec.FixedKey.Kind || !bytes.Equal(actualKey, valueBytes(s.rt.Values, spec.FixedKey.Ref)) {
			return nil, xsderrors.New(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
		}
		return validated, nil
	}

	if !bytes.Equal(canonical, valueBytes(s.rt.Values, spec.Fixed)) {
		return nil, xsderrors.New(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
	}
	return validated, nil
}

func (s *Session) validateComplexWildcardAttr(
	validated []Start,
	attr Start,
	resolver value.NSResolver,
	storeAttrs bool,
	storeValues bool,
	anyAttr runtime.WildcardID,
	seenID *bool,
) ([]Start, error) {
	if !s.rt.WildcardAccepts(anyAttr, attr.NSBytes, attr.NS) {
		return nil, newValidationError(xsderrors.ErrAttributeNotDeclared, "attribute wildcard rejected namespace")
	}

	rule := s.rt.Wildcards[anyAttr]
	wildcardAttr, resolved, err := s.resolveWildcardAttrID(rule.PC, attr.Sym)
	if err != nil {
		return nil, err
	}
	if !resolved {
		if storeValues {
			return StoreRaw(validated, attr, storeAttrs, s.ensureAttrNameStable, s.storeValue), nil
		}
		return StoreRawIdentity(validated, attr, storeAttrs, s.ensureAttrNameStable), nil
	}
	if int(wildcardAttr) >= len(s.rt.Attributes) {
		return nil, fmt.Errorf("attribute %d out of range", wildcardAttr)
	}

	globalAttr := s.rt.Attributes[wildcardAttr]
	return s.validateComplexAttrValue(validated, attr, resolver, storeAttrs, storeValues, SpecFromAttribute(globalAttr), seenID)
}

func (s *Session) resolveWildcardAttrID(pc runtime.ProcessContents, sym runtime.SymbolID) (runtime.AttrID, bool, error) {
	var wildcardAttr runtime.AttrID
	resolved, err := ResolveStartSymbol(pc, sym, func(symbol runtime.SymbolID) bool {
		id, ok := GlobalAttributeBySymbol(s.rt, symbol)
		if !ok {
			return false
		}
		wildcardAttr = id
		return true
	}, func() error {
		return newValidationError(xsderrors.ErrValidateWildcardAttrStrictUnresolved, "attribute wildcard strict unresolved")
	})
	if err != nil {
		return 0, false, err
	}
	if !resolved {
		return 0, false, nil
	}
	return wildcardAttr, true, nil
}

func (s *Session) applyDefaultAttrs(uses []runtime.AttrUse, present []bool, storeAttrs, seenID bool) ([]Applied, error) {
	readValue := func(ref runtime.ValueRef) []byte { return valueBytes(s.rt.Values, ref) }
	materializeKey := func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID, stored runtime.ValueKeyRef) (runtime.ValueKind, []byte, error) {
		return materializeValueKey(
			validator,
			canonical,
			member,
			stored,
			readValue,
			func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
				return s.keyForCanonicalValue(validator, canonical, nil, member)
			},
		)
	}
	return ApplyDefaults(
		uses,
		present,
		storeAttrs,
		seenID,
		s.attrAppliedBuf[:0],
		SelectDefaultOrFixed,
		s.isIDValidator,
		readValue,
		func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID) error {
			return s.trackDefaultValue(validator, canonical, nil, member)
		},
		materializeKey,
		s.storeKey,
	)
}

func (s *Session) ensureAttrNameStable(attr *Start) {
	if s == nil || attr == nil || attr.NameCached {
		return
	}
	attr.Local, attr.NSBytes, attr.NameCached = s.Names.Stabilize(attr.Local, attr.NSBytes, attr.NameCached)
}

func (s *Session) isIDValidator(id runtime.ValidatorID) bool {
	meta, ok, err := s.validatorMetaIfPresent(id)
	if err != nil || !ok {
		return false
	}
	if meta.Kind != runtime.VString {
		return false
	}
	kind, ok := s.stringKind(meta)
	if !ok {
		return false
	}
	return kind == runtime.StringID
}

func (s *Session) needsIdentityAttrs(elemID runtime.ElemID) bool {
	if s == nil || s.rt == nil {
		return false
	}
	if s.icState.Scopes.Len() > 0 {
		return true
	}
	elem, ok := s.element(elemID)
	return ok && elem.ICLen > 0
}
