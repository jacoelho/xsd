package validate

import (
	"encoding/xml"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	xsdValue "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func (s *session) validateAttributes(typ xsdSchema.TypeID, attrs []xmlstream.Attr, line, col int) error {
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
	if len(attrs) == 0 && set.RequiredSlots().Len() == 0 && set.ValueConstraintSlots().Len() == 0 {
		return nil
	}
	return s.validateAttributeSet(set, attrs, line, col)
}

func (s *session) attributeUseSetForType(typ xsdSchema.TypeID) (set xsdSchema.AttributeUseSetRead, present, valid bool) {
	return s.rt.AttributeUseSetForType(typ)
}

func (s *session) attributeDecl(id xsdSchema.AttributeID) (xsdSchema.AttributeDeclRead, bool) {
	return s.rt.AttributeDecl(id)
}

// attributeValue reads attr through the instance reader. The session never
// configures the reader to skip values, so a skipped value is an invariant
// violation.
func (s *session) attributeValue(attr *xmlstream.Attr) (string, error) {
	value, err := s.reader.AttrValue(attr)
	if err != nil {
		return "", xsderrors.InternalInvariant("instance reader skipped attribute value")
	}
	return value, nil
}

func (s *session) validateSimpleValue(
	id xsdSchema.SimpleTypeID,
	lexical string,
	resolve xsdValue.Resolver,
	needs xsdValue.Needs,
) (xsdValue.Value, error) {
	return s.rt.ValueProgram().Validate(id, lexical, resolve, needs, s.limits.evaluation(&s.patternTotal), &s.valueScratch)
}

func (s *session) validateSimpleValueBytes(
	id xsdSchema.SimpleTypeID,
	lexical []byte,
	input xsdValue.InputRequirements,
	needs xsdValue.Needs,
) (xsdValue.Value, error) {
	program := s.rt.ValueProgram()
	resolve := s.simpleValueQNameResolver(input)
	// QName/NOTATION resolution and document identity projections consume
	// lexical strings. Reuse the reader's bounded owned spellings; resolve them
	// again against the current namespace bindings on every evaluation.
	if input.NeedsQName || input.DocumentIdentity || needs != 0 && input.UnconstrainedString {
		lexicalString := s.reader.InternBytes(lexical)
		return program.ValidateAdmitted(id, lexicalString, resolve, needs, s.limits.evaluation(&s.patternTotal), &s.valueScratch)
	}
	return program.ValidateBytes(id, lexical, resolve, needs, s.limits.evaluation(&s.patternTotal), &s.valueScratch)
}

func (s *session) validateAdmittedSimpleValue(id xsdSchema.SimpleTypeID, lexical string, input xsdValue.InputRequirements, needs xsdValue.Needs) (xsdValue.Value, error) {
	return s.rt.ValueProgram().ValidateAdmitted(id, lexical, s.simpleValueQNameResolver(input), needs, s.limits.evaluation(&s.patternTotal), &s.valueScratch)
}

func (s *session) validateAttributeSet(set xsdSchema.AttributeUseSetRead, attrs []xmlstream.Attr, line, col int) error {
	seen := newAttributeSeenWithScratch(set.UseCount(), &s.attributeSeen)
	ctx := s.startContext(line, col)
	for i := range attrs {
		if err := s.validateAttribute(ctx, set, &seen, &attrs[i], line, col); err != nil {
			return err
		}
	}
	return s.validateRequiredAndDefaultAttributes(ctx, set, seen)
}

func (s *session) validateAttribute(ctx startContext, set xsdSchema.AttributeUseSetRead, seen *attributeSeen, attr *xmlstream.Attr, line, col int) error {
	handled, err := s.validateReservedAttribute(attr, line, col)
	if err != nil || handled {
		if handled && set.RequiredSlots().Len() != 0 {
			s.markIntrinsicAttributePresent(set, seen, attr.Name)
		}
		return err
	}
	rn := s.runtimeName(attr.Name)
	handled, err = s.validateDeclaredAttribute(ctx, set, seen, rn, attr)
	if err != nil || handled {
		return err
	}
	return s.validateUndeclaredAttribute(ctx, set, rn, attr)
}

// Intrinsic XSI assessment bypasses declared value constraints, but physical
// presence still satisfies required uses.
func (s *session) markIntrinsicAttributePresent(set xsdSchema.AttributeUseSetRead, seen *attributeSeen, name xml.Name) {
	if !isXSIAttributeName(name) {
		return
	}
	rn := s.runtimeName(name)
	if !rn.Known {
		return
	}
	if _, slot, ok := set.DeclaredUse(rn.Name); ok {
		seen.mark(slot)
	}
}

func (s *session) validateDeclaredAttribute(ctx startContext, set xsdSchema.AttributeUseSetRead, seen *attributeSeen, rn xsdSchema.RuntimeName, attr *xmlstream.Attr) (bool, error) {
	if !rn.Known {
		return false, nil
	}
	use, slot, ok := set.DeclaredUse(rn.Name)
	if !ok {
		return false, nil
	}
	if !seen.mark(slot) {
		return true, s.recoverAssessment(attributeValidation(ctx, duplicateAttributeMessage(rn)))
	}
	return true, s.recoverAssessment(s.validateDeclaredAttributeUse(ctx, &use, rn, attr))
}

func (s *session) validateUndeclaredAttribute(ctx startContext, set xsdSchema.AttributeUseSetRead, rn xsdSchema.RuntimeName, attr *xmlstream.Attr) error {
	handled, err := s.validateWildcardAttribute(ctx, set, rn, attr)
	if err != nil {
		return s.recoverUnassessedIdentityAttribute(rn, err)
	}
	if handled {
		return nil
	}
	return s.recoverUnassessedIdentityAttribute(rn, attributeValidation(ctx, undeclaredAttributeMessage(rn)))
}

func (s *session) validateSimpleTypeAttributes(attrs []xmlstream.Attr, line, col int) error {
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
		if err := s.recoverUnassessedIdentityAttribute(rn, attributeValidation(ctx, simpleTypeAttributeMessage(rn))); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) validateReservedAttribute(attr *xmlstream.Attr, line, col int) (bool, error) {
	name := attr.Name
	if name.Space == "" && name.Local != vocab.XMLNSPrefix {
		return false, nil
	}
	if xmlstream.IsExpandedNamespaceName(name) {
		return true, nil
	}
	if !isXSIAttributeName(name) {
		return false, nil
	}
	value, err := s.attributeValue(attr)
	if err != nil {
		return true, err
	}
	if isSchemaLocationHintName(name) {
		if err := s.recoverAssessment(s.locateCurrent(checkSchemaLocationHint(name.Local, value), line, col)); err != nil {
			return true, err
		}
	}
	return true, s.validateXSIAttribute(name, value, line, col)
}

func (s *session) validateDeclaredAttributeUse(
	ctx startContext, use *xsdSchema.AttributeUseRead,
	rn xsdSchema.RuntimeName,
	attr *xmlstream.Attr,
) error {
	identityTarget, targetErr := s.doc.identity.prepareAttributeValue(rn, s.doc.Depth())
	if targetErr != nil {
		return targetErr
	}
	if err := s.validateAttributeTypeAvailable(ctx, use.TypeID(), rn.Label()); err != nil {
		return s.rejectAttributeIdentityValue(identityTarget, err)
	}
	input, ok := s.rt.ValueProgram().InputRequirements(use.TypeID())
	if !ok {
		return s.rejectAttributeIdentityValue(identityTarget, xsderrors.InternalInvariant("attribute input metadata is invalid"))
	}
	plan := newDeclaredAttributePlan(use, identityTarget, input)
	if handled, err := s.validateDeclaredAttributeFast(ctx, &plan, attr, rn); handled {
		return err
	}
	lexical, err := s.attributeValue(attr)
	if err != nil {
		return s.rejectAttributeIdentityValue(identityTarget, err)
	}
	return s.validateDeclaredAttributeValue(ctx, &plan, lexical, rn)
}

type declaredAttributePlan struct {
	fixed    xsdSchema.ValueConstraintRead
	typeID   xsdSchema.SimpleTypeID
	target   identityValueTarget
	input    xsdValue.InputRequirements
	needs    xsdValue.Needs
	hasFixed bool
}

func newDeclaredAttributePlan(use *xsdSchema.AttributeUseRead, target identityValueTarget, input xsdValue.InputRequirements) declaredAttributePlan {
	fixed, hasFixed := use.FixedValue()
	var needs xsdValue.Needs
	if target.needsIdentity() || hasFixed {
		needs |= xsdValue.NeedIdentity
	}
	return declaredAttributePlan{typeID: use.TypeID(), target: target, fixed: fixed, needs: needs, hasFixed: hasFixed, input: input}
}

// canValidateFixedString selects the projection-free fixed check: when no
// identity value is needed and the type is LexicalCanonical, an explicit
// attribute is validated without value projections and then compared as raw
// text with the fixed value's CanonicalText, because an accepted lexical form
// is its own canonical value.
func (p *declaredAttributePlan) canValidateFixedString() bool {
	return !p.target.needsIdentity() && p.hasFixed && p.input.LexicalCanonical
}

func (p *declaredAttributePlan) canValidateRaw() bool {
	return !p.target.needsIdentity() && !p.hasFixed
}

func (s *session) validateDeclaredAttributeFast(ctx startContext, plan *declaredAttributePlan, attr *xmlstream.Attr, rn xsdSchema.RuntimeName) (bool, error) {
	if plan.canValidateFixedString() {
		lexical, err := s.attributeValue(attr)
		if err != nil {
			return true, err
		}
		if _, err := s.validateAdmittedSimpleValue(plan.typeID, lexical, plan.input, 0); err != nil {
			return true, simpleValueError(ctx, xsderrors.CodeValidationFacet, invalidAttributeMessage(rn), err, s.limits.evaluation(&s.patternTotal))
		}
		return true, validateFixedAttributeString(ctx, lexical, plan.fixed, rn)
	}
	// ID/IDREF-carrying types use the owned lexical path below.
	if !plan.canValidateRaw() || plan.input.DocumentIdentity {
		return false, nil
	}
	if raw, ok := attr.RawValue(); ok {
		typeID := plan.typeID
		value, err := s.validateSimpleValueBytes(typeID, raw, plan.input, 0)
		if err != nil {
			return true, simpleValueError(ctx, xsderrors.CodeValidationFacet, invalidAttributeMessage(rn), err, s.limits.evaluation(&s.patternTotal))
		}
		return true, s.doc.identity.recordValue(ctx, plan.target, value)
	}
	return false, nil
}

func validateFixedAttributeString(ctx startContext, value string, fixed xsdSchema.ValueConstraintRead, rn xsdSchema.RuntimeName) error {
	if value != fixed.CanonicalText() {
		return attributeValidation(ctx, fixedAttributeMessage(rn.Label(), value, fixed.CanonicalText()))
	}
	return nil
}

func (s *session) validateDeclaredAttributeValue(ctx startContext, plan *declaredAttributePlan, lexical string, rn xsdSchema.RuntimeName) error {
	typeID := plan.typeID
	value, err := s.validateAdmittedSimpleValue(typeID, lexical, plan.input, plan.needs)
	if err != nil {
		return s.declaredAttributeValueError(ctx, plan.target, rn, err)
	}
	if err := s.doc.identity.recordValue(ctx, plan.target, value); err != nil {
		return err
	}
	if err := s.doc.identity.captureValue(ctx, plan.target); err != nil {
		return err
	}
	if plan.hasFixed {
		if err := s.validateFixedAttributeValue(ctx, value, plan.fixed, plan.target, rn.Label(), lexical); err != nil {
			return err
		}
	}
	return s.doc.identity.commitValue(plan.target)
}

func (s *session) declaredAttributeValueError(ctx startContext, target identityValueTarget, rn xsdSchema.RuntimeName, err error) error {
	if rejectErr := s.doc.identity.rejectValue(target); rejectErr != nil {
		return rejectErr
	}
	return simpleValueError(ctx, xsderrors.CodeValidationFacet, invalidAttributeMessage(rn), err, s.limits.evaluation(&s.patternTotal))
}

func (s *session) rejectAttributeIdentityValue(target identityValueTarget, reason error) error {
	if err := s.doc.identity.rejectValue(target); err != nil {
		return err
	}
	return reason
}

func (s *session) validateWildcardAttribute(
	ctx startContext, set xsdSchema.AttributeUseSetRead,
	rn xsdSchema.RuntimeName,
	attr *xmlstream.Attr,
) (bool, error) {
	match, valid := matchAttributeWildcard(s.rt, set.Wildcard(), rn)
	if !valid {
		return true, xsderrors.InternalInvariant("attribute wildcard state is invalid")
	}
	switch match.disposition {
	case attributeWildcardNoMatch:
		return false, nil
	case attributeWildcardStrictMissing:
		return true, attributeValidation(ctx, strictAttributeMessage(rn))
	case attributeWildcardSkip:
		return true, nil
	case attributeWildcardLaxMissing:
		return true, s.captureUnknownIdentityAttribute(ctx, rn)
	case attributeWildcardDeclared:
		decl, ok := s.attributeDecl(match.attribute)
		if !ok {
			return true, xsderrors.InternalInvariant("attribute wildcard matched invalid declaration")
		}
		value, err := s.attributeValue(attr)
		if err != nil {
			return true, err
		}
		return true, s.validateKnownWildcardAttribute(ctx, set, decl, rn, value)
	}
	return true, xsderrors.InternalInvariant("attribute wildcard match disposition is invalid")
}

// captureUnknownIdentityAttribute records an attribute that a lax wildcard
// admitted without a declaration; identity fields see it as an unknown value.
func (s *session) captureUnknownIdentityAttribute(ctx startContext, rn xsdSchema.RuntimeName) error {
	target, err := s.doc.identity.prepareAttributeValue(rn, s.doc.Depth())
	if err != nil {
		return err
	}
	return s.doc.identity.captureUnknownValue(ctx, target)
}

func (s *session) recoverUnassessedIdentityAttribute(rn xsdSchema.RuntimeName, err error) error {
	target, targetErr := s.doc.identity.prepareAttributeValue(rn, s.doc.Depth())
	if targetErr != nil {
		return targetErr
	}
	if invalidateErr := s.doc.identity.rejectValue(target); invalidateErr != nil {
		return invalidateErr
	}
	return s.recoverAssessment(err)
}

// validateKnownWildcardAttribute assesses a wildcard-admitted attribute that
// has a global declaration. An unavailable declared type fails first. An
// ID-typed declaration conflicts with an ID attribute use of the set even when
// that use is absent from the instance, and the conflict is reported before
// any value is recorded.
func (s *session) validateKnownWildcardAttribute(
	ctx startContext, set xsdSchema.AttributeUseSetRead,
	decl xsdSchema.AttributeDeclRead,
	rn xsdSchema.RuntimeName,
	lexical string,
) error {
	identityTarget, targetErr := s.doc.identity.prepareAttributeValue(rn, s.doc.Depth())
	if targetErr != nil {
		return targetErr
	}
	if err := s.validateAttributeTypeAvailable(ctx, decl.TypeID(), rn.Label()); err != nil {
		return s.rejectAttributeIdentityValue(identityTarget, err)
	}
	fixed, hasFixed := decl.FixedValue()
	needs := xsdValue.NeedCanonical
	if identityTarget.needsIdentity() || hasFixed {
		needs |= xsdValue.NeedIdentity
	}
	typeID := decl.TypeID()
	input, ok := s.rt.ValueProgram().InputRequirements(typeID)
	if !ok {
		return s.rejectAttributeIdentityValue(identityTarget, xsderrors.InternalInvariant("attribute input metadata is invalid"))
	}
	if set.HasIDUse() && input.CarriesID {
		return s.rejectAttributeIdentityValue(identityTarget, attributeValidation(ctx, msgWildcardIDDeclaredUse))
	}
	value, err := s.validateAdmittedSimpleValue(typeID, lexical, input, needs)
	if err != nil {
		return s.wildcardAttributeValueError(ctx, identityTarget, rn, err)
	}
	return s.commitKnownWildcardAttributeValue(ctx, identityTarget, value, optionalFixedAttribute{value: fixed, present: hasFixed}, rn, lexical)
}

func (s *session) wildcardAttributeValueError(ctx startContext, target identityValueTarget, rn xsdSchema.RuntimeName, err error) error {
	if rejectErr := s.doc.identity.rejectValue(target); rejectErr != nil {
		return rejectErr
	}
	return simpleValueError(ctx, xsderrors.CodeValidationFacet, invalidWildcardAttributeMessage(rn), err, s.limits.evaluation(&s.patternTotal))
}

type optionalFixedAttribute struct {
	value   xsdSchema.ValueConstraintRead
	present bool
}

func (s *session) commitKnownWildcardAttributeValue(ctx startContext, identityTarget identityValueTarget, value xsdValue.Value, fixed optionalFixedAttribute, rn xsdSchema.RuntimeName, lexical string) error {
	if err := s.doc.identity.recordValue(ctx, identityTarget, value); err != nil {
		return err
	}
	if fixed.present {
		if err := s.validateFixedAttributeValue(ctx, value, fixed.value, identityTarget, rn.Label(), lexical); err != nil {
			return err
		}
	}
	if err := s.doc.identity.captureValue(ctx, identityTarget); err != nil {
		return err
	}
	return s.doc.identity.commitValue(identityTarget)
}

func (s *session) validateFixedAttributeValue(
	ctx startContext, actual xsdValue.Value,
	fixed xsdSchema.ValueConstraintRead,
	identityTarget identityValueTarget,
	label string,
	lexical string,
) error {
	equal, valid := fixedValueEqual(actual, fixed.Value())
	if equal {
		return nil
	}
	if invalidateErr := s.doc.identity.rejectValue(identityTarget); invalidateErr != nil {
		return invalidateErr
	}
	if !valid {
		return xsderrors.InternalInvariant("fixed attribute value-space identity is missing")
	}
	return attributeValidation(ctx, fixedAttributeMessage(label, lexical, fixed.CanonicalText()))
}

func (s *session) validateAttributeTypeAvailable(ctx startContext, id xsdSchema.SimpleTypeID, label string) error {
	unavailable, ok := s.rt.SimpleTypeUnavailable(id)
	if !ok {
		return xsderrors.InternalInvariant("attribute type metadata is invalid")
	}
	if unavailable {
		return attributeValidation(ctx, unavailableAttributeTypeMessage(label))
	}
	return nil
}

func (s *session) validateRequiredAndDefaultAttributes(
	ctx startContext, set xsdSchema.AttributeUseSetRead,
	seen attributeSeen,
) error {
	if err := s.validateRequiredAttributes(ctx, set, seen); err != nil {
		return err
	}
	return s.validateDefaultAttributes(ctx, set, seen)
}

func (s *session) validateRequiredAttributes(ctx startContext, set xsdSchema.AttributeUseSetRead, seen attributeSeen) error {
	required := set.RequiredSlots()
	for slotIndex := range required.Len() {
		if err := s.recoverAssessment(requiredAttributeError(ctx, set, seen, required, slotIndex)); err != nil {
			return err
		}
	}
	return nil
}

func requiredAttributeError(ctx startContext, set xsdSchema.AttributeUseSetRead, seen attributeSeen, required xsdSchema.AttributeUseSlots, slotIndex int) error {
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
	return attributeValidation(ctx, requiredAttributeMessage(use.Label()))
}

func (s *session) validateDefaultAttributes(ctx startContext, set xsdSchema.AttributeUseSetRead, seen attributeSeen) error {
	valueConstraints := set.ValueConstraintSlots()
	for slotIndex := range valueConstraints.Len() {
		if err := s.recoverAssessment(s.validateDefaultAttribute(ctx, set, seen, valueConstraints, slotIndex)); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) validateDefaultAttribute(ctx startContext, set xsdSchema.AttributeUseSetRead, seen attributeSeen, valueConstraints xsdSchema.AttributeUseSlots, slotIndex int) error {
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
	return s.applyDefaultAttributeValue(ctx, use, vc.Value())
}

func (s *session) applyDefaultAttributeValue(ctx startContext, use xsdSchema.AttributeUseRead, value xsdValue.Value) error {
	target, err := s.doc.identity.prepareAttributeValue(knownIdentityAttributeName(use.Name()), s.doc.Depth())
	if err != nil {
		return err
	}
	if err := s.doc.identity.recordValue(ctx, target, value); err != nil {
		return err
	}
	if err := s.doc.identity.captureValue(ctx, target); err != nil {
		return err
	}
	return s.doc.identity.commitValue(target)
}

func (s *session) validateXSIAttribute(name xml.Name, value string, line, col int) error {
	rn := resolveRuntimeName(s.rt, name)
	target, err := s.doc.identity.prepareAttributeValue(rn, s.doc.Depth())
	if err != nil {
		return err
	}
	if err := s.doc.identity.captureXSIAttribute(
		s.startContext(line, col), target,
		name,
		value,
		s.qnameResolver(),
		s.limits.evaluation(&s.patternTotal),
	); err != nil {
		return s.recover(err)
	}
	return nil
}

// fixedValueEqual compares in value space; valid is false when either side
// lacks identity. Validation owns this policy for attribute and element paths.
func fixedValueEqual(actual, fixed xsdValue.Value) (equal, valid bool) {
	if !actual.HasIdentity() || !fixed.HasIdentity() {
		return false, false
	}
	return actual.Equal(fixed), true
}
