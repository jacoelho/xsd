package validate

import (
	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	xsdValue "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

// validateSimpleContent assesses an element's simple content at its end. Only
// empty text takes the declaration's value constraint; whitespace-only text is
// instance text. A constraint owned by the element's actual type supplies its
// stored value directly. When xsi:type changed the type, the constraint's
// application text is revalidated against the actual type with the
// constraint's schema namespace projection and the instance value-work limit,
// and that supplied value needs no second fixed-value comparison. Instance
// text uses instance namespaces and is compared to a fixed value by typed
// equality.
//
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
		return false, s.validateNonSimpleFixedContent(ctx, f, constraints, rawText)
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
	if !needsIdentity && !hasValueConstraint && !input.DocumentIdentity {
		value, err := s.validateSimpleValueBytes(typeID, rawText, input, 0)
		if err != nil {
			return false, simpleValueError(ctx, xsderrors.CodeValidationFacet, msgInvalidSimpleContent, err, s.limits.evaluation(&s.patternTotal))
		}
		return true, s.doc.identity.recordValue(ctx, identityTarget, value)
	}
	if len(rawText) == 0 && hasValueConstraint {
		constraint, present := constraints.FixedValue()
		if !present {
			constraint, present = constraints.DefaultValueConstraint()
		}
		if present {
			// cvc-elt clause 5.1.2 validates the constraint against the actual
			// type in the element's namespace scope, the reading recorded in
			// docs/spec/INTERPRETATIONS.md. Only a value whose lexical mapping
			// reads no namespace bindings can reuse the value compiled against
			// the same type.
			if constraints.OwnerType() == f.Type && !input.NeedsQName {
				return s.recordElementSimpleContent(ctx, constraint.Value(), identityTarget)
			}
			return s.applyElementValueConstraint(ctx, typeID, constraint, identityTarget, input)
		}
	}
	return s.validateElementSimpleContentValue(ctx, typeID, rawText, constraints, identityTarget, input)
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

func (s *session) applyElementValueConstraint(ctx startContext, typeID xsdSchema.SimpleTypeID, constraint xsdSchema.ValueConstraintRead, target identityValueTarget, input xsdValue.InputRequirements) (bool, error) {
	result, err := s.validateAdmittedSimpleValue(typeID, constraint.ApplicationText(), input, simpleContentNeeds(target, input))
	if err != nil {
		return false, s.elementSimpleContentValueError(ctx, target, err)
	}
	// The constraint supplies this value. Its actual type must accept it, but
	// stronger normalization cannot make it disagree with its own source.
	return s.recordElementSimpleContent(ctx, result, target)
}

func (s *session) validateElementSimpleContentValue(ctx startContext, typeID xsdSchema.SimpleTypeID, rawText []byte, constraints xsdSchema.ElementValueConstraints, target identityValueTarget, input xsdValue.InputRequirements) (bool, error) {
	needs := simpleContentNeeds(target, input)
	if _, fixed := constraints.FixedValue(); fixed {
		needs |= xsdValue.NeedCanonical | xsdValue.NeedIdentity
	}
	result, err := s.validateSimpleValueBytes(typeID, rawText, input, needs)
	if err != nil {
		return false, s.elementSimpleContentValueError(ctx, target, err)
	}
	return s.commitElementSimpleContentValue(ctx, result, constraints, target)
}

func (s *session) elementSimpleContentValueError(ctx startContext, target identityValueTarget, err error) error {
	if rejectErr := s.doc.identity.rejectValue(target); rejectErr != nil {
		return rejectErr
	}
	return simpleValueError(ctx, xsderrors.CodeValidationFacet, msgInvalidSimpleContent, err, s.limits.evaluation(&s.patternTotal))
}

func (s *session) commitElementSimpleContentValue(ctx startContext, result xsdValue.Value, constraints xsdSchema.ElementValueConstraints, target identityValueTarget) (bool, error) {
	if err := s.doc.identity.recordValue(ctx, target, result); err != nil {
		return false, err
	}
	if fixed, ok := constraints.FixedValue(); ok {
		if err := s.checkFixedElementValue(ctx, result, fixed, target); err != nil {
			return false, err
		}
	}
	if err := s.doc.identity.captureValue(ctx, target); err != nil {
		return false, err
	}
	if err := s.doc.identity.commitValue(target); err != nil {
		return false, err
	}
	return true, nil
}

func (s *session) checkFixedElementValue(ctx startContext, result xsdValue.Value, fixed xsdSchema.ValueConstraintRead, target identityValueTarget) error {
	equal, valid := fixedValueEqual(result, fixed.Value())
	if equal {
		return nil
	}
	if rejectErr := s.doc.identity.rejectValue(target); rejectErr != nil {
		return rejectErr
	}
	if !valid {
		return xsderrors.InternalInvariant("fixed element value-space identity is missing")
	}
	return validation(ctx, xsderrors.CodeValidationElement, fixedElementMessage(result.CanonicalText(), fixed.CanonicalText()))
}

func simpleContentNeeds(
	target identityValueTarget,
	input xsdValue.InputRequirements,
) xsdValue.Needs {
	var needs xsdValue.Needs
	if target.needsIdentity() {
		needs |= xsdValue.NeedIdentity
	}
	if target.needsIdentity() || input.DocumentIdentity {
		needs |= xsdValue.NeedCanonical
	}
	return needs
}

// validateNonSimpleFixedContent checks a fixed value on untyped mixed content
// by lexical equality with the constraint's application text; element
// children always conflict and empty text takes the constraint.
func (*session) validateNonSimpleFixedContent(
	ctx startContext, f *frame,
	constraints xsdSchema.ElementValueConstraints,
	rawText []byte,
) error {
	fixed, ok := constraints.FixedValue()
	if !ok {
		return nil
	}
	if f.HasChild {
		return validation(ctx, xsderrors.CodeValidationElement, fixedChildContentMessage(fixed.CanonicalText()))
	}
	if len(rawText) != 0 && !bytesEqualString(rawText, fixed.ApplicationText()) {
		return validation(ctx, xsderrors.CodeValidationElement, fixedElementMessage(rawText, fixed.ApplicationText()))
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
	ctx startContext, result xsdValue.Value,
	identityTarget identityValueTarget,
) (bool, error) {
	if err := s.doc.identity.recordValue(ctx, identityTarget, result); err != nil {
		return false, err
	}
	if err := s.doc.identity.captureValue(ctx, identityTarget); err != nil {
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
	return s.doc.identity.endDocument(&s.doc.xmlDocument, func(err error) error {
		return s.recover(err)
	})
}
