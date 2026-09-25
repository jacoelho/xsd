package validate

import (
	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	xsdValue "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

//nolint:gocognit // Keeping the raw fast-path gate here avoids copying full constraints into a helper.
func (s *session) validateSimpleContent(f *frame, line, col int) (bool, error) {
	if f.Nilled {
		return false, nil
	}
	typeID := f.SimpleContent
	rawText := s.doc.text[f.TextStart:]
	hasValueConstraint := f.TextContent.HasValueConstraint()
	if typeID == xsdSchema.NoSimpleType {
		if !hasValueConstraint {
			return false, nil
		}
		constraints, err := s.frameElementValueConstraints(f)
		if err != nil {
			return false, err
		}
		ctx := s.startContext(line, col)
		return false, s.validateNonSimpleFixedContent(f, constraints, rawText, ctx)
	}
	var constraints xsdSchema.ElementValueConstraints
	if hasValueConstraint {
		var err error
		constraints, err = s.frameElementValueConstraints(f)
		if err != nil {
			return false, err
		}
	}
	identityTarget, identityErr := s.doc.identity.prepareElementValue(s.doc.Depth())
	if identityErr != nil {
		return false, identityErr
	}
	ctx := s.startContext(line, col)
	needsIdentity := identityTarget.needsIdentity()
	input, ok := s.rt.ValueProgram().InputRequirements(typeID)
	if !ok {
		return false, xsderrors.InternalInvariant("simple content input metadata is invalid")
	}
	if !needsIdentity && !hasValueConstraint && input.Identity == xsdValue.IdentityNone {
		value, err := s.validateSimpleValueBytes(typeID, rawText, input, 0)
		if err != nil {
			return false, simpleValueFacetError(ctx, "invalid simple content", err)
		}
		return true, s.doc.identity.recordValue(identityTarget, value, ctx)
	}
	if len(rawText) == 0 && hasValueConstraint {
		constraint, present := constraints.FixedValue()
		if !present {
			constraint, present = constraints.DefaultValueConstraint()
		}
		if present {
			if constraints.OwnerType() == f.Type {
				return s.recordElementSimpleContent(constraint.Value(), identityTarget, ctx)
			}
			return s.applyElementValueConstraint(typeID, constraint, identityTarget, input, ctx)
		}
	}
	return s.validateElementSimpleContentValue(typeID, rawText, constraints, identityTarget, input, ctx)
}

func (s *session) frameElementValueConstraints(f *frame) (xsdSchema.ElementValueConstraints, error) {
	if !f.TextContent.HasValueConstraint() {
		return xsdSchema.ElementValueConstraints{}, nil
	}
	constraints, _, ok := s.rt.ElementValueConstraints(f.Element)
	if !ok {
		return xsdSchema.ElementValueConstraints{}, xsderrors.InternalInvariant("element value constraint metadata is invalid")
	}
	return constraints, nil
}

func (s *session) applyElementValueConstraint(typeID xsdSchema.SimpleTypeID, constraint xsdSchema.ValueConstraintRead, target identityValueTarget, input xsdValue.InputRequirements, ctx StartContext) (bool, error) {
	var resolver xsdValue.Resolver
	if input.NeedsQName {
		resolver.QName = constraint.ResolveQName
		resolver.Notation = s.rt.NotationDeclared
	}
	result, err := s.validateSimpleValue(typeID, constraint.ApplicationText(), resolver, simpleContentNeeds(target, input.Identity))
	if err != nil {
		return false, s.elementSimpleContentValueError(target, err, ctx)
	}
	// The constraint supplies this value. Its actual type must accept it, but
	// stronger normalization cannot make it disagree with its own source.
	return s.recordElementSimpleContent(result, target, ctx)
}

func (s *session) validateElementSimpleContentValue(typeID xsdSchema.SimpleTypeID, rawText []byte, constraints xsdSchema.ElementValueConstraints, target identityValueTarget, input xsdValue.InputRequirements, ctx StartContext) (bool, error) {
	needs := simpleContentNeeds(target, input.Identity)
	if _, fixed := constraints.FixedValue(); fixed {
		needs |= xsdValue.NeedCanonical | xsdValue.NeedIdentity
	}
	result, err := s.validateSimpleValueBytes(typeID, rawText, input, needs)
	if err != nil {
		return false, s.elementSimpleContentValueError(target, err, ctx)
	}
	return s.commitElementSimpleContentValue(result, constraints, target, ctx)
}

func (s *session) elementSimpleContentValueError(target identityValueTarget, err error, ctx StartContext) error {
	if rejectErr := s.doc.identity.rejectValue(target, identityInvalidValue, ctx); rejectErr != nil {
		return rejectErr
	}
	return simpleValueFacetError(ctx, "invalid simple content", err)
}

func (s *session) commitElementSimpleContentValue(result xsdValue.Value, constraints xsdSchema.ElementValueConstraints, target identityValueTarget, ctx StartContext) (bool, error) {
	if err := s.doc.identity.recordValue(target, result, ctx); err != nil {
		return false, err
	}
	if fixed, ok := constraints.FixedValue(); ok && !result.Equal(fixed.Value()) {
		if rejectErr := s.doc.identity.rejectValue(target, identityInvalidValue, ctx); rejectErr != nil {
			return false, rejectErr
		}
		return false, validation(ctx, xsderrors.CodeValidationElement, "fixed element value mismatch")
	}
	if err := s.doc.identity.captureValue(target, ctx); err != nil {
		return false, err
	}
	if err := s.doc.identity.commitValue(target); err != nil {
		return false, err
	}
	return true, nil
}

func simpleContentNeeds(
	target identityValueTarget,
	typeIdentity xsdValue.IdentityKind,
) xsdValue.Needs {
	var needs xsdValue.Needs
	if target.needsIdentity() {
		needs |= xsdValue.NeedIdentity
	}
	if target.needsIdentity() || typeIdentity != xsdValue.IdentityNone {
		needs |= xsdValue.NeedCanonical
		return needs
	}
	return needs
}

func (*session) validateNonSimpleFixedContent(
	f *frame,
	constraints xsdSchema.ElementValueConstraints,
	rawText []byte,
	ctx StartContext,
) error {
	fixed, ok := constraints.FixedValue()
	if !ok {
		return nil
	}
	if f.HasChild {
		return validation(ctx, xsderrors.CodeValidationElement, "fixed element value mismatch")
	}
	if len(rawText) != 0 && !bytesEqualString(rawText, fixed.ApplicationText()) {
		return validation(ctx, xsderrors.CodeValidationElement, "fixed element value mismatch")
	}
	return nil
}

func bytesEqualString(raw []byte, text string) bool {
	if len(raw) != len(text) {
		return false
	}
	for i, b := range raw {
		if b != text[i] {
			return false
		}
	}
	return true
}

func (s *session) recordElementSimpleContent(
	result xsdValue.Value,
	identityTarget identityValueTarget,
	ctx StartContext,
) (bool, error) {
	if err := s.doc.identity.recordValue(identityTarget, result, ctx); err != nil {
		return false, err
	}
	if err := s.doc.identity.captureValue(identityTarget, ctx); err != nil {
		return false, err
	}
	if err := s.doc.identity.commitValue(identityTarget); err != nil {
		return false, err
	}
	return true, nil
}

func knownIdentityAttributeName(name xsdSchema.QName) xsdSchema.RuntimeName {
	return xsdSchema.RuntimeName{Name: name, Known: true}
}

func (s *session) checkIDRefs() error {
	return s.doc.identity.endDocument(func(err error) error {
		return s.recover(err)
	})
}
