package validate

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

//nolint:gocognit // Keeping the raw fast-path gate here avoids copying full constraints into a helper.
func (s *session) validateSimpleContent(f *frame, line, col int) (bool, error) {
	if f.Nilled {
		return false, nil
	}
	typeID := f.SimpleContent
	rawText := s.doc.text[f.TextStart:]
	constraints, err := s.frameElementValueConstraints(f)
	if err != nil {
		return false, err
	}
	if typeID == runtime.NoSimpleType {
		if !f.TextContent.HasValueConstraint() {
			return false, nil
		}
		ctx := s.startContext(line, col)
		return false, s.validateNonSimpleFixedContent(f, constraints, rawText, ctx)
	}
	identityTarget, identityErr := s.doc.identity.prepareElementValue()
	if identityErr != nil {
		return false, identityErr
	}
	ctx := s.startContext(line, col)
	if !identityTarget.needsIdentity() && !constraints.HasAny() {
		rawResult, err := s.validateRawUnconstrainedElementSimpleContent(typeID, rawText, ctx)
		if rawResult.handled {
			return rawResult.captured, err
		}
	}
	input := s.simpleContentValueInput(f.Type, rawText, constraints)
	if input.prevalidated {
		return s.recordElementSimpleContent(input.value, identityTarget, ctx)
	}
	return s.validateElementSimpleContentValue(typeID, input.text, constraints, identityTarget, ctx)
}

func (s *session) frameElementValueConstraints(f *frame) (runtime.ElementValueConstraints, error) {
	if !f.TextContent.HasValueConstraint() {
		return runtime.ElementValueConstraints{}, nil
	}
	constraints, _, ok := s.rt.ElementValueConstraints(f.Element)
	if !ok {
		return runtime.ElementValueConstraints{}, xsderrors.InternalInvariant("element value constraint metadata is invalid")
	}
	return constraints, nil
}

type rawElementSimpleContentValidation struct {
	handled  bool
	captured bool
}

func (s *session) validateRawUnconstrainedElementSimpleContent(typeID runtime.SimpleTypeID, rawText []byte, ctx StartContext) (rawElementSimpleContentValidation, error) {
	result := s.validateRawSimpleValue(typeID, rawText)
	if result.err != nil {
		return rawElementSimpleContentValidation{handled: true}, rawElementSimpleContentError(result, ctx)
	}
	if result.handled {
		return rawElementSimpleContentValidation{handled: true, captured: true}, nil
	}
	return rawElementSimpleContentValidation{}, nil
}

func rawElementSimpleContentError(result rawSimpleValueValidation, ctx StartContext) error {
	if invariantErr := simpleValueMetadataInvariant(result.err); invariantErr != nil {
		return invariantErr
	}
	if result.handled {
		return validation(ctx, xsderrors.CodeValidationFacet, "invalid simple content: "+result.err.Error())
	}
	return result.err
}

func (s *session) validateElementSimpleContentValue(typeID runtime.SimpleTypeID, text string, constraints runtime.ElementValueConstraints, target identityValueTarget, ctx StartContext) (bool, error) {
	value, err := s.validateSimpleValue(typeID, text, s.simpleValueQNameResolver(typeID), s.simpleContentNeeds(typeID, constraints, target))
	if err != nil {
		return false, s.elementSimpleContentValueError(target, err, ctx)
	}
	return s.commitElementSimpleContentValue(value, constraints, target, ctx)
}

func (s *session) elementSimpleContentValueError(target identityValueTarget, err error, ctx StartContext) error {
	if rejectErr := s.doc.identity.rejectValue(target, identityInvalidValue, ctx); rejectErr != nil {
		return rejectErr
	}
	if invariantErr := simpleValueMetadataInvariant(err); invariantErr != nil {
		return invariantErr
	}
	if xsderrors.IsUnsupported(err) {
		return err
	}
	return validation(ctx, xsderrors.CodeValidationFacet, "invalid simple content: "+err.Error())
}

func (s *session) commitElementSimpleContentValue(value runtime.SimpleValue, constraints runtime.ElementValueConstraints, target identityValueTarget, ctx StartContext) (bool, error) {
	if err := s.doc.identity.recordValue(target, value, ctx); err != nil {
		return false, err
	}
	if fixed, ok := constraints.FixedValue(); ok && value.CanonicalText() != fixed.CanonicalText() {
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

type sessionSimpleContentInput struct {
	text         string
	value        runtime.SimpleValue
	prevalidated bool
}

func (s *session) simpleContentValueInput(
	typ runtime.TypeID,
	rawText []byte,
	constraints runtime.ElementValueConstraints,
) sessionSimpleContentInput {
	if len(rawText) != 0 {
		return sessionSimpleContentInput{text: s.valueStrings.Intern(rawText)}
	}
	if fixed, ok := constraints.FixedValue(); ok {
		if constraints.OwnerType() == typ {
			return sessionSimpleContentInput{value: fixed.SimpleValue(), prevalidated: true}
		}
		return sessionSimpleContentInput{text: fixed.LexicalText()}
	}
	if def, ok := constraints.DefaultValueConstraint(); ok {
		if constraints.OwnerType() == typ {
			return sessionSimpleContentInput{value: def.SimpleValue(), prevalidated: true}
		}
		return sessionSimpleContentInput{text: def.LexicalText()}
	}
	return sessionSimpleContentInput{text: s.valueStrings.Intern(rawText)}
}

func (s *session) simpleContentNeeds(
	typeID runtime.SimpleTypeID,
	constraints runtime.ElementValueConstraints,
	target identityValueTarget,
) runtime.SimpleValueNeed {
	var needs runtime.SimpleValueNeed
	if target.needsIdentity() {
		needs |= runtime.SimpleNeedIdentity
	}
	if target.needsIdentity() || s.simpleIdentity(typeID) != runtime.SimpleIdentityNone {
		needs |= runtime.SimpleNeedCanonical
		return needs
	}
	if _, fixed := constraints.FixedValue(); fixed {
		needs |= runtime.SimpleNeedCanonical
	}
	return needs
}

func (s *session) simpleIdentity(id runtime.SimpleTypeID) runtime.SimpleIdentityKind {
	return s.rt.SimpleIdentity(id)
}

func (s *session) validateNonSimpleFixedContent(
	f *frame,
	constraints runtime.ElementValueConstraints,
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
	text := s.valueStrings.Intern(rawText)
	if text != "" && text != fixed.LexicalText() {
		return validation(ctx, xsderrors.CodeValidationElement, "fixed element value mismatch")
	}
	return nil
}

func (s *session) recordElementSimpleContent(
	value runtime.SimpleValue,
	identityTarget identityValueTarget,
	ctx StartContext,
) (bool, error) {
	if err := s.doc.identity.recordValue(identityTarget, value, ctx); err != nil {
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

func knownIdentityAttributeName(name runtime.QName) runtime.RuntimeName {
	return runtime.RuntimeName{Name: name, Known: true}
}

func (s *session) checkIDRefs() error {
	return s.doc.identity.endDocument(func(err error) error {
		return s.recover(err)
	})
}
