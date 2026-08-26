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
	if len(attrs) == 0 && typ.IsSimple() {
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
	return s.rt.ValidateRawSimpleValueWithScratch(id, raw, &s.stringPatternScratch)
}

func (s *session) validateSimpleValue(
	id runtime.SimpleTypeID,
	lexical string,
	resolve runtime.ResolveQNameParts,
	needs runtime.SimpleValueNeed,
) (runtime.SimpleValue, error) {
	return s.rt.ValidateSimpleValueWithScratch(id, lexical, resolve, needs, &s.stringPatternScratch)
}

func (s *session) validateAttributeSet(set runtime.AttributeUseSetRead, attrs []stream.Attr, line, col int) error {
	seen := newAttributeSeenWithScratch(set.UseCount(), &s.attributeSeen)
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
					if err := s.recoverAssessment(attributeValidation(ctx, "duplicate attribute "+rn.Label())); err != nil {
						return err
					}
					continue
				}
				if err := s.validateDeclaredAttributeUse(use, rn, a, ctx); err != nil {
					if recoverErr := s.recoverAssessment(err); recoverErr != nil {
						return recoverErr
					}
				}
				continue
			}
		}
		handled, err := s.validateWildcardAttribute(set, rn, a, ctx)
		if err != nil {
			if recoverErr := s.recoverUnassessedIdentityAttribute(rn, ctx, err); recoverErr != nil {
				return recoverErr
			}
			continue
		}
		if handled {
			continue
		}
		if err := s.recoverUnassessedIdentityAttribute(rn, ctx, attributeValidation(ctx, "attribute is not declared: "+rn.Label())); err != nil {
			return err
		}
	}
	return s.validateRequiredAndDefaultAttributes(set, seen, ctx)
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
		rn := s.runtimeName(a.Name)
		if err := s.recoverUnassessedIdentityAttribute(rn, ctx, attributeValidation(ctx, "simple type does not allow attributes")); err != nil {
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
) error {
	identityTarget, targetErr := s.doc.identity.prepareAttributeValue(rn)
	if targetErr != nil {
		return targetErr
	}
	if err := s.validateAttributeTypeAvailable(use.TypeID(), rn.Label(), ctx); err != nil {
		if invalidateErr := s.doc.identity.rejectValue(identityTarget, identityInvalidValue, ctx); invalidateErr != nil {
			return invalidateErr
		}
		return err
	}
	var needs runtime.SimpleValueNeed
	fixed, hasFixed := use.FixedValue()
	if hasFixed {
		needs |= runtime.SimpleNeedCanonical
	}
	if identityTarget.needsIdentity() || hasFixed && use.FixedUsesValueSpace() {
		needs |= runtime.SimpleNeedIdentity
	}
	if !identityTarget.needsIdentity() && hasFixed && use.CanValidateFixedStringFast() {
		if attr.StringValue(&s.valueStrings) != fixed.CanonicalText() {
			return attributeValidation(ctx, "fixed attribute mismatch "+rn.Label())
		}
		return nil
	}
	if !identityTarget.needsIdentity() && !hasFixed {
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
		if invalidateErr := s.doc.identity.rejectValue(identityTarget, identityInvalidValue, ctx); invalidateErr != nil {
			return invalidateErr
		}
		if invariantErr := simpleValueMetadataInvariant(err); invariantErr != nil {
			return invariantErr
		}
		if xsderrors.IsUnsupported(err) {
			return err
		}
		return validation(ctx, xsderrors.CodeValidationFacet, "invalid attribute "+rn.Label()+": "+err.Error())
	}
	if err := s.doc.identity.recordValue(identityTarget, value, ctx); err != nil {
		return err
	}
	if err := s.doc.identity.captureValue(identityTarget, ctx); err != nil {
		return err
	}
	if hasFixed {
		if err := s.validateFixedAttributeValue(value, fixed, use.FixedUsesValueSpace(), identityTarget, ctx, rn.Label()); err != nil {
			return err
		}
	}
	return s.doc.identity.commitValue(identityTarget)
}

func (s *session) validateWildcardAttribute(
	set runtime.AttributeUseSetRead,
	rn runtime.RuntimeName,
	attr *stream.Attr,
	ctx StartContext,
) (bool, error) {
	match, valid := MatchAttributeWildcard(s.rt, set.Wildcard(), rn)
	if !valid {
		return true, xsderrors.InternalInvariant("attribute wildcard state is invalid")
	}
	if !match.Matched {
		return false, nil
	}
	if match.Skip {
		return true, s.rejectUnassessedIdentityAttribute(rn, ctx, true)
	}
	if match.HasAttribute {
		decl, ok := s.attributeDecl(match.Attribute)
		if !ok {
			return true, xsderrors.InternalInvariant("attribute wildcard matched invalid declaration")
		}
		return true, s.validateKnownWildcardAttribute(decl, rn, attr.StringValue(&s.valueStrings), ctx)
	}
	if match.LaxMissing {
		return true, s.rejectUnassessedIdentityAttribute(rn, ctx, true)
	}
	if s.hasSchemaLocationHint(rn.NS) {
		return true, unsupportedSchemaLocation(ctx, vocab.XSDElemAttribute, rn)
	}
	return false, nil
}

func (s *session) rejectUnassessedIdentityAttributes(attrs []stream.Attr, line, col int, report bool) error {
	ctx := s.startContext(line, col)
	for i := range attrs {
		if xmlns.IsNamespaceName(attrs[i].Name) {
			continue
		}
		rn := s.runtimeName(attrs[i].Name)
		err := s.rejectUnassessedIdentityAttribute(rn, ctx, report)
		if err == nil {
			continue
		}
		if !report {
			return err
		}
		if recoverErr := s.recoverAssessment(err); recoverErr != nil {
			return recoverErr
		}
	}
	return nil
}

func (s *session) rejectUnassessedIdentityAttribute(rn runtime.RuntimeName, ctx StartContext, report bool) error {
	target, err := s.doc.identity.prepareAttributeValue(rn)
	if err != nil {
		return err
	}
	if report {
		return s.doc.identity.rejectValue(target, identityMissingSimpleValue, ctx)
	}
	return s.doc.identity.rejectValue(target, identityInvalidValue, ctx)
}

func (s *session) recoverUnassessedIdentityAttribute(rn runtime.RuntimeName, ctx StartContext, err error) error {
	if invalidateErr := s.rejectUnassessedIdentityAttribute(rn, ctx, false); invalidateErr != nil {
		return invalidateErr
	}
	return s.recoverAssessment(err)
}

func (s *session) validateKnownWildcardAttribute(
	decl runtime.AttributeDeclRead,
	rn runtime.RuntimeName,
	lexical string,
	ctx StartContext,
) error {
	identityTarget, targetErr := s.doc.identity.prepareAttributeValue(rn)
	if targetErr != nil {
		return targetErr
	}
	if err := s.validateAttributeTypeAvailable(decl.TypeID(), rn.Label(), ctx); err != nil {
		if invalidateErr := s.doc.identity.rejectValue(identityTarget, identityInvalidValue, ctx); invalidateErr != nil {
			return invalidateErr
		}
		return err
	}
	fixed, hasFixed := decl.FixedValue()
	needs := runtime.SimpleNeedCanonical
	if identityTarget.needsIdentity() || hasFixed {
		needs |= runtime.SimpleNeedIdentity
	}
	typeID := decl.TypeID()
	value, err := s.validateSimpleValue(typeID, lexical, s.simpleValueQNameResolver(typeID), needs)
	if err != nil {
		if invalidateErr := s.doc.identity.rejectValue(identityTarget, identityInvalidValue, ctx); invalidateErr != nil {
			return invalidateErr
		}
		if invariantErr := simpleValueMetadataInvariant(err); invariantErr != nil {
			return invariantErr
		}
		if xsderrors.IsUnsupported(err) {
			return err
		}
		return validation(ctx, xsderrors.CodeValidationFacet, "invalid wildcard attribute "+rn.Label())
	}
	if err := s.doc.identity.recordValue(identityTarget, value, ctx); err != nil {
		return err
	}
	if hasFixed {
		if err := s.validateFixedAttributeValue(value, fixed, true, identityTarget, ctx, rn.Label()); err != nil {
			return err
		}
	}
	if err := s.doc.identity.captureValue(identityTarget, ctx); err != nil {
		return err
	}
	return s.doc.identity.commitValue(identityTarget)
}

func (s *session) validateFixedAttributeValue(
	value runtime.SimpleValue,
	fixed runtime.ValueConstraintRead,
	valueSpace bool,
	identityTarget identityValueTarget,
	ctx StartContext,
	label string,
) error {
	equal, valid := runtime.FixedAttributeValueEqual(value, fixed, valueSpace)
	if equal {
		return nil
	}
	if invalidateErr := s.doc.identity.rejectValue(identityTarget, identityInvalidValue, ctx); invalidateErr != nil {
		return invalidateErr
	}
	if !valid {
		return xsderrors.InternalInvariant("fixed attribute value-space identity is missing")
	}
	return attributeValidation(ctx, "fixed attribute mismatch "+label)
}

func (s *session) validateAttributeTypeAvailable(id runtime.SimpleTypeID, label string, ctx StartContext) error {
	unavailable, ok := s.rt.SimpleTypeUnavailable(id)
	if !ok {
		return xsderrors.InternalInvariant("attribute type metadata is invalid")
	}
	if unavailable {
		return attributeValidation(ctx, "attribute type is unavailable: "+label)
	}
	return nil
}

func (s *session) validateRequiredAndDefaultAttributes(
	set runtime.AttributeUseSetRead,
	seen AttributeSeen,
	ctx StartContext,
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
		if err := s.recoverAssessment(attributeValidation(ctx, "missing required attribute "+use.Label())); err != nil {
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
		identityTarget, err := s.doc.identity.prepareAttributeValue(knownIdentityAttributeName(use.Name()))
		if err != nil {
			if recoverErr := s.recoverAssessment(err); recoverErr != nil {
				return recoverErr
			}
			continue
		}
		if err := s.doc.identity.recordValue(identityTarget, value, ctx); err != nil {
			if recoverErr := s.recoverAssessment(err); recoverErr != nil {
				return recoverErr
			}
			continue
		}
		if err := s.doc.identity.captureValue(identityTarget, ctx); err != nil {
			if recoverErr := s.recoverAssessment(err); recoverErr != nil {
				return recoverErr
			}
			continue
		}
		if err := s.doc.identity.commitValue(identityTarget); err != nil {
			if recoverErr := s.recoverAssessment(err); recoverErr != nil {
				return recoverErr
			}
		}
	}
	return nil
}

func (s *session) validateXSIAttribute(name xml.Name, value string, line, col int) error {
	rn := ResolveRuntimeName(s.rt, name)
	target, err := s.doc.identity.prepareAttributeValue(rn)
	if err != nil {
		return err
	}
	if err := s.doc.identity.captureXSIAttribute(
		target,
		name,
		value,
		s.qnameResolver(),
		s.startContext(line, col),
	); err != nil {
		return s.recover(err)
	}
	return nil
}

func (s *session) recordSchemaLocationHints(attrs []stream.Attr, line, col int) error {
	return s.doc.schemaLocationHints.RecordAttributes(
		attrs,
		&s.valueStrings,
		schemaLocationHintLimits{
			Namespaces:     s.maxSchemaLocationNamespaces,
			NamespaceBytes: s.maxSchemaLocationNamespaceBytes,
		},
		s.startContext(line, col),
	)
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
