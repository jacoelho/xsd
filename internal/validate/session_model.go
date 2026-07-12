package validate

import (
	"errors"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stream"
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
	return s.acceptPublishedSchemaChild(parent, rn, hasXSIType, line, col)
}

func (s *session) acceptPublishedSchemaChild(parent *frame, rn runtime.RuntimeName, hasXSIType bool, line, col int) (acceptedChild, error) {
	policy := childFramePolicy(parent.Skip, parent.Nilled)
	if policy.skip {
		return s.publishedSchemaSkippedChild(), nil
	}
	if policy.issue.valid() {
		return s.recoverablePublishedSchemaChildIssue(line, col, policy.issue)
	}
	parentContent := parent.Child
	if !parent.ChildOK {
		var ok bool
		parentContent, ok = s.schemaChildContentInfo(parent.Type)
		if !ok {
			return acceptedChild{}, xsderrors.InternalInvariant("child content metadata is invalid")
		}
	}
	if issue := childContentPolicy(parentContent, parent.Content, rn); issue.valid() {
		return s.recoverablePublishedSchemaChildIssue(line, col, issue)
	}
	st := parent.Content
	scratch := s.contentScratch(parent)
	match, status := s.rt.AdvanceContent(&st, runtime.ContentInput{
		Name:       rn,
		HasXSIType: hasXSIType,
	}, &scratch)
	if status == runtime.ContentAdvanceInvalid {
		return acceptedChild{}, xsderrors.InternalInvariant("content model state is invalid")
	}
	if status == runtime.ContentAdvanceNoMatch {
		return s.recoverablePublishedSchemaChildIssue(line, col, unexpectedChildIssue(rn))
	}
	if match.StrictMissing {
		if hasSchemaLocation := s.schemaLocationHintLookup(); hasSchemaLocation != nil && hasSchemaLocation(rn.NS) {
			return acceptedChild{}, unsupportedSchemaLocation(s.startContext(line, col), vocab.XSDElemElement, rn)
		}
		parent.Content = st
		return s.recoverablePublishedSchemaChildIssue(line, col, strictMissingChildIssue(rn))
	}
	parent.Content = st
	if match.Element == runtime.NoElement {
		return acceptedChild{element: runtime.NoElement, typ: s.rt.AnyType(), skip: match.Skip}, nil
	}
	decl, declared := s.rt.Element(match.Element)
	if !declared {
		return acceptedChild{}, xsderrors.InternalInvariant("content model matched invalid element declaration")
	}
	return acceptedChild{element: match.Element, typ: decl.Type, skip: match.Skip}, nil
}

func (s *session) publishedSchemaSkippedChild() acceptedChild {
	return acceptedChild{element: runtime.NoElement, typ: s.rt.AnyType(), skip: true}
}

func (s *session) recoverablePublishedSchemaChildIssue(line, col int, issue validationIssue) (acceptedChild, error) {
	return s.recoverablePublishedSchemaChildIssueAt(s.startContext(line, col), issue)
}

func (s *session) recoverablePublishedSchemaChildIssueAt(ctx StartContext, issue validationIssue) (acceptedChild, error) {
	out := s.publishedSchemaSkippedChild()
	out.recover = true
	return out, validationFromIssue(ctx, issue)
}

func (s *session) end(line, col int, ee stream.EndElement) error {
	if err := s.doc.ValidateEnd(ee, line, col); err != nil {
		return err
	}
	if s.doc.syntaxOnly {
		return s.doc.CommitEnd()
	}
	f, ok := s.doc.Current()
	if !ok {
		return xsderrors.InternalInvariant("end element has no schema frame")
	}
	stop := s.validateFrameEnd(f, line, col)
	if errors.Is(stop, errSemanticStop) {
		stop = nil
	} else if stop == nil && s.hasIdentityConstraints {
		stop = s.finishFrameIdentity(line, col)
		if errors.Is(stop, errSemanticStop) {
			stop = nil
		}
	}
	s.doc.allBits = s.doc.allBits[:f.BitBase]
	s.doc.text = s.doc.text[:f.TextStart]
	if s.hasIdentityConstraints && len(s.doc.namePath) > 0 {
		nameIndex := len(s.doc.namePath) - 1
		s.doc.namePath[nameIndex] = runtime.RuntimeName{}
		s.doc.namePath = s.doc.namePath[:nameIndex]
	}
	if err := s.doc.CommitEnd(); err != nil {
		return err
	}
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
	return s.completePublishedSchemaFrame(f, line, col)
}

func (s *session) completePublishedSchemaFrame(f *frame, line, col int) error {
	if !contentCompletionRequired(f.Nilled, f.Type, f.Content) {
		return nil
	}
	scratch := s.contentScratch(f)
	status := s.rt.CompleteContent(f.Content, &scratch)
	if status == runtime.ContentCompletionInvalid {
		return xsderrors.InternalInvariant("content model state is invalid")
	}
	if status == runtime.ContentCompletionComplete {
		return nil
	}
	return validationFromIssue(s.startContext(line, col), missingRequiredChildIssue())
}

func (s *session) contentScratch(f *frame) runtime.ContentScratch {
	return runtime.NewContentScratch(s.doc.allBits, f.BitBase, f.BitLen)
}
