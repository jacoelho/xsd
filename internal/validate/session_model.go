package validate

import (
	"errors"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

type acceptedChild struct {
	start             schemaStart
	transition        runtime.ContentTransition
	recover           bool
	advances          bool
	invalidatesParent bool
}

func (s *session) acceptChild(parent *frame, rn runtime.RuntimeName, hasXSIType bool, line, col int) (acceptedChild, error) {
	return s.acceptPublishedSchemaChild(parent, rn, hasXSIType, line, col)
}

func (s *session) acceptPublishedSchemaChild(parent *frame, rn runtime.RuntimeName, hasXSIType bool, line, col int) (acceptedChild, error) {
	if parent.Mode != elementAssessed {
		return acceptedChild{start: schemaStart{element: runtime.NoElement, mode: parent.Mode}}, nil
	}
	policy := childFramePolicy(parent.Nilled)
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
	scratch := s.contentScratch(parent)
	transition, status := s.rt.NextContent(parent.Content, runtime.ContentInput{
		Name:       rn,
		HasXSIType: hasXSIType,
	}, &scratch)
	if status == runtime.ContentTransitionInvalid {
		return acceptedChild{}, xsderrors.InternalInvariant("content model state is invalid")
	}
	if status == runtime.ContentTransitionNoMatch {
		return s.recoverablePublishedSchemaChildIssue(line, col, unexpectedChildIssue(rn))
	}
	match := transition.Match()
	if match.StrictMissing {
		if hasSchemaLocation := s.schemaLocationHintLookup(); hasSchemaLocation != nil && hasSchemaLocation(rn.NS) {
			return acceptedChild{}, unsupportedSchemaLocation(s.startContext(line, col), vocab.XSDElemElement, rn)
		}
		accepted, err := s.recoverablePublishedSchemaChildIssue(line, col, strictMissingChildIssue(rn))
		accepted.transition = transition
		accepted.advances = true
		return accepted, err
	}
	if match.Element == runtime.NoElement {
		if match.Skip {
			return acceptedChild{start: wildcardSkippedSchemaStart(), transition: transition, advances: true}, nil
		}
		return acceptedChild{start: assessedSchemaStart(runtime.NoElement, s.rt.AnyType()), transition: transition, advances: true}, nil
	}
	decl, declared := s.rt.Element(match.Element)
	if !declared {
		return acceptedChild{}, xsderrors.InternalInvariant("content model matched invalid element declaration")
	}
	return acceptedChild{start: assessedSchemaStart(match.Element, decl.Type), transition: transition, advances: true}, nil
}

func (s *session) recoverablePublishedSchemaChildIssue(line, col int, issue validationIssue) (acceptedChild, error) {
	return s.recoverablePublishedSchemaChildIssueAt(s.startContext(line, col), issue)
}

func (s *session) recoverablePublishedSchemaChildIssueAt(ctx StartContext, issue validationIssue) (acceptedChild, error) {
	out := acceptedChild{start: recoverySchemaStart()}
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
	contentCaptured, stop := s.validateFrameEnd(f, line, col)
	if errors.Is(stop, errSemanticStop) {
		stop = nil
	} else if stop == nil {
		result, identityErr := s.doc.identity.endElement(identityElementEnd{
			Context:           s.startContext(line, col),
			ContentCaptured:   contentCaptured,
			AssessmentInvalid: f.AssessmentInvalid,
		}, s.recover)
		f.AssessmentInvalid = result.AssessmentInvalid
		stop = identityErr
		if errors.Is(stop, errSemanticStop) {
			stop = nil
		}
	}
	s.doc.allBits = s.doc.allBits[:f.BitBase]
	s.doc.text = s.doc.text[:f.TextStart]
	if err := s.doc.CommitEnd(); err != nil {
		return err
	}
	return stop
}

func (s *session) validateFrameEnd(f *frame, line, col int) (bool, error) {
	switch f.Mode {
	case elementWildcardSkipped, elementRecovery:
		return false, nil
	case elementAssessed:
	default:
		return false, xsderrors.InternalInvariant("element assessment mode is invalid")
	}
	if !f.Nilled {
		if err := s.completeFrame(f, line, col); err != nil {
			if recoverErr := s.recoverAssessment(err); recoverErr != nil {
				return false, recoverErr
			}
		}
	}
	if !s.doc.identity.hasConstraints() &&
		f.SimpleContentKnown && !f.HasSimpleContent &&
		f.ElementValueKnown && !f.ElementHasValueConstraint {
		return false, nil
	}
	contentCaptured, err := s.validateSimpleContent(f, line, col)
	if err != nil {
		return false, s.recoverAssessment(err)
	}
	return contentCaptured, nil
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
