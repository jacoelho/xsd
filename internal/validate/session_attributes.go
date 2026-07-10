package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlns"
	"github.com/jacoelho/xsd/xsderrors"
)

func (s *session) validateAttributes(typ runtime.TypeID, attrs []stream.Attr, line, col int) error {
	if len(attrs) == 0 && typ.Kind == runtime.TypeSimple {
		return nil
	}
	set, isComplex, ok := s.attributeUseSetForType(typ)
	if !isComplex {
		return s.validateSimpleTypeAttributes(attrs, line, col)
	}
	if !ok {
		return xsderrors.InternalInvariant("complex attribute use set is invalid")
	}
	if len(attrs) == 0 && set.UseCount() == 0 && set.Wildcard() == runtime.NoWildcard {
		return nil
	}
	return s.validateAttributeSet(set, attrs, line, col)
}

func (s *session) attributeUseSetForType(typ runtime.TypeID) (runtime.AttributeUseSetRead, bool, bool) {
	return s.rt.AttributeUseSetForType(typ)
}

func (s *session) attributeDecl(id runtime.AttributeID) (runtime.AttributeDeclRead, bool) {
	return s.rt.AttributeDecl(id)
}

func (s *session) validateRawSimpleValue(id runtime.SimpleTypeID, raw []byte) (bool, error) {
	return s.rt.ValidateRawSimpleValue(id, raw)
}

func (s *session) validateSimpleValue(
	id runtime.SimpleTypeID,
	lexical string,
	resolve runtime.ResolveQNameParts,
	needs runtime.SimpleValueNeed,
) (runtime.SimpleValue, error) {
	return s.rt.ValidateSimpleValue(id, lexical, resolve, needs)
}

func (s *session) validateAttributeSet(set runtime.AttributeUseSetRead, attrs []stream.Attr, line, col int) error {
	seen := newAttributeSeenWithScratch(set.UseCount(), &s.attributeSeen)
	seenIDAttr := false
	ctx := s.startContext(line, col)
	for i := range attrs {
		a := &attrs[i]
		if a.Name.Space != "" || a.Name.Local == vocab.XMLNSPrefix {
			if xmlns.IsNamespaceName(a.Name) {
				continue
			}
			if isXSIAttributeName(a.Name) {
				if err := s.validateXSIAttribute(a.Name, a.StringValue(&s.valueStrings), line, col); err != nil {
					return err
				}
				continue
			}
		}
		rn := s.runtimeName(a.Name)
		if rn.Known {
			if use, slot, ok := set.DeclaredUse(rn.Name); ok {
				if !seen.mark(slot) {
					if err := s.recover(attributeValidation(ctx, "duplicate attribute "+rn.Label())); err != nil {
						return err
					}
					continue
				}
				if err := s.validateDeclaredAttributeUse(use, rn, a, ctx, line, col, &seenIDAttr); err != nil {
					if recoverErr := s.recover(err); recoverErr != nil {
						return recoverErr
					}
				}
				continue
			}
		}
		handled, err := s.validateWildcardAttribute(set, rn, a, ctx, line, col, &seenIDAttr)
		if err != nil {
			if recoverErr := s.recover(err); recoverErr != nil {
				return recoverErr
			}
			continue
		}
		if handled {
			continue
		}
		if err := s.recover(attributeValidation(ctx, "attribute is not declared: "+rn.Label())); err != nil {
			return err
		}
	}
	return s.validateRequiredAndDefaultAttributes(set, seen, ctx, line, col, &seenIDAttr)
}

func (s *session) validateSimpleTypeAttributes(attrs []stream.Attr, line, col int) error {
	if len(attrs) == 0 {
		return nil
	}
	ctx := s.startContext(line, col)
	for i := range attrs {
		a := &attrs[i]
		if a.Name.Space != "" || a.Name.Local == vocab.XMLNSPrefix {
			if xmlns.IsNamespaceName(a.Name) {
				continue
			}
			if isXSIAttributeName(a.Name) {
				if err := s.validateXSIAttribute(a.Name, a.StringValue(&s.valueStrings), line, col); err != nil {
					return err
				}
				continue
			}
		}
		if err := s.recover(attributeValidation(ctx, "simple type does not allow attributes")); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) validateDeclaredAttributeUse(
	use runtime.AttributeUseRead,
	rn runtime.RuntimeName,
	attr *stream.Attr,
	ctx StartContext,
	line, col int,
	seenIDAttr *bool,
) error {
	var identityFields []IdentityFieldMatch
	if s.hasIdentityConstraints {
		var err error
		identityFields, err = s.identityAttributeFields(use.Name())
		if err != nil {
			return err
		}
	}
	var needs runtime.SimpleValueNeed
	fixed, hasFixed := use.FixedValue()
	if hasFixed {
		needs |= runtime.SimpleNeedCanonical
	}
	if len(identityFields) != 0 {
		needs |= runtime.SimpleNeedIdentity
	}
	if len(identityFields) == 0 && hasFixed && use.CanValidateFixedStringFast() {
		if attr.StringValue(&s.valueStrings) != fixed.CanonicalText() {
			return attributeValidation(ctx, "fixed attribute mismatch "+rn.Label())
		}
		return nil
	}
	if len(identityFields) == 0 && !hasFixed {
		if raw, ok := attr.RawValue(); ok {
			handled, rawErr := s.validateRawSimpleValue(use.TypeID(), raw)
			if rawErr != nil {
				if invariantErr := simpleValueMetadataInvariant(rawErr); invariantErr != nil {
					return invariantErr
				}
				if handled {
					return validation(ctx, xsderrors.CodeValidationFacet, "invalid attribute "+rn.Label()+": "+rawErr.Error())
				}
				return rawErr
			}
			if handled {
				return nil
			}
		}
	}
	typeID := use.TypeID()
	value, err := s.validateSimpleValue(typeID, attr.StringValue(&s.valueStrings), s.simpleValueQNameResolver(typeID), needs)
	if err != nil {
		if invariantErr := simpleValueMetadataInvariant(err); invariantErr != nil {
			return invariantErr
		}
		if xsderrors.IsUnsupported(err) {
			return err
		}
		return validation(ctx, xsderrors.CodeValidationFacet, "invalid attribute "+rn.Label()+": "+err.Error())
	}
	if err := s.recordAttributeIdentity(value, line, col, seenIDAttr); err != nil {
		return err
	}
	if len(identityFields) != 0 {
		if err := s.captureSimpleValueIdentityFields(identityFields, value, ctx); err != nil {
			return err
		}
	}
	if hasFixed && value.CanonicalText() != fixed.CanonicalText() {
		return attributeValidation(ctx, "fixed attribute mismatch "+rn.Label())
	}
	return nil
}

func (s *session) validateWildcardAttribute(
	set runtime.AttributeUseSetRead,
	rn runtime.RuntimeName,
	attr *stream.Attr,
	ctx StartContext,
	line, col int,
	seenIDAttr *bool,
) (bool, error) {
	match, valid := MatchAttributeWildcard(s.rt, set.Wildcard(), rn)
	if !valid {
		return true, xsderrors.InternalInvariant("attribute wildcard state is invalid")
	}
	if !match.Matched {
		return false, nil
	}
	if match.Skip {
		return true, nil
	}
	if match.HasAttribute {
		decl, ok := s.attributeDecl(match.Attribute)
		if !ok {
			return true, xsderrors.InternalInvariant("attribute wildcard matched invalid declaration")
		}
		return true, s.validateKnownWildcardAttribute(decl, rn, attr.StringValue(&s.valueStrings), ctx, line, col, seenIDAttr)
	}
	if match.LaxMissing {
		return true, nil
	}
	if s.hasSchemaLocationHint(rn.NS) {
		return true, unsupportedSchemaLocation(ctx, vocab.XSDElemAttribute, rn)
	}
	return false, nil
}

func (s *session) validateKnownWildcardAttribute(
	decl runtime.AttributeDeclRead,
	rn runtime.RuntimeName,
	lexical string,
	ctx StartContext,
	line, col int,
	seenIDAttr *bool,
) error {
	var identityFields []IdentityFieldMatch
	if s.hasIdentityConstraints {
		var err error
		identityFields, err = s.identityAttributeFields(decl.Name())
		if err != nil {
			return err
		}
	}
	needs := runtime.SimpleNeedCanonical
	if len(identityFields) != 0 {
		needs |= runtime.SimpleNeedIdentity
	}
	typeID := decl.TypeID()
	value, err := s.validateSimpleValue(typeID, lexical, s.simpleValueQNameResolver(typeID), needs)
	if err != nil {
		if invariantErr := simpleValueMetadataInvariant(err); invariantErr != nil {
			return invariantErr
		}
		if xsderrors.IsUnsupported(err) {
			return err
		}
		return validation(ctx, xsderrors.CodeValidationFacet, "invalid wildcard attribute "+rn.Label())
	}
	if err := s.recordAttributeIdentity(value, line, col, seenIDAttr); err != nil {
		return err
	}
	if fixed, ok := decl.FixedValue(); ok && value.CanonicalText() != fixed.CanonicalText() {
		return attributeValidation(ctx, "fixed attribute mismatch "+rn.Label())
	}
	if len(identityFields) == 0 {
		return nil
	}
	return s.captureSimpleValueIdentityFields(identityFields, value, ctx)
}

func (s *session) validateRequiredAndDefaultAttributes(
	set runtime.AttributeUseSetRead,
	seen AttributeSeen,
	ctx StartContext,
	line, col int,
	seenIDAttr *bool,
) error {
	required := set.RequiredSlots()
	for slotIndex := range required.Len() {
		slot, ok := required.At(slotIndex)
		if !ok {
			return xsderrors.InternalInvariant("required attribute slot is invalid")
		}
		if seen.has(int(slot)) {
			continue
		}
		use, ok := set.UseAt(int(slot))
		if !ok {
			return xsderrors.InternalInvariant("required attribute slot is invalid")
		}
		if err := s.recover(attributeValidation(ctx, "missing required attribute "+use.Label())); err != nil {
			return err
		}
	}
	valueConstraints := set.ValueConstraintSlots()
	for slotIndex := range valueConstraints.Len() {
		slot, ok := valueConstraints.At(slotIndex)
		if !ok {
			return xsderrors.InternalInvariant("value constraint attribute slot is invalid")
		}
		if seen.has(int(slot)) {
			continue
		}
		use, ok := set.UseAt(int(slot))
		if !ok {
			return xsderrors.InternalInvariant("value constraint attribute slot is invalid")
		}
		if use.Required() {
			continue
		}
		vc, ok := use.AbsentValueConstraint()
		if !ok {
			continue
		}
		value := vc.SimpleValue()
		if err := s.recordAttributeIdentity(value, line, col, seenIDAttr); err != nil {
			if recoverErr := s.recover(err); recoverErr != nil {
				return recoverErr
			}
			continue
		}
		if s.hasIdentityConstraints {
			fields, err := s.identityAttributeFields(use.Name())
			if err != nil {
				if recoverErr := s.recover(err); recoverErr != nil {
					return recoverErr
				}
				continue
			}
			if len(fields) != 0 {
				if err := s.captureSimpleValueIdentityFields(fields, value, ctx); err != nil {
					if recoverErr := s.recover(err); recoverErr != nil {
						return recoverErr
					}
				}
			}
		}
	}
	return nil
}

func (s *session) validateXSIAttribute(name xml.Name, value string, line, col int) error {
	if !s.hasIdentityConstraints {
		return nil
	}
	if err := s.captureIdentityXSIAttribute(name, value, line, col); err != nil {
		return s.recover(err)
	}
	return nil
}

func (s *session) recordSchemaLocationHints(attrs []stream.Attr, line, col int) error {
	return s.doc.schemaLocationHints.RecordAttributes(attrs, &s.valueStrings, s.startContext(line, col))
}

func (s *session) hasSchemaLocationHint(ns string) bool {
	return s.doc.schemaLocationHints.Has(ns)
}

func (s *session) schemaLocationHintLookup() HasSchemaLocation {
	if s.doc.schemaLocationHints.namespaces == nil {
		return nil
	}
	return s.hasSchemaLocationHint
}
