package validator

import (
	stderrors "errors"
	"fmt"
	"io"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	xsdxml "github.com/jacoelho/xsd/internal/xml"
	"github.com/jacoelho/xsd/pkg/xmlstream"
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

type matchOrigin int

const (
	matchFromDeclaration matchOrigin = iota
	matchFromWildcard
)

type listItemMode int

const (
	listItemModeString listItemMode = iota
	listItemModeBytes
)

type streamStart struct {
	Name       types.QName
	Attrs      []streamAttr
	Line       int
	Column     int
	ID         uint64
	ScopeDepth int
}

type streamAttr = xmlstream.StringAttr

// streamFrame groups per-element validation state kept on the streaming stack.
// it keeps declaration/type info, content-model validators, and text/list buffers
// together to avoid extra allocations during streaming validation.
type streamFrame struct {
	allGroup           *grammar.AllGroupStreamValidator
	decl               *grammar.CompiledElement
	typ                *grammar.CompiledType
	textType           *grammar.CompiledType
	contentModel       *grammar.CompiledContentModel
	automaton          *grammar.AutomatonStreamValidator
	qname              types.QName
	fieldCaptures      []fieldCapture
	textBuf            []byte
	minOccurs          types.Occurs
	listStream         listStreamState
	id                 uint64
	textColumn         int
	textLine           int
	startColumn        int
	startLine          int
	scopeDepth         int
	contentKind        streamContentKind
	hasChildElements   bool
	nilled             bool
	skipChildren       bool
	invalid            bool
	collectStringValue bool
}

type listStreamState struct {
	itemBuiltin  *types.BuiltinType
	itemBuf      []byte
	collapsedBuf []byte
	itemMode     listItemMode
	itemIndex    int
	active       bool
	sawContent   bool
	needLexical  bool
	skipValidate bool
}

type streamRun struct {
	*validationRun
	dec             *xmlstream.StringReader
	constraintDecls map[types.QName][]*grammar.CompiledElement
	frames          []streamFrame
	violations      []errors.Validation
	identityScopes  []*identityScope
	currentLine     int
	currentColumn   int
	rootSeen        bool
	rootClosed      bool
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

	dec, err := xmlstream.NewStringReader(r, xmlstream.MaxQNameInternEntries(0))
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

func (r *streamRun) newAutomatonValidator(a *grammar.Automaton, wildcards []*types.AnyElement) *grammar.AutomatonStreamValidator {
	if r == nil || a == nil {
		return nil
	}
	return a.NewStreamValidator(r.matcher(), wildcards)
}

func (r *streamRun) validate(dec *xmlstream.StringReader) ([]errors.Validation, error) {
	r.reset()
	r.dec = dec
	r.frames = r.frames[:0]
	r.violations = r.violations[:0]
	r.rootSeen = false
	r.rootClosed = false
	r.currentLine = 0
	r.currentColumn = 0
	r.identityScopes = r.identityScopes[:0]
	r.constraintDecls = nil
	for {
		ev, err := dec.Next()
		if stderrors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return r.violations, err
		}
		r.setCurrentPos(ev.Line, ev.Column)

		switch ev.Kind {
		case xmlstream.EventStartElement:
			if r.rootClosed {
				return r.violations, fmt.Errorf("unexpected element %s after document end", ev.Name.Local)
			}
			if !r.rootSeen {
				r.rootSeen = true
			}
			if err := r.handleStart(dec, &ev); err != nil {
				return r.violations, err
			}

		case xmlstream.EventEndElement:
			if len(r.frames) == 0 {
				return r.violations, fmt.Errorf("unexpected end element %s", ev.Name.Local)
			}
			if err := r.handleEnd(); err != nil {
				return r.violations, err
			}
			if len(r.frames) == 0 && r.rootSeen {
				r.rootClosed = true
			}

		case xmlstream.EventCharData:
			if len(r.frames) == 0 {
				if !isIgnorableOutsideRoot(ev.Text) {
					return r.violations, fmt.Errorf("unexpected character data outside root element")
				}
				continue
			}
			r.handleCharData(&ev)
		}
	}

	if !r.rootSeen {
		return r.violations, fmt.Errorf("document has no root element")
	}
	if len(r.frames) != 0 {
		return r.violations, io.ErrUnexpectedEOF
	}

	r.addViolations(r.checkIDRefs())
	return r.violations, nil
}

func (r *streamRun) handleStart(dec *xmlstream.StringReader, ev *xmlstream.StringEvent) error {
	parent := r.currentFrame()
	name := toTypesQName(ev.Name)
	attrs := ev.Attrs
	if parent != nil {
		skipSubtree := r.prevalidateParentForChild(parent, name.Local)
		if skipSubtree {
			return dec.SkipSubtree()
		}
	}

	match, skipSubtree := r.resolveChildMatch(parent, name)
	if skipSubtree {
		return dec.SkipSubtree()
	}

	if match.processContents == types.Skip {
		return dec.SkipSubtree()
	}

	r.path.push(name.Local)
	start := streamStart{
		Name:       name,
		Attrs:      attrs,
		Line:       ev.Line,
		Column:     ev.Column,
		ID:         uint64(ev.ID),
		ScopeDepth: ev.ScopeDepth,
	}
	frame, skipSubtree := r.startFrame(&start, parent, match.processContents, match.matchedDecl, match.matchedQName, match.origin)
	if skipSubtree {
		r.path.pop()
		return dec.SkipSubtree()
	}
	r.frames = append(r.frames, frame)
	r.handleIdentityStart(&r.frames[len(r.frames)-1], attrs)
	r.maybeEnableListStreaming(len(r.frames) - 1)
	return nil
}

func toTypesQName(name xmlstream.QName) types.QName {
	return types.QName{
		Namespace: types.NamespaceURI(name.Namespace),
		Local:     name.Local,
	}
}

type startMatch struct {
	matchedDecl     *grammar.CompiledElement
	matchedQName    types.QName
	processContents types.ProcessContents
	origin          matchOrigin
}

func (r *streamRun) prevalidateParentForChild(parent *streamFrame, childName string) bool {
	parent.hasChildElements = true
	if parent.invalid || parent.skipChildren {
		return true
	}
	if parent.nilled {
		violation := errors.NewValidation(errors.ErrNilElementNotEmpty,
			"Element with xsi:nil='true' must be empty", r.path.String())
		r.addViolation(&violation)
		parent.invalid = true
		return true
	}
	if parent.decl != nil && parent.decl.HasFixed {
		violation := errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
			"Element '%s' has a fixed value constraint and cannot have element children", parent.decl.QName.Local)
		r.addViolation(&violation)
		parent.invalid = true
		return true
	}
	if parent.textType != nil && (parent.typ == nil || (!isAnyType(parent.typ) && !parent.typ.HasContentModel())) {
		r.addChildViolationf(errors.ErrTextInElementOnly, childName,
			"Element '%s' is not allowed in simple content", childName)
		parent.invalid = true
		return true
	}
	if parent.contentKind == streamContentEmpty {
		r.addChildViolationf(errors.ErrUnexpectedElement, childName,
			"Element '%s' is not allowed. No element declaration found for it in the empty content model.", childName)
		parent.invalid = true
		return true
	}
	if parent.contentKind == streamContentRejectAll {
		violation := errors.NewValidation(errors.ErrUnexpectedElement, "element not allowed by empty choice", r.path.String())
		r.addViolation(&violation)
		parent.invalid = true
		return true
	}
	return false
}

func (r *streamRun) resolveChildMatch(parent *streamFrame, name types.QName) (startMatch, bool) {
	match := startMatch{
		processContents: types.Strict,
		origin:          matchFromDeclaration,
	}
	if parent == nil {
		return match, false
	}

	switch parent.contentKind {
	case streamContentAny:
		match.processContents = types.Lax
	case streamContentAutomaton:
		automatonMatch, err := parent.automaton.Feed(name)
		if err != nil {
			r.addContentModelError(err)
			parent.invalid = true
			return match, true
		}
		if automatonMatch.IsWildcard {
			match.processContents = automatonMatch.ProcessContents
			match.origin = matchFromWildcard
		}
		match.matchedQName = automatonMatch.MatchedQName
		match.matchedDecl = automatonMatch.MatchedElement
	case streamContentAllGroup:
		groupMatch, err := parent.allGroup.Feed(name)
		if err != nil {
			r.addContentModelError(err)
			parent.invalid = true
			return match, true
		}
		match.matchedQName = groupMatch.MatchedQName
		match.matchedDecl = groupMatch.MatchedElement
	}

	return match, false
}

func (r *streamRun) addChildViolationf(code errors.ErrorCode, childName, format string, args ...any) {
	r.path.push(childName)
	violation := errors.NewValidationf(code, r.path.String(), format, args...)
	r.addViolation(&violation)
	r.path.pop()
}

func (r *streamRun) handleCharData(ev *xmlstream.StringEvent) {
	frame := r.currentFrame()
	if frame == nil || frame.invalid {
		return
	}

	if frame.nilled {
		if !isWhitespaceOnly(ev.Text) {
			violation := errors.NewValidation(errors.ErrNilElementNotEmpty,
				"Element with xsi:nil='true' must be empty", r.path.String())
			r.addViolation(&violation)
			frame.invalid = true
		}
		return
	}

	if frame.typ != nil && !frame.typ.AllowsText() && !isWhitespaceOnlyBytes(ev.Text) {
		violation := errors.NewValidation(errors.ErrTextInElementOnly,
			"Element content cannot have character children (non-whitespace text found)", r.path.String())
		r.addViolation(&violation)
		frame.invalid = true
		return
	}

	r.captureTextPos(frame, ev)

	if frame.listStream.active {
		frame.listStream.sawContent = true
		r.feedListStream(frame, ev.Text)
		return
	}

	if frame.collectStringValue {
		frame.textBuf = append(frame.textBuf, ev.Text...)
	}
}

func (r *streamRun) captureTextPos(frame *streamFrame, ev *xmlstream.StringEvent) {
	if frame == nil || frame.textLine > 0 || frame.textColumn > 0 {
		return
	}
	line, column, ok := firstNonWhitespacePos(ev.Text, ev.Line, ev.Column)
	if !ok {
		return
	}
	frame.textLine = line
	frame.textColumn = column
}

func (frame *streamFrame) textPos() (int, int) {
	if frame == nil {
		return 0, 0
	}
	if frame.textLine > 0 && frame.textColumn > 0 {
		return frame.textLine, frame.textColumn
	}
	return frame.startLine, frame.startColumn
}

func (r *streamRun) handleEnd() error {
	frame := r.popFrame()
	if frame == nil {
		return nil
	}
	defer r.path.pop()
	defer r.handleIdentityEnd(frame)

	if frame.invalid {
		r.propagateText(frame)
		return nil
	}

	if frame.nilled {
		if frame.hasChildElements {
			violation := errors.NewValidation(errors.ErrNilElementNotEmpty,
				"Element with xsi:nil='true' must be empty", r.path.String())
			r.addViolation(&violation)
		}
		r.propagateText(frame)
		return nil
	}

	if frame.skipChildren {
		r.propagateText(frame)
		return nil
	}

	if frame.textType != nil {
		if frame.listStream.active {
			r.finishListStream(frame)
			r.propagateText(frame)
			return nil
		}
		text := string(frame.textBuf)
		hadContent := text != ""
		if text == "" && frame.decl != nil {
			if frame.decl.Default != "" {
				text = frame.decl.Default
			} else if frame.decl.HasFixed {
				text = frame.decl.Fixed
			}
		}
		textLine, textColumn := frame.textPos()
		r.addViolationsAt(r.checkSimpleValue(text, frame.textType, frame.scopeDepth), textLine, textColumn)
		r.addViolationsAt(r.collectIDRefs(text, frame.textType, textLine, textColumn), textLine, textColumn)
		if frame.typ != nil && frame.typ.Kind == grammar.TypeKindComplex && len(frame.typ.Facets) > 0 {
			r.addViolationsAt(r.checkComplexTypeFacets(text, frame.typ), textLine, textColumn)
		}
		if frame.decl != nil && frame.decl.HasFixed && hadContent {
			r.addViolationsAt(r.checkFixedValue(text, frame.decl.Fixed, frame.textType), textLine, textColumn)
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
		if frame.minOccurs.CmpInt(0) > 0 && !frame.hasChildElements {
			violation := errors.NewValidation(errors.ErrRequiredElementMissing,
				"content does not satisfy empty choice", r.path.String())
			r.addViolation(&violation)
		}
	}

	if frame.decl != nil && frame.decl.HasFixed {
		text := string(frame.textBuf)
		if text == "" && !frame.hasChildElements {
			text = frame.decl.Fixed
		}
		textLine, textColumn := frame.textPos()
		r.addViolationsAt(r.checkFixedValue(text, frame.decl.Fixed, textTypeForFixedValue(frame.decl)), textLine, textColumn)
	}

	r.propagateText(frame)
	return nil
}

func (r *streamRun) startFrame(ev *streamStart, parent *streamFrame, processContents types.ProcessContents, matchedDecl *grammar.CompiledElement, matchedQName types.QName, origin matchOrigin) (streamFrame, bool) {
	attrs := newAttributeIndex(ev.Attrs)
	decl := r.resolveMatchedDecl(parent, ev.Name, matchedDecl, matchedQName)

	missingCode := errors.ErrElementNotDeclared
	if origin == matchFromWildcard && processContents == types.Strict {
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
		violation := errors.NewValidationf(errors.ErrElementAbstract, r.path.String(),
			"Element '%s' is abstract and cannot be used directly in instance documents", ev.Name.Local)
		r.addViolation(&violation)
		return streamFrame{}, true
	}

	nilValue, hasNil := attrs.Value(xsdxml.XSINamespace, "nil")
	isNil := hasNil && (nilValue == "true" || nilValue == "1")
	if hasNil && !decl.Nillable {
		violation := errors.NewValidationf(errors.ErrElementNotNillable, r.path.String(),
			"Element '%s' is not nillable but has xsi:nil attribute", ev.Name.Local)
		r.addViolation(&violation)
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
			violation := errors.NewValidation(errors.ErrXsiTypeInvalid, err.Error(), r.path.String())
			r.addViolation(&violation)
			return streamFrame{}, true
		}
		if xsiType != nil {
			effectiveType = xsiType
		}
	}

	if effectiveType.Abstract {
		violation := errors.NewValidationf(errors.ErrElementTypeAbstract, r.path.String(),
			"Type '%s' is abstract and cannot be used for element '%s'", effectiveType.QName.String(), ev.Name.Local)
		r.addViolation(&violation)
		return streamFrame{}, true
	}

	if isNil {
		if decl.HasFixed {
			violation := errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
				"Element '%s' has a fixed value constraint and cannot be nil", ev.Name.Local)
			r.addViolation(&violation)
			return streamFrame{}, true
		}
		violations := r.checkAttributesStream(attrs, effectiveType.AllAttributes, effectiveType.AnyAttribute, ev.ScopeDepth, ev.Line, ev.Column)
		if len(violations) > 0 {
			r.addViolations(violations)
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

	violations := r.checkAttributesStream(attrs, attrsToCheck, anyAttr, ev.ScopeDepth, ev.Line, ev.Column)
	if len(violations) > 0 {
		r.addViolations(violations)
		return streamFrame{}, true
	}

	frame := r.newFrame(ev, decl, effectiveType, parent)
	return frame, false
}

func (r *streamRun) newFrame(ev *streamStart, decl *grammar.CompiledElement, compiledType *grammar.CompiledType, parent *streamFrame) streamFrame {
	frame := streamFrame{
		id:          ev.ID,
		qname:       ev.Name,
		decl:        decl,
		typ:         compiledType,
		scopeDepth:  ev.ScopeDepth,
		startLine:   ev.Line,
		startColumn: ev.Column,
	}

	if compiledType != nil {
		frame.textType = compiledType.TextType()
		frame.contentModel = compiledType.ContentModel
		if isAnyType(compiledType) {
			frame.contentKind = streamContentAny
		} else if compiledType.ContentModel != nil {
			switch {
			case compiledType.ContentModel.RejectAll:
				frame.contentKind = streamContentRejectAll
				frame.minOccurs = compiledType.ContentModel.MinOccurs
			case compiledType.ContentModel.AllElements != nil:
				frame.contentKind = streamContentAllGroup
				allElements := make([]grammar.AllGroupElementInfo, len(compiledType.ContentModel.AllElements))
				for i := range compiledType.ContentModel.AllElements {
					allElements[i] = compiledType.ContentModel.AllElements[i]
				}
				frame.allGroup = grammar.NewAllGroupValidator(allElements, compiledType.ContentModel.Mixed, compiledType.ContentModel.MinOccurs).NewStreamValidator(r.matcher())
			case compiledType.ContentModel.Automaton != nil:
				frame.contentKind = streamContentAutomaton
				frame.automaton = r.newAutomatonValidator(compiledType.ContentModel.Automaton, compiledType.ContentModel.Wildcards())
			case compiledType.ContentModel.Empty:
				frame.contentKind = streamContentEmpty
			}
		}
	}

	frame.collectStringValue = frameNeedsStringValue(&frame, parent)
	return frame
}

func frameNeedsStringValue(frame, parent *streamFrame) bool {
	if frame == nil {
		return false
	}
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

func (r *streamRun) maybeEnableListStreaming(frameIndex int) {
	if r == nil || frameIndex < 0 || frameIndex >= len(r.frames) {
		return
	}
	frame := &r.frames[frameIndex]
	if !listStreamingAllowed(frame) {
		return
	}
	if frameIndex > 0 && r.frames[frameIndex-1].collectStringValue {
		return
	}
	if frame.decl != nil && frame.decl.HasFixed {
		return
	}
	if len(frame.fieldCaptures) > 0 {
		return
	}
	frame.listStream.skipValidate = listItemValidationSkippable(frame.textType.ItemType)
	frame.collectStringValue = false
	frame.listStream.active = true
	frame.listStream.needLexical = listFacetsNeedLexical(frame.textType.Facets)
	frame.listStream.itemMode, frame.listStream.itemBuiltin = listItemModeFor(frame.textType.ItemType)
}

func listStreamingAllowed(frame *streamFrame) bool {
	if frame == nil || frame.textType == nil || frame.textType.ItemType == nil {
		return false
	}
	if len(frame.textType.MemberTypes) > 0 {
		return false
	}
	if frame.textType.Original == nil || frame.textType.Original.WhiteSpace() != types.WhiteSpaceCollapse {
		return false
	}
	if _, ok := unresolvedSimpleType(frame.textType.Original); ok {
		return false
	}
	if frame.textType.IDTypeName != "" && frame.textType.IDTypeName != "IDREFS" {
		return false
	}
	if frame.typ != nil && frame.typ.Kind == grammar.TypeKindComplex && len(frame.typ.Facets) > 0 {
		return false
	}
	return listFacetsAllowStreaming(frame.textType.Facets)
}

func listFacetsAllowStreaming(facets []types.Facet) bool {
	return facetsAllowSimpleValue(facets)
}

func listFacetsNeedLexical(facets []types.Facet) bool {
	for _, facet := range facets {
		switch facet.(type) {
		case *types.Length, *types.MinLength, *types.MaxLength:
			continue
		default:
			return true
		}
	}
	return false
}

func listItemValidationSkippable(itemType *grammar.CompiledType) bool {
	if itemType == nil || itemType.Original == nil {
		return false
	}
	if len(itemType.MemberTypes) > 0 || len(itemType.Facets) > 0 {
		return false
	}
	if itemType.ItemType != nil || itemType.IsNotationType {
		return false
	}
	if _, ok := unresolvedSimpleType(itemType.Original); ok {
		return false
	}
	if itemType.Kind == grammar.TypeKindBuiltin {
		return listItemNoopBuiltin(itemType.Original)
	}
	base := itemType.BaseType
	for base != nil && base.Kind != grammar.TypeKindBuiltin {
		base = base.BaseType
	}
	if base == nil || base.Original == nil {
		return false
	}
	return listItemNoopBuiltin(base.Original)
}

func listItemNoopBuiltin(typ types.Type) bool {
	builtinType, ok := types.AsBuiltinType(typ)
	if !ok || builtinType == nil {
		return false
	}
	switch builtinType.Name().Local {
	case "anySimpleType", "string", "normalizedString", "token":
		return true
	default:
		return false
	}
}

func listItemModeFor(itemType *grammar.CompiledType) (listItemMode, *types.BuiltinType) {
	if itemType == nil || itemType.Original == nil {
		return listItemModeString, nil
	}
	if len(itemType.MemberTypes) > 0 || len(itemType.Facets) > 0 {
		return listItemModeString, nil
	}
	if itemType.ItemType != nil || itemType.IsNotationType {
		return listItemModeString, nil
	}
	if _, ok := unresolvedSimpleType(itemType.Original); ok {
		return listItemModeString, nil
	}
	builtin := listItemBuiltinBase(itemType)
	if builtin == nil || !builtin.HasByteValidator() {
		return listItemModeString, nil
	}
	return listItemModeBytes, builtin
}

func listItemBuiltinBase(itemType *grammar.CompiledType) *types.BuiltinType {
	if itemType == nil || itemType.Original == nil {
		return nil
	}
	if builtin, ok := types.AsBuiltinType(itemType.Original); ok {
		return builtin
	}
	base := itemType.BaseType
	for base != nil && base.Kind != grammar.TypeKindBuiltin {
		base = base.BaseType
	}
	if base == nil || base.Original == nil {
		return nil
	}
	builtin, _ := types.AsBuiltinType(base.Original)
	return builtin
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
		violation := errors.NewValidationf(code, r.path.String(),
			"Element '%s' is not declared (strict wildcard requires declaration)", local)
		r.addViolation(&violation)
		return
	}
	violation := errors.NewValidationf(code, r.path.String(),
		"Cannot find declaration for element '%s'", local)
	r.addViolation(&violation)
}

func (r *streamRun) addContentModelError(err error) {
	var ve *grammar.ValidationError
	if !stderrors.As(err, &ve) {
		return
	}
	violation := errors.NewValidation(errors.ErrorCode(ve.FullCode()), ve.Message, r.path.String())
	r.addViolation(&violation)
}

func (r *streamRun) setCurrentPos(line, column int) {
	if r == nil {
		return
	}
	r.currentLine = line
	r.currentColumn = column
}

func (r *streamRun) withPos(v *errors.Validation, line, column int) errors.Validation {
	if v == nil {
		return errors.Validation{}
	}
	if v.Line != 0 || v.Column != 0 {
		return *v
	}
	updated := *v
	if line > 0 && column > 0 {
		updated.Line = line
		updated.Column = column
	}
	return updated
}

func (r *streamRun) addViolation(v *errors.Validation) {
	if v == nil {
		return
	}
	r.violations = append(r.violations, r.withPos(v, r.currentLine, r.currentColumn))
}

func (r *streamRun) addViolationAt(v *errors.Validation, line, column int) {
	if v == nil {
		return
	}
	r.violations = append(r.violations, r.withPos(v, line, column))
}

func (r *streamRun) addViolations(violations []errors.Validation) {
	if len(violations) == 0 {
		return
	}
	for i := range violations {
		r.violations = append(r.violations, r.withPos(&violations[i], r.currentLine, r.currentColumn))
	}
}

func (r *streamRun) addViolationsAt(violations []errors.Validation, line, column int) {
	if len(violations) == 0 {
		return
	}
	for i := range violations {
		r.violations = append(r.violations, r.withPos(&violations[i], line, column))
	}
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

func (r *streamRun) feedListStream(frame *streamFrame, data []byte) {
	if frame == nil || frame.textType == nil || frame.textType.ItemType == nil {
		return
	}
	state := &frame.listStream
	for i := 0; i < len(data); {
		if isXMLWhitespaceByte(data[i]) {
			if len(state.itemBuf) > 0 {
				r.flushListItem(frame)
			}
			i++
			continue
		}
		start := i
		for i < len(data) && !isXMLWhitespaceByte(data[i]) {
			i++
		}
		if len(state.itemBuf) == 0 && i < len(data) {
			r.processListItemBytes(frame, data[start:i])
			continue
		}
		state.itemBuf = append(state.itemBuf, data[start:i]...)
		if i < len(data) {
			r.flushListItem(frame)
		}
	}
}

func (r *streamRun) finishListStream(frame *streamFrame) {
	if frame == nil || frame.textType == nil || frame.textType.ItemType == nil {
		return
	}
	state := &frame.listStream
	if !state.sawContent {
		value := ""
		if frame.decl != nil {
			if frame.decl.Default != "" {
				value = frame.decl.Default
			} else if frame.decl.HasFixed {
				value = frame.decl.Fixed
			}
		}
		textLine, textColumn := frame.textPos()
		r.addViolationsAt(r.checkSimpleValue(value, frame.textType, frame.scopeDepth), textLine, textColumn)
		r.addViolationsAt(r.collectIDRefs(value, frame.textType, textLine, textColumn), textLine, textColumn)
		return
	}

	r.flushListItem(frame)
	textLine, textColumn := frame.textPos()
	r.applyListStreamingFacets(frame.textType, state, textLine, textColumn)
}

func (r *streamRun) flushListItem(frame *streamFrame) {
	if frame == nil || frame.textType == nil || frame.textType.ItemType == nil {
		return
	}
	state := &frame.listStream
	if len(state.itemBuf) == 0 {
		return
	}
	itemBytes := state.itemBuf
	state.itemBuf = state.itemBuf[:0]
	r.processListItemBytes(frame, itemBytes)
}

func (r *streamRun) processListItemBytes(frame *streamFrame, itemBytes []byte) {
	if frame == nil || frame.textType == nil || frame.textType.ItemType == nil {
		return
	}
	if len(itemBytes) == 0 {
		return
	}
	state := &frame.listStream
	if state.needLexical {
		if len(state.collapsedBuf) > 0 {
			state.collapsedBuf = append(state.collapsedBuf, ' ')
		}
		state.collapsedBuf = append(state.collapsedBuf, itemBytes...)
	}
	index := state.itemIndex
	state.itemIndex++
	if state.skipValidate {
		if frame.textType.IDTypeName == "IDREFS" {
			r.trackListIDRefs(string(itemBytes), frame.textType)
		}
		return
	}
	if r.validateListItemBytes(frame, itemBytes, index) {
		if frame.textType.IDTypeName == "IDREFS" {
			r.trackListIDRefs(string(itemBytes), frame.textType)
		}
		return
	}
	itemString := string(itemBytes)
	valid, violations := r.validateListItemNormalized(itemString, frame.textType.ItemType, index, frame.scopeDepth, errorPolicyReport)
	if !valid {
		r.addViolations(violations)
	}
	if frame.textType.IDTypeName == "IDREFS" {
		r.trackListIDRefs(itemString, frame.textType)
	}
}

func (r *streamRun) validateListItemBytes(frame *streamFrame, itemBytes []byte, index int) bool {
	state := &frame.listStream
	if state.itemMode != listItemModeBytes || state.itemBuiltin == nil {
		return false
	}
	validated, err := state.itemBuiltin.ValidateBytes(itemBytes)
	if !validated {
		return false
	}
	if err != nil {
		violation := errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
			"list item[%d]: %s", index, err.Error())
		r.addViolation(&violation)
	}
	return true
}

func (r *streamRun) applyListStreamingFacets(compiledType *grammar.CompiledType, state *listStreamState, line, column int) {
	if compiledType == nil || state == nil {
		return
	}
	lexicalValue := ""
	if state.needLexical {
		lexicalValue = string(state.collapsedBuf)
	}
	var typedValue types.TypedValue

	for _, facet := range compiledType.Facets {
		if shouldSkipLengthFacet(compiledType, facet) {
			continue
		}
		var err error
		switch f := facet.(type) {
		case *types.Length:
			if state.itemIndex != f.Value {
				err = fmt.Errorf("length must be %d, got %d", f.Value, state.itemIndex)
			}
		case *types.MinLength:
			if state.itemIndex < f.Value {
				err = fmt.Errorf("length must be at least %d, got %d", f.Value, state.itemIndex)
			}
		case *types.MaxLength:
			if state.itemIndex > f.Value {
				err = fmt.Errorf("length must be at most %d, got %d", f.Value, state.itemIndex)
			}
		default:
			if lexicalFacet, ok := facet.(types.LexicalValidator); ok {
				err = lexicalFacet.ValidateLexical(lexicalValue, compiledType.Original)
				break
			}
			if typedValue == nil {
				if !state.needLexical {
					continue
				}
				typedValue = typedValueForFacets(lexicalValue, compiledType.Original, compiledType.Facets)
			}
			if facetErr := facet.Validate(typedValue, compiledType.Original); facetErr != nil {
				err = facetErr
			}
		}
		if err != nil {
			violation := errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String())
			r.addViolationAt(&violation, line, column)
		}
	}
}

func (r *streamRun) trackListIDRefs(item string, compiledType *grammar.CompiledType) {
	if compiledType == nil || compiledType.IDTypeName != "IDREFS" {
		return
	}
	r.trackIDREF(item, r.path.String(), r.currentLine, r.currentColumn)
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

func firstNonWhitespacePos(data []byte, line, column int) (int, int, bool) {
	if line <= 0 || column <= 0 {
		return 0, 0, false
	}
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case ' ', '\t':
			column++
		case '\n':
			line++
			column = 1
		case '\r':
			line++
			column = 1
			if i+1 < len(data) && data[i+1] == '\n' {
				i++
			}
		default:
			return line, column, true
		}
	}
	return 0, 0, false
}
