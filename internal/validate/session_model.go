package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

type acceptedChild struct {
	element runtime.ElementID
	typ     runtime.TypeID
	skip    bool
	recover bool
}

func (s *session) acceptChild(parent *frame, rn runtime.RuntimeName, hasXSIType bool, line, col int) (acceptedChild, error) {
	if s.schema != nil {
		return s.acceptSchemaChild(parent, rn, hasXSIType, line, col)
	}
	input := ChildInput{
		HasSchemaLocation: s.schemaLocationHintLookup(),
		Context:           s.startContext(line, col),
		Scratch:           s.contentScratch(parent),
		Name:              rn,
		ParentChild:       parent.Child,
		ParentContent:     parent.Content,
		ParentType:        parent.Type,
		HasXSIType:        hasXSIType,
		HasParentChild:    parent.ChildOK,
		ParentSkip:        parent.Skip,
		ParentNilled:      parent.Nilled,
	}
	var child ChildResult
	var err error
	child, err = ChildStart(s.rt, input)
	if err != nil {
		return acceptedChild{
			element: child.Element,
			typ:     child.Type,
			skip:    child.Skip,
			recover: child.Recover,
		}, err
	}
	parent.Content = child.Content
	return acceptedChild{element: child.Element, typ: child.Type, skip: child.Skip}, nil
}

func (s *session) acceptSchemaChild(parent *frame, rn runtime.RuntimeName, hasXSIType bool, line, col int) (acceptedChild, error) {
	if parent.Skip {
		return s.schemaSkippedChild(), nil
	}
	if parent.Nilled {
		return s.recoverableSchemaChild(s.startContext(line, col), xsderrors.CodeValidationNil, "nilled element must be empty")
	}
	parentContent := parent.Child
	if !parent.ChildOK {
		var ok bool
		parentContent, ok = s.schemaChildContentInfo(parent.Type)
		if !ok {
			return acceptedChild{}, xsderrors.InternalInvariant("child content metadata is invalid")
		}
	}
	if !parentContent.Complex {
		return s.recoverableSchemaChild(s.startContext(line, col), xsderrors.CodeValidationContent, "simple type cannot contain child elements")
	}
	if parentContent.Simple {
		return s.recoverableSchemaChild(s.startContext(line, col), xsderrors.CodeValidationContent, "simple content cannot contain child elements")
	}
	if !parent.Content.HasModel() {
		return s.recoverableSchemaChild(s.startContext(line, col), xsderrors.CodeValidationElement, "unexpected child element "+rn.Label())
	}
	st := parent.Content
	scratch := s.contentScratch(parent)
	match, ok, valid := s.schema.AdvancePublishedSchemaContent(&st, runtime.ContentInput{
		Name:       rn,
		HasXSIType: hasXSIType,
	}, &scratch)
	if !valid {
		return acceptedChild{}, xsderrors.InternalInvariant("content model state is invalid")
	}
	if !ok {
		return s.recoverableSchemaChild(s.startContext(line, col), xsderrors.CodeValidationElement, "unexpected child element "+rn.Label())
	}
	if match.StrictMissing {
		ctx := s.startContext(line, col)
		if hasSchemaLocation := s.schemaLocationHintLookup(); hasSchemaLocation != nil && hasSchemaLocation(rn.NS) {
			return acceptedChild{}, unsupportedSchemaLocation(ctx, vocab.XSDElemElement, rn)
		}
		return s.recoverableSchemaChild(ctx, xsderrors.CodeValidationElement, "wildcard requires declared element "+rn.Label())
	}
	parent.Content = st
	if match.Element == runtime.NoElement {
		return acceptedChild{element: runtime.NoElement, typ: s.schema.AnyType(), skip: match.Skip}, nil
	}
	if !runtime.ValidElementID(match.Element, len(s.schema.ElementStartInfos)) {
		return acceptedChild{}, xsderrors.InternalInvariant("content model matched invalid element declaration")
	}
	typ := s.schema.ElementStartInfos[match.Element].Type
	return acceptedChild{element: match.Element, typ: typ, skip: match.Skip}, nil
}

func (s *session) schemaSkippedChild() acceptedChild {
	return acceptedChild{element: runtime.NoElement, typ: s.schema.AnyType(), skip: true}
}

func (s *session) recoverableSchemaChild(ctx StartContext, code xsderrors.Code, msg string) (acceptedChild, error) {
	out := s.schemaSkippedChild()
	out.recover = true
	return out, validation(ctx, code, msg)
}

func (s *session) end(line, col int, ee xml.EndElement) error {
	if len(s.doc.stack) == 0 {
		return ValidateEndElement(EndElementInput{
			Context:      s.startContext(line, col),
			OpenElements: len(s.doc.stack),
		})
	}
	translated, err := s.translateName(ee.Name, xmlElementName, line, col)
	if err != nil {
		return err
	}
	ee.Name = translated
	expected := s.doc.elementNames[len(s.doc.elementNames)-1]
	if ee.Name != expected {
		return validation(s.startContext(line, col), xsderrors.CodeValidationXML,
			"end element </"+formatXMLName(ee.Name)+"> does not match start element <"+formatXMLName(expected)+">")
	}
	f := &s.doc.stack[len(s.doc.stack)-1]
	stop := s.validateFrameEnd(f, line, col)
	if stop == nil && s.hasIdentityConstraints {
		stop = s.finishFrameIdentity(line, col)
	}
	s.doc.allBits = s.doc.allBits[:f.BitBase]
	s.doc.text = s.doc.text[:f.TextStart]
	s.doc.stack = s.doc.stack[:len(s.doc.stack)-1]
	s.popPath()
	if s.hasIdentityConstraints && len(s.doc.namePath) > 0 {
		s.doc.namePath = s.doc.namePath[:len(s.doc.namePath)-1]
	}
	if len(s.doc.elementNames) > 0 {
		s.doc.elementNames = s.doc.elementNames[:len(s.doc.elementNames)-1]
	}
	s.doc.ns.Pop()
	return stop
}

func (s *session) validateFrameEnd(f *frame, line, col int) error {
	if f.Skip {
		return nil
	}
	if f.Nilled && (f.HasChild || f.HasText) {
		err := validation(s.startContext(line, col), xsderrors.CodeValidationNil, "nilled element must be empty")
		if recoverErr := s.recover(err); recoverErr != nil {
			return recoverErr
		}
	}
	if !f.Nilled {
		if err := s.completeFrame(f, line, col); err != nil {
			if recoverErr := s.recover(err); recoverErr != nil {
				return recoverErr
			}
		}
	}
	if !s.hasIdentityConstraints &&
		f.SimpleContentKnown && !f.HasSimpleContent &&
		f.ElementValueKnown && !f.ElementHasValueConstraint {
		return nil
	}
	contentCaptured, err := s.validateSimpleContent(f, line, col)
	if err != nil {
		return s.recover(err)
	}
	if !s.hasIdentityConstraints {
		return nil
	}
	return s.captureEndIdentity(f, contentCaptured, line, col)
}

func (s *session) captureEndIdentity(f *frame, contentCaptured bool, line, col int) error {
	action, err := EndIdentityCapture(s.rt, EndIdentityInput{
		Type:            f.Type,
		Element:         f.Element,
		ContentCaptured: contentCaptured,
		Nilled:          f.Nilled,
	})
	if err != nil {
		return err
	}
	switch action {
	case EndIdentityCaptureNone:
		return nil
	case EndIdentityCaptureNilledElement:
		fields, err := s.identityElementFields()
		if err != nil {
			return s.recover(err)
		}
		return s.recover(s.captureIdentityFieldKey(fields, NilledElementIdentityKey(), line, col))
	case EndIdentityCaptureComplexElement:
		return s.recover(s.captureIdentityComplexElement(s.doc.text[f.TextStart:], line, col))
	default:
		return xsderrors.InternalInvariant("unknown end identity capture action")
	}
}

func (s *session) finishFrameIdentity(line, col int) error {
	if err := s.finishIdentitySelections(len(s.doc.namePath), line, col); err != nil {
		return err
	}
	return s.closeIdentityScopes(len(s.doc.namePath))
}

func (s *session) completeFrame(f *frame, line, col int) error {
	if s.schema != nil {
		if f.Nilled || f.Type.Kind != runtime.TypeComplex || !f.Content.HasModel() {
			return nil
		}
		scratch := s.contentScratch(f)
		complete, ok := s.schema.CompletePublishedSchemaContent(f.Content, &scratch)
		if !ok {
			return xsderrors.InternalInvariant("content model state is invalid")
		}
		if complete {
			return nil
		}
		return validation(s.startContext(line, col), xsderrors.CodeValidationContent, "missing required child element")
	}
	input := CompleteInput{
		Context: s.startContext(line, col),
		Scratch: s.contentScratch(f),
		Type:    f.Type,
		Content: f.Content,
		Nilled:  f.Nilled,
	}
	return ContentComplete(s.rt, input)
}

func (s *session) contentScratch(f *frame) runtime.ContentScratch {
	return runtime.NewContentScratch(s.doc.allBits, f.BitBase, f.BitLen)
}
