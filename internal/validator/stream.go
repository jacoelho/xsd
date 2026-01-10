package validator

import (
	"fmt"
	"io"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

type streamContentKind int

const (
	streamContentNone streamContentKind = iota
	streamContentAutomaton
	streamContentAllGroup
	streamContentAny
	streamContentEmpty
	streamContentRejectAll
)

type streamFrame struct {
	id                 uint64
	qname              types.QName
	decl               *grammar.CompiledElement
	typ                *grammar.CompiledType
	textType           *grammar.CompiledType
	contentModel       *grammar.CompiledContentModel
	automaton          *grammar.AutomatonStreamValidator
	allGroup           *grammar.AllGroupStreamValidator
	textBuf            []byte
	fieldCaptures      []fieldCapture
	minOccurs          int
	scopeDepth         int
	contentKind        streamContentKind
	hasChildElements   bool
	nilled             bool
	skipChildren       bool
	invalid            bool
	collectStringValue bool
}

type streamRun struct {
	*validationRun
	dec             *xsdxml.StreamDecoder
	frames          []streamFrame
	violations      []errors.Validation
	rootSeen        bool
	rootClosed      bool
	identityScopes  []*identityScope
	constraintDecls map[types.QName][]*grammar.CompiledElement
}

// ValidateStream validates an XML document using streaming schemacheck.
func (v *Validator) ValidateStream(r io.Reader) ([]errors.Validation, error) {
	if v == nil || v.grammar == nil {
		return []errors.Validation{errors.NewValidation(errors.ErrSchemaNotLoaded, "schema not loaded", "")}, nil
	}
	if r == nil {
		return nil, fmt.Errorf("nil reader")
	}

	run := v.newStreamRun()

	dec, err := xsdxml.NewStreamDecoder(r)
	if err != nil {
		return nil, err
	}

	return run.validate(dec)
}

func (v *Validator) newStreamRun() *streamRun {
	base := &validationRun{
		validator: v,
		schema:    v.baseView,
	}
	return &streamRun{
		validationRun: base,
	}
}

func (v *Validator) acquireAutomatonStreamValidator(a *grammar.Automaton, matcher grammar.SymbolMatcher, wildcards []*types.AnyElement) *grammar.AutomatonStreamValidator {
	if v == nil || a == nil {
		return nil
	}
	pooled, _ := v.automatonValidatorPool.Get().(*grammar.AutomatonStreamValidator)
	if pooled == nil {
		pooled = &grammar.AutomatonStreamValidator{}
	}
	pooled.Reset(a, matcher, wildcards)
	return pooled
}

func (v *Validator) releaseAutomatonStreamValidator(stream *grammar.AutomatonStreamValidator) {
	if v == nil || stream == nil {
		return
	}
	stream.Release()
	v.automatonValidatorPool.Put(stream)
}

func (r *streamRun) newAutomatonValidator(a *grammar.Automaton, wildcards []*types.AnyElement) *grammar.AutomatonStreamValidator {
	if r == nil || a == nil {
		return nil
	}
	if r.validator == nil {
		return a.NewStreamValidator(r.matcher(), wildcards)
	}
	return r.validator.acquireAutomatonStreamValidator(a, r.matcher(), wildcards)
}

func (r *streamRun) releaseFrameResources(frame *streamFrame) {
	if frame == nil || r == nil || r.validator == nil {
		return
	}
	if frame.automaton != nil {
		r.validator.releaseAutomatonStreamValidator(frame.automaton)
		frame.automaton = nil
	}
}

func (r *streamRun) validate(dec *xsdxml.StreamDecoder) ([]errors.Validation, error) {
	r.reset()
	r.dec = dec
	r.frames = r.frames[:0]
	r.violations = r.violations[:0]
	r.rootSeen = false
	r.rootClosed = false
	r.identityScopes = r.identityScopes[:0]
	r.constraintDecls = nil

	for {
		ev, err := dec.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return r.violations, err
		}

		switch ev.Kind {
		case xsdxml.EventStartElement:
			if r.rootClosed {
				return r.violations, fmt.Errorf("unexpected element %s after document end", ev.Name.Local)
			}
			if !r.rootSeen {
				r.rootSeen = true
			}
			if err := r.handleStart(dec, ev); err != nil {
				return r.violations, err
			}

		case xsdxml.EventEndElement:
			if len(r.frames) == 0 {
				return r.violations, fmt.Errorf("unexpected end element %s", ev.Name.Local)
			}
			if err := r.handleEnd(ev); err != nil {
				return r.violations, err
			}
			if len(r.frames) == 0 && r.rootSeen {
				r.rootClosed = true
			}

		case xsdxml.EventCharData:
			if len(r.frames) == 0 {
				if !isIgnorableOutsideRoot(ev.Text) {
					return r.violations, fmt.Errorf("unexpected character data outside root element")
				}
				continue
			}
			r.handleCharData(ev)
		}
	}

	if !r.rootSeen {
		return r.violations, fmt.Errorf("document has no root element")
	}
	if len(r.frames) != 0 {
		return r.violations, io.ErrUnexpectedEOF
	}

	r.violations = append(r.violations, r.checkIDRefs()...)
	return r.violations, nil
}

func (r *streamRun) handleStart(dec *xsdxml.StreamDecoder, ev xsdxml.Event) error {
	parent := r.currentFrame()
	if parent != nil {
		parent.hasChildElements = true
		if parent.invalid || parent.skipChildren {
			return dec.SkipSubtree()
		}
		if parent.nilled {
			r.addViolation(errors.NewValidation(errors.ErrNilElementNotEmpty,
				"Element with xsi:nil='true' must be empty", r.path.String()))
			parent.invalid = true
			return dec.SkipSubtree()
		}
		if parent.decl != nil && parent.decl.HasFixed {
			r.addViolation(errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
				"Element '%s' has a fixed value constraint and cannot have element children", parent.decl.QName.Local))
			parent.invalid = true
			return dec.SkipSubtree()
		}
		if parent.textType != nil && (parent.typ == nil || (!isAnyType(parent.typ) && !parent.typ.HasContentModel())) {
			r.path.push(ev.Name.Local)
			r.addViolation(errors.NewValidationf(errors.ErrTextInElementOnly, r.path.String(),
				"Element '%s' is not allowed in simple content", ev.Name.Local))
			r.path.pop()
			parent.invalid = true
			return dec.SkipSubtree()
		}
		if parent.contentKind == streamContentEmpty {
			r.path.push(ev.Name.Local)
			r.addViolation(errors.NewValidationf(errors.ErrUnexpectedElement, r.path.String(),
				"Element '%s' is not allowed. No element declaration found for it in the empty content model.", ev.Name.Local))
			r.path.pop()
			parent.invalid = true
			return dec.SkipSubtree()
		}
		if parent.contentKind == streamContentRejectAll {
			r.addViolation(errors.NewValidation(errors.ErrUnexpectedElement, "element not allowed by empty choice", r.path.String()))
			parent.invalid = true
			return dec.SkipSubtree()
		}
	}

	processContents := types.Strict
	var matchedQName types.QName
	var matchedDecl *grammar.CompiledElement
	fromWildcard := false

	if parent != nil {
		switch parent.contentKind {
		case streamContentAny:
			processContents = types.Lax
		case streamContentAutomaton:
			match, err := parent.automaton.Feed(ev.Name)
			if err != nil {
				r.addContentModelError(err)
				parent.invalid = true
				return dec.SkipSubtree()
			}
			if match.IsWildcard {
				processContents = match.ProcessContents
				fromWildcard = true
			}
			matchedQName = match.MatchedQName
			if match.MatchedElement != nil {
				matchedDecl = match.MatchedElement
			}
		case streamContentAllGroup:
			match, err := parent.allGroup.Feed(ev.Name)
			if err != nil {
				r.addContentModelError(err)
				parent.invalid = true
				return dec.SkipSubtree()
			}
			matchedQName = match.MatchedQName
			if match.MatchedElement != nil {
				matchedDecl = match.MatchedElement
			}
		}
	}

	if processContents == types.Skip {
		return dec.SkipSubtree()
	}

	r.path.push(ev.Name.Local)
	frame, skipSubtree := r.startFrame(ev, parent, processContents, matchedDecl, matchedQName, fromWildcard)
	if skipSubtree {
		r.path.pop()
		return dec.SkipSubtree()
	}
	r.frames = append(r.frames, frame)
	r.handleIdentityStart(&r.frames[len(r.frames)-1], ev.Attrs)
	return nil
}

func (r *streamRun) handleCharData(ev xsdxml.Event) {
	frame := r.currentFrame()
	if frame == nil || frame.invalid {
		return
	}

	if frame.nilled {
		if !isWhitespaceOnly(ev.Text) {
			r.addViolation(errors.NewValidation(errors.ErrNilElementNotEmpty,
				"Element with xsi:nil='true' must be empty", r.path.String()))
			frame.invalid = true
		}
		return
	}

	if frame.typ != nil && !frame.typ.AllowsText() && !isWhitespaceOnlyBytes(ev.Text) {
		r.addViolation(errors.NewValidation(errors.ErrTextInElementOnly,
			"Element content cannot have character children (non-whitespace text found)", r.path.String()))
		frame.invalid = true
		return
	}

	if frame.collectStringValue {
		frame.textBuf = append(frame.textBuf, ev.Text...)
	}
}

func (r *streamRun) handleEnd(ev xsdxml.Event) error {
	frame := r.popFrame()
	if frame == nil {
		return nil
	}
	defer r.path.pop()
	defer r.releaseFrameResources(frame)
	defer r.handleIdentityEnd(frame)

	if frame.invalid {
		r.propagateText(frame)
		return nil
	}

	if frame.nilled {
		if frame.hasChildElements {
			r.addViolation(errors.NewValidation(errors.ErrNilElementNotEmpty,
				"Element with xsi:nil='true' must be empty", r.path.String()))
		}
		r.propagateText(frame)
		return nil
	}

	if frame.skipChildren {
		r.propagateText(frame)
		return nil
	}

	if frame.textType != nil {
		text := string(frame.textBuf)
		hadContent := text != ""
		if text == "" && frame.decl != nil {
			if frame.decl.Default != "" {
				text = frame.decl.Default
			} else if frame.decl.HasFixed {
				text = frame.decl.Fixed
			}
		}
		r.violations = append(r.violations, r.checkSimpleValue(text, frame.textType, frame.scopeDepth)...)
		r.violations = append(r.violations, r.collectIDRefs(text, frame.textType)...)
		if frame.typ != nil && frame.typ.Kind == grammar.TypeKindComplex && len(frame.typ.Facets) > 0 {
			r.violations = append(r.violations, r.checkComplexTypeFacets(text, frame.typ)...)
		}
		if frame.decl != nil && frame.decl.HasFixed && hadContent {
			r.violations = append(r.violations, r.checkFixedValue(text, frame.decl.Fixed, frame.textType)...)
		}
		r.propagateText(frame)
		return nil
	}

	switch frame.contentKind {
	case streamContentAutomaton:
		if err := frame.automaton.Close(); err != nil {
			r.addContentModelError(err)
		}
	case streamContentAllGroup:
		if err := frame.allGroup.Close(); err != nil {
			r.addContentModelError(err)
		}
	case streamContentRejectAll:
		if frame.minOccurs > 0 && !frame.hasChildElements {
			r.addViolation(errors.NewValidation(errors.ErrRequiredElementMissing,
				"content does not satisfy empty choice", r.path.String()))
		}
	}

	if frame.decl != nil && frame.decl.HasFixed {
		text := string(frame.textBuf)
		if text == "" && !frame.hasChildElements {
			text = frame.decl.Fixed
		}
		r.violations = append(r.violations, r.checkFixedValue(text, frame.decl.Fixed, textTypeForFixedValue(frame.decl))...)
	}

	r.propagateText(frame)
	return nil
}

func (r *streamRun) startFrame(ev xsdxml.Event, parent *streamFrame, processContents types.ProcessContents, matchedDecl *grammar.CompiledElement, matchedQName types.QName, fromWildcard bool) (streamFrame, bool) {
	attrs := newAttributeIndex(ev.Attrs)
	decl := r.resolveMatchedDecl(parent, ev.Name, matchedDecl, matchedQName)

	missingCode := errors.ErrElementNotDeclared
	if fromWildcard && processContents == types.Strict {
		missingCode = errors.ErrWildcardNotDeclared
	}

	xsiTypeValue, hasXsiType := attrs.Value(xsdxml.XSINamespace, "type")

	if decl == nil {
		if hasXsiType {
			xsiType, err := r.resolveXsiTypeOnly(ev.ScopeDepth, xsiTypeValue)
			if err == nil && xsiType != nil {
				frame := r.newFrame(ev, nil, xsiType, parent)
				return frame, false
			}
		}
		if processContents == types.Strict {
			r.addMissingDeclViolation(ev.Name.Local, missingCode)
			return streamFrame{}, true
		}
		if processContents == types.Skip {
			return streamFrame{}, true
		}
		anyType := r.validator.getBuiltinCompiledType(types.GetBuiltin(types.TypeNameAnyType))
		frame := r.newFrame(ev, nil, anyType, parent)
		return frame, false
	}

	if decl.Abstract {
		r.addViolation(errors.NewValidationf(errors.ErrElementAbstract, r.path.String(),
			"Element '%s' is abstract and cannot be used directly in instance documents", ev.Name.Local))
		return streamFrame{}, true
	}

	nilValue, hasNil := attrs.Value(xsdxml.XSINamespace, "nil")
	isNil := hasNil && (nilValue == "true" || nilValue == "1")
	if hasNil && !decl.Nillable {
		r.addViolation(errors.NewValidationf(errors.ErrElementNotNillable, r.path.String(),
			"Element '%s' is not nillable but has xsi:nil attribute", ev.Name.Local))
		return streamFrame{}, true
	}

	if decl.Type == nil {
		frame := r.newFrame(ev, decl, nil, parent)
		frame.nilled = isNil
		frame.skipChildren = true
		return frame, false
	}

	effectiveType := decl.Type
	if hasXsiType {
		xsiType, err := r.resolveXsiType(ev.ScopeDepth, xsiTypeValue, decl.Type, decl.Block)
		if err != nil {
			r.addViolation(errors.NewValidation(errors.ErrXsiTypeInvalid, err.Error(), r.path.String()))
			return streamFrame{}, true
		}
		if xsiType != nil {
			effectiveType = xsiType
		}
	}

	if effectiveType.Abstract {
		r.addViolation(errors.NewValidationf(errors.ErrElementTypeAbstract, r.path.String(),
			"Type '%s' is abstract and cannot be used for element '%s'", effectiveType.QName.String(), ev.Name.Local))
		return streamFrame{}, true
	}

	if isNil {
		if decl.HasFixed {
			r.addViolation(errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
				"Element '%s' has a fixed value constraint and cannot be nil", ev.Name.Local))
			return streamFrame{}, true
		}
		violations := r.checkAttributesStream(attrs, effectiveType.AllAttributes, effectiveType.AnyAttribute, ev.ScopeDepth)
		if len(violations) > 0 {
			r.violations = append(r.violations, violations...)
			return streamFrame{}, true
		}
		frame := r.newFrame(ev, decl, effectiveType, parent)
		frame.nilled = true
		frame.skipChildren = true
		return frame, false
	}

	attrsToCheck := effectiveType.AllAttributes
	anyAttr := effectiveType.AnyAttribute
	if isAnyType(effectiveType) {
		attrsToCheck = nil
		anyAttr = &types.AnyAttribute{
			Namespace:       types.NSCAny,
			ProcessContents: types.Lax,
			TargetNamespace: types.NamespaceEmpty,
		}
	}

	violations := r.checkAttributesStream(attrs, attrsToCheck, anyAttr, ev.ScopeDepth)
	if len(violations) > 0 {
		r.violations = append(r.violations, violations...)
		return streamFrame{}, true
	}

	frame := r.newFrame(ev, decl, effectiveType, parent)
	return frame, false
}

func (r *streamRun) newFrame(ev xsdxml.Event, decl *grammar.CompiledElement, ct *grammar.CompiledType, parent *streamFrame) streamFrame {
	frame := streamFrame{
		id:         uint64(ev.ID),
		qname:      ev.Name,
		decl:       decl,
		typ:        ct,
		scopeDepth: ev.ScopeDepth,
	}

	if ct != nil {
		frame.textType = ct.TextType()
		frame.contentModel = ct.ContentModel
		if isAnyType(ct) {
			frame.contentKind = streamContentAny
		} else if ct.ContentModel != nil {
			switch {
			case ct.ContentModel.RejectAll:
				frame.contentKind = streamContentRejectAll
				frame.minOccurs = ct.ContentModel.MinOccurs
			case ct.ContentModel.AllElements != nil:
				frame.contentKind = streamContentAllGroup
				allElements := make([]grammar.AllGroupElementInfo, len(ct.ContentModel.AllElements))
				for i := range ct.ContentModel.AllElements {
					allElements[i] = ct.ContentModel.AllElements[i]
				}
				frame.allGroup = grammar.NewAllGroupValidator(allElements, ct.ContentModel.Mixed, ct.ContentModel.MinOccurs).NewStreamValidator(r.matcher())
			case ct.ContentModel.Automaton != nil:
				frame.contentKind = streamContentAutomaton
				frame.automaton = r.newAutomatonValidator(ct.ContentModel.Automaton, ct.ContentModel.Wildcards())
			case ct.ContentModel.Empty:
				frame.contentKind = streamContentEmpty
			}
		}
	}

	frame.collectStringValue = frameNeedsStringValue(frame, parent)
	return frame
}

func frameNeedsStringValue(frame streamFrame, parent *streamFrame) bool {
	if parent != nil && parent.collectStringValue {
		return true
	}
	if frame.textType != nil {
		return true
	}
	if frame.decl != nil && frame.decl.HasFixed {
		return true
	}
	return false
}

func (r *streamRun) resolveMatchedDecl(parent *streamFrame, actual types.QName, matchedDecl *grammar.CompiledElement, matchedQName types.QName) *grammar.CompiledElement {
	if matchedDecl != nil {
		return r.resolveSubstitutionDecl(actual, matchedDecl)
	}
	if !matchedQName.IsZero() && parent != nil && parent.contentModel != nil && parent.contentModel.ElementIndex != nil {
		if decl := parent.contentModel.ElementIndex[matchedQName]; decl != nil {
			return r.resolveSubstitutionDecl(actual, decl)
		}
	}
	if decl := r.findElementDeclaration(actual); decl != nil {
		return decl
	}
	return nil
}

func (r *streamRun) addMissingDeclViolation(local string, code errors.ErrorCode) {
	if code == errors.ErrWildcardNotDeclared {
		r.addViolation(errors.NewValidationf(code, r.path.String(),
			"Element '%s' is not declared (strict wildcard requires declaration)", local))
		return
	}
	r.addViolation(errors.NewValidationf(code, r.path.String(),
		"Cannot find declaration for element '%s'", local))
}

func (r *streamRun) addContentModelError(err error) {
	if ve, ok := err.(*grammar.ValidationError); ok {
		r.addViolation(errors.NewValidation(errors.ErrorCode(ve.FullCode()), ve.Message, r.path.String()))
	}
}

func (r *streamRun) addViolation(v errors.Validation) {
	r.violations = append(r.violations, v)
}

func (r *streamRun) currentFrame() *streamFrame {
	if len(r.frames) == 0 {
		return nil
	}
	return &r.frames[len(r.frames)-1]
}

func (r *streamRun) popFrame() *streamFrame {
	if len(r.frames) == 0 {
		return nil
	}
	frame := &r.frames[len(r.frames)-1]
	r.frames = r.frames[:len(r.frames)-1]
	return frame
}

func (r *streamRun) propagateText(frame *streamFrame) {
	if frame == nil || !frame.collectStringValue {
		return
	}
	parent := r.currentFrame()
	if parent == nil || !parent.collectStringValue {
		return
	}
	parent.textBuf = append(parent.textBuf, frame.textBuf...)
}

func (r *streamRun) childPath(local string) string {
	if local == "" {
		return r.path.String()
	}
	current := r.path.String()
	if current == "/" {
		return "/" + local
	}
	return current + "/" + local
}

func isIgnorableOutsideRoot(data []byte) bool {
	for _, r := range string(data) {
		if r == '\uFEFF' {
			continue
		}
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
