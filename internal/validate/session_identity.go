package validate

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func (s *session) validateSimpleContent(f *frame, line, col int) (bool, error) {
	if f.Nilled {
		return false, nil
	}
	typeID := f.SimpleContent
	hasSimpleContent := typeID != runtime.NoSimpleType
	rawText := s.doc.text[f.TextStart:]
	var constraints runtime.ElementValueConstraints
	if f.TextContent.HasValueConstraint() {
		var ok bool
		constraints, _, ok = s.rt.ElementValueConstraints(f.Element)
		if !ok {
			return false, xsderrors.InternalInvariant("element value constraint metadata is invalid")
		}
	}
	if !hasSimpleContent {
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
	if !identityTarget.needsIdentity() && !f.TextContent.HasValueConstraint() {
		ok, rawErr := s.validateRawSimpleValue(typeID, rawText)
		if rawErr != nil {
			if invariantErr := simpleValueMetadataInvariant(rawErr); invariantErr != nil {
				return false, invariantErr
			}
			if ok {
				ctx := s.startContext(line, col)
				return false, validation(ctx, xsderrors.CodeValidationFacet, "invalid simple content: "+rawErr.Error())
			}
			return false, rawErr
		}
		if ok {
			return true, nil
		}
	}
	ctx := s.startContext(line, col)
	input := s.simpleContentValueInput(f.Type, rawText, constraints)
	if input.prevalidated {
		return s.recordElementSimpleContent(input.value, identityTarget, ctx)
	}
	value, err := s.validateSimpleValue(typeID, input.text, s.simpleValueQNameResolver(typeID), s.simpleContentNeeds(typeID, constraints, identityTarget.needsIdentity()))
	if err != nil {
		if invalidateErr := s.doc.identity.rejectValue(identityTarget, identityInvalidValue, ctx); invalidateErr != nil {
			return false, invalidateErr
		}
		if invariantErr := simpleValueMetadataInvariant(err); invariantErr != nil {
			return false, invariantErr
		}
		if xsderrors.IsUnsupported(err) {
			return false, err
		}
		return false, validation(ctx, xsderrors.CodeValidationFacet, "invalid simple content: "+err.Error())
	}
	if err := s.doc.identity.recordValue(identityTarget, value, ctx); err != nil {
		return false, err
	}
	if fixed, ok := constraints.FixedValue(); ok && value.CanonicalText() != fixed.CanonicalText() {
		if invalidateErr := s.doc.identity.rejectValue(identityTarget, identityInvalidValue, ctx); invalidateErr != nil {
			return false, invalidateErr
		}
		return false, validation(ctx, xsderrors.CodeValidationElement, "fixed element value mismatch")
	}
	if err := s.doc.identity.captureValue(identityTarget, ctx); err != nil {
		return false, err
	}
	if err := s.doc.identity.commitValue(identityTarget); err != nil {
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
	needIdentity bool,
) runtime.SimpleValueNeed {
	var needs runtime.SimpleValueNeed
	if needIdentity {
		needs |= runtime.SimpleNeedIdentity
	}
	if needIdentity || s.simpleIdentity(typeID) != runtime.SimpleIdentityNone {
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
