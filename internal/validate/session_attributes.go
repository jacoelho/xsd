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
		if err := s.validateAttribute(set, &seen, &attrs[i], line, col, ctx); err != nil {
			return err
		}
	}
	return s.validateRequiredAndDefaultAttributes(set, seen, ctx)
}

func (s *session) validateAttribute(set runtime.AttributeUseSetRead, seen *AttributeSeen, attr *stream.Attr, line, col int, ctx StartContext) error {
	handled, err := s.validateReservedAttribute(attr, line, col)
	if err != nil || handled {
		return err
	}
	rn := s.runtimeName(attr.Name)
	handled, err = s.validateDeclaredAttribute(set, seen, rn, attr, ctx)
	if err != nil || handled {
		return err
	}
	return s.validateUndeclaredAttribute(set, rn, attr, ctx)
}

func (s *session) validateDeclaredAttribute(set runtime.AttributeUseSetRead, seen *AttributeSeen, rn runtime.RuntimeName, attr *stream.Attr, ctx StartContext) (bool, error) {
	if !rn.Known {
		return false, nil
	}
	use, slot, ok := set.DeclaredUse(rn.Name)
	if !ok {
		return false, nil
	}
	if !seen.mark(slot) {
		return true, s.recoverAssessment(attributeValidation(ctx, "duplicate attribute "+rn.Label()))
	}
	return true, s.recoverAssessment(s.validateDeclaredAttributeUse(use, rn, attr, ctx))
}

func (s *session) validateUndeclaredAttribute(set runtime.AttributeUseSetRead, rn runtime.RuntimeName, attr *stream.Attr, ctx StartContext) error {
	handled, err := s.validateWildcardAttribute(set, rn, attr, ctx)
	if err != nil {
		return s.recoverUnassessedIdentityAttribute(rn, ctx, err)
	}
	if handled {
		return nil
	}
	return s.recoverUnassessedIdentityAttribute(rn, ctx, attributeValidation(ctx, "attribute is not declared: "+rn.Label()))
}

func (s *session) validateSimpleTypeAttributes(attrs []stream.Attr, line, col int) error {
	if len(attrs) == 0 {
		return nil
	}
	ctx := s.startContext(line, col)
	for i := range attrs {
		a := &attrs[i]
		if handled, err := s.validateReservedAttribute(a, line, col); err != nil {
			return err
		} else if handled {
			continue
		}
		rn := s.runtimeName(a.Name)
		if err := s.recoverUnassessedIdentityAttribute(rn, ctx, attributeValidation(ctx, "simple type does not allow attributes")); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) validateReservedAttribute(attr *stream.Attr, line, col int) (bool, error) {
	name := attr.Name
	if name.Space == "" && name.Local != vocab.XMLNSPrefix {
		return false, nil
	}
	if xmlns.IsNamespaceName(name) {
		return true, nil
	}
	if !isXSIAttributeName(name) {
		return false, nil
	}
	return true, s.validateXSIAttribute(name, attr.StringValue(&s.valueStrings), line, col)
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
		return s.rejectAttributeIdentityValue(identityTarget, ctx, err)
	}
	plan := newDeclaredAttributePlan(use, identityTarget)
	if handled, err := s.validateDeclaredAttributeFast(plan, attr, rn, ctx); handled {
		return err
	}
	return s.validateDeclaredAttributeValue(plan, attr.StringValue(&s.valueStrings), rn, ctx)
}

type declaredAttributePlan struct {
	fixed    runtime.ValueConstraintRead
	use      runtime.AttributeUseRead
	target   identityValueTarget
	needs    runtime.SimpleValueNeed
	hasFixed bool
}

func newDeclaredAttributePlan(use runtime.AttributeUseRead, target identityValueTarget) declaredAttributePlan {
	fixed, hasFixed := use.FixedValue()
	var needs runtime.SimpleValueNeed
	if hasFixed {
		needs |= runtime.SimpleNeedCanonical
	}
	if target.needsIdentity() || hasFixed && use.FixedUsesValueSpace() {
		needs |= runtime.SimpleNeedIdentity
	}
	return declaredAttributePlan{use: use, target: target, fixed: fixed, needs: needs, hasFixed: hasFixed}
}

func (p declaredAttributePlan) canValidateFixedString() bool {
	return !p.target.needsIdentity() && p.hasFixed && p.use.CanValidateFixedStringFast()
}

func (p declaredAttributePlan) canValidateRaw() bool {
	return !p.target.needsIdentity() && !p.hasFixed
}

func (s *session) validateDeclaredAttributeFast(plan declaredAttributePlan, attr *stream.Attr, rn runtime.RuntimeName, ctx StartContext) (bool, error) {
	if plan.canValidateFixedString() {
		return true, validateFixedAttributeString(attr.StringValue(&s.valueStrings), plan.fixed, rn, ctx)
	}
	if !plan.canValidateRaw() {
		return false, nil
	}
	if raw, ok := attr.RawValue(); ok {
		handled, err := s.validateRawSimpleValue(plan.use.TypeID(), raw)
		return handled || err != nil, declaredRawAttributeResult(handled, err, rn, ctx)
	}
	return false, nil
}

func validateFixedAttributeString(value string, fixed runtime.ValueConstraintRead, rn runtime.RuntimeName, ctx StartContext) error {
	if value != fixed.CanonicalText() {
		return attributeValidation(ctx, "fixed attribute mismatch "+rn.Label())
	}
	return nil
}

func declaredRawAttributeResult(handled bool, err error, rn runtime.RuntimeName, ctx StartContext) error {
	if err == nil {
		return nil
	}
	if invariantErr := simpleValueMetadataInvariant(err); invariantErr != nil {
		return invariantErr
	}
	if handled {
		return validation(ctx, xsderrors.CodeValidationFacet, "invalid attribute "+rn.Label()+": "+err.Error())
	}
	return err
}

func (s *session) validateDeclaredAttributeValue(plan declaredAttributePlan, lexical string, rn runtime.RuntimeName, ctx StartContext) error {
	typeID := plan.use.TypeID()
	value, err := s.validateSimpleValue(typeID, lexical, s.simpleValueQNameResolver(typeID), plan.needs)
	if err != nil {
		return s.declaredAttributeValueError(plan.target, rn, ctx, err)
	}
	if err := s.doc.identity.recordValue(plan.target, value, ctx); err != nil {
		return err
	}
	if err := s.doc.identity.captureValue(plan.target, ctx); err != nil {
		return err
	}
	if plan.hasFixed {
		if err := s.validateFixedAttributeValue(value, plan.fixed, plan.use.FixedUsesValueSpace(), plan.target, ctx, rn.Label()); err != nil {
			return err
		}
	}
	return s.doc.identity.commitValue(plan.target)
}

func (s *session) declaredAttributeValueError(target identityValueTarget, rn runtime.RuntimeName, ctx StartContext, err error) error {
	if rejectErr := s.doc.identity.rejectValue(target, identityInvalidValue, ctx); rejectErr != nil {
		return rejectErr
	}
	if invariantErr := simpleValueMetadataInvariant(err); invariantErr != nil {
		return invariantErr
	}
	if xsderrors.IsUnsupported(err) {
		return err
	}
	return validation(ctx, xsderrors.CodeValidationFacet, "invalid attribute "+rn.Label()+": "+err.Error())
}

func (s *session) rejectAttributeIdentityValue(target identityValueTarget, ctx StartContext, reason error) error {
	if err := s.doc.identity.rejectValue(target, identityInvalidValue, ctx); err != nil {
		return err
	}
	return reason
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
		return s.rejectAttributeIdentityValue(identityTarget, ctx, err)
	}
	fixed, hasFixed := decl.FixedValue()
	needs := runtime.SimpleNeedCanonical
	if identityTarget.needsIdentity() || hasFixed {
		needs |= runtime.SimpleNeedIdentity
	}
	typeID := decl.TypeID()
	value, err := s.validateSimpleValue(typeID, lexical, s.simpleValueQNameResolver(typeID), needs)
	if err != nil {
		return s.wildcardAttributeValueError(identityTarget, rn, ctx, err)
	}
	return s.commitKnownWildcardAttributeValue(identityTarget, value, fixed, hasFixed, rn, ctx)
}

func (s *session) wildcardAttributeValueError(target identityValueTarget, rn runtime.RuntimeName, ctx StartContext, err error) error {
	if rejectErr := s.doc.identity.rejectValue(target, identityInvalidValue, ctx); rejectErr != nil {
		return rejectErr
	}
	if invariantErr := simpleValueMetadataInvariant(err); invariantErr != nil {
		return invariantErr
	}
	if xsderrors.IsUnsupported(err) {
		return err
	}
	return validation(ctx, xsderrors.CodeValidationFacet, "invalid wildcard attribute "+rn.Label())
}

func (s *session) commitKnownWildcardAttributeValue(identityTarget identityValueTarget, value runtime.SimpleValue, fixed runtime.ValueConstraintRead, hasFixed bool, rn runtime.RuntimeName, ctx StartContext) error {
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
	if err := s.validateRequiredAttributes(set, seen, ctx); err != nil {
		return err
	}
	return s.validateDefaultAttributes(set, seen, ctx)
}

func (s *session) validateRequiredAttributes(set runtime.AttributeUseSetRead, seen AttributeSeen, ctx StartContext) error {
	required := set.RequiredSlots()
	for slotIndex := range required.Len() {
		if err := s.recoverAssessment(requiredAttributeError(set, seen, required, slotIndex, ctx)); err != nil {
			return err
		}
	}
	return nil
}

func requiredAttributeError(set runtime.AttributeUseSetRead, seen AttributeSeen, required runtime.AttributeUseSlots, slotIndex int, ctx StartContext) error {
	slot, ok := required.At(slotIndex)
	if !ok {
		return xsderrors.InternalInvariant("required attribute slot is invalid")
	}
	if seen.has(int(slot)) {
		return nil
	}
	use, ok := set.UseAt(int(slot))
	if !ok {
		return xsderrors.InternalInvariant("required attribute slot is invalid")
	}
	return attributeValidation(ctx, "missing required attribute "+use.Label())
}

func (s *session) validateDefaultAttributes(set runtime.AttributeUseSetRead, seen AttributeSeen, ctx StartContext) error {
	valueConstraints := set.ValueConstraintSlots()
	for slotIndex := range valueConstraints.Len() {
		if err := s.recoverAssessment(s.validateDefaultAttribute(set, seen, valueConstraints, slotIndex, ctx)); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) validateDefaultAttribute(set runtime.AttributeUseSetRead, seen AttributeSeen, valueConstraints runtime.AttributeUseSlots, slotIndex int, ctx StartContext) error {
	slot, ok := valueConstraints.At(slotIndex)
	if !ok {
		return xsderrors.InternalInvariant("value constraint attribute slot is invalid")
	}
	if seen.has(int(slot)) {
		return nil
	}
	use, ok := set.UseAt(int(slot))
	if !ok {
		return xsderrors.InternalInvariant("value constraint attribute slot is invalid")
	}
	if use.Required() {
		return nil
	}
	vc, ok := use.AbsentValueConstraint()
	if !ok {
		return nil
	}
	return s.recordDefaultAttribute(use, vc.SimpleValue(), ctx)
}

func (s *session) recordDefaultAttribute(use runtime.AttributeUseRead, value runtime.SimpleValue, ctx StartContext) error {
	target, err := s.doc.identity.prepareAttributeValue(knownIdentityAttributeName(use.Name()))
	if err != nil {
		return err
	}
	if err := s.doc.identity.recordValue(target, value, ctx); err != nil {
		return err
	}
	if err := s.doc.identity.captureValue(target, ctx); err != nil {
		return err
	}
	return s.doc.identity.commitValue(target)
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
			Namespaces:     s.limits.SchemaLocationNamespaces,
			NamespaceBytes: s.limits.SchemaLocationNamespaceBytes,
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
