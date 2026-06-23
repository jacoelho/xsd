package validate

import (
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// CharacterDataContent summarizes whether character data is allowed in an
// element frame.
type CharacterDataContent interface {
	HasSimpleContent() bool
	IsComplexType() bool
	AllowsMixedContent() bool
	HasFixedElementValue() bool
}

// CharacterDataInput is the current element frame and token state needed to
// validate character data.
type CharacterDataInput struct {
	Data       []byte
	Content    CharacterDataContent
	Context    StartContext
	HasElement bool
	CDATA      bool
	Skip       bool
	Nilled     bool
}

// CharacterDataRuntime supplies element content metadata needed to validate
// character data for the current frame.
type CharacterDataRuntime[Content CharacterDataContent] interface {
	ElementTextContent(typ runtime.TypeID, elem runtime.ElementID) (Content, bool)
}

// ElementCharacterDataInput is the current element frame and token state before
// runtime content metadata has been read.
type ElementCharacterDataInput struct {
	Data    []byte
	Context StartContext
	Type    runtime.TypeID
	Element runtime.ElementID
	CDATA   bool
	Skip    bool
	Nilled  bool
}

// CharacterDataResult reports which schema-owned frame/text buffers need
// mutation after character-data validation.
type CharacterDataResult struct {
	AppendText bool
	HasText    bool
}

// ValidateElementCharacterData validates character data for an element frame.
func ValidateElementCharacterData[Content CharacterDataContent, RT CharacterDataRuntime[Content]](rt RT, in ElementCharacterDataInput) (CharacterDataResult, error) {
	if any(rt) == nil {
		return CharacterDataResult{}, xsderrors.InternalInvariant("character data runtime is missing")
	}
	content, ok := rt.ElementTextContent(in.Type, in.Element)
	if !ok {
		return CharacterDataResult{}, xsderrors.InternalInvariant("character data content info is invalid")
	}
	return ValidateCharacterData(CharacterDataInput{
		Data:       in.Data,
		Content:    content,
		Context:    in.Context,
		HasElement: true,
		CDATA:      in.CDATA,
		Skip:       in.Skip,
		Nilled:     in.Nilled,
	})
}

// ValidateCharacterData validates character data and returns required frame
// mutations for the caller-owned session state.
func ValidateCharacterData(in CharacterDataInput) (CharacterDataResult, error) {
	if !in.HasElement {
		if in.CDATA {
			return CharacterDataResult{}, validation(in.Context, xsderrors.CodeValidationXML, "CDATA section outside root element")
		}
		if lex.IsXMLWhitespaceBytes(in.Data) {
			return CharacterDataResult{}, nil
		}
		return CharacterDataResult{}, validation(in.Context, xsderrors.CodeValidationText, "text outside root element")
	}
	if len(in.Data) == 0 || in.Skip {
		return CharacterDataResult{}, nil
	}
	if err := ValidateNilledContent(NilledContentInput{Context: in.Context, Nilled: in.Nilled, HasText: true}); err != nil {
		return CharacterDataResult{}, err
	}
	if in.Content == nil {
		return CharacterDataResult{}, xsderrors.InternalInvariant("character data content info is missing")
	}
	if in.Content.HasSimpleContent() {
		return CharacterDataResult{AppendText: true}, nil
	}
	whitespace := lex.IsXMLWhitespaceBytes(in.Data)
	out := CharacterDataResult{HasText: !whitespace}
	if in.Content.AllowsMixedContent() {
		out.AppendText = in.Content.HasFixedElementValue()
		return out, nil
	}
	if in.Content.IsComplexType() && !whitespace {
		return CharacterDataResult{}, validation(in.Context, xsderrors.CodeValidationText, "character data is not allowed")
	}
	return out, nil
}

// ContentRuntime supplies semantic runtime facts used by content validation.
type ContentRuntime interface {
	runtime.CompiledContentRuntime
	AnyType() runtime.TypeID
	ChildContent(id runtime.TypeID) (runtime.ChildContentInfo, bool)
	DeclaredElementType(id runtime.ElementID) (runtime.TypeID, bool)
}

// ChildInput is the parent-frame and child-name state needed to accept a child.
type ChildInput struct {
	HasSchemaLocation HasSchemaLocation
	Context           StartContext
	Name              runtime.RuntimeName
	Scratch           runtime.ContentScratch
	ParentChild       runtime.ChildContentInfo
	ParentContent     runtime.ContentState
	ParentType        runtime.TypeID
	HasXSIType        bool
	HasParentChild    bool
	ParentSkip        bool
	ParentNilled      bool
}

// ChildResult is the child declaration and updated parent content-model state.
type ChildResult struct {
	Element runtime.ElementID
	Type    runtime.TypeID
	Content runtime.ContentState
	Skip    bool
	Recover bool
}

// NilledContentInput reports whether a nilled element has content.
type NilledContentInput struct {
	Context  StartContext
	Nilled   bool
	HasChild bool
	HasText  bool
}

// ValidateNilledContent rejects child or text content inside a nilled element.
func ValidateNilledContent(in NilledContentInput) error {
	if in.Nilled && (in.HasChild || in.HasText) {
		return validation(in.Context, xsderrors.CodeValidationNil, "nilled element must be empty")
	}
	return nil
}

// ChildStart validates a child element against its parent content model.
func ChildStart[RT ContentRuntime](rt RT, in ChildInput) (ChildResult, error) {
	if in.ParentSkip {
		return skippedChild(rt), nil
	}
	if err := ValidateNilledContent(NilledContentInput{Context: in.Context, Nilled: in.ParentNilled, HasChild: true}); err != nil {
		return recoverableChildError(rt, err)
	}
	parentContent := in.ParentChild
	if !in.HasParentChild {
		var ok bool
		parentContent, ok = rt.ChildContent(in.ParentType)
		if !ok {
			return ChildResult{}, xsderrors.InternalInvariant("child content metadata is invalid")
		}
	}
	if !parentContent.Complex {
		return recoverableChild(rt, in.Context, xsderrors.CodeValidationContent, "simple type cannot contain child elements")
	}
	if parentContent.Simple {
		return recoverableChild(rt, in.Context, xsderrors.CodeValidationContent, "simple content cannot contain child elements")
	}
	if !in.ParentContent.HasModel() {
		return recoverableChild(rt, in.Context, xsderrors.CodeValidationElement, "unexpected child element "+in.Name.Label())
	}
	st := in.ParentContent
	contentInput := runtime.ContentInput{Name: in.Name, HasXSIType: in.HasXSIType}
	var match runtime.ContentMatch
	var ok bool
	var valid bool
	if schema, isSchema := any(rt).(*runtime.Schema); isSchema {
		match, ok, valid = schema.AdvancePublishedSchemaContent(&st, contentInput, &in.Scratch)
	} else {
		match, ok, valid = runtime.AdvanceCompiledContent(rt, &st, contentInput, &in.Scratch)
	}
	if !valid {
		return ChildResult{}, xsderrors.InternalInvariant("content model state is invalid")
	}
	if !ok {
		return recoverableChild(rt, in.Context, xsderrors.CodeValidationElement, "unexpected child element "+in.Name.Label())
	}
	out, err := childFromMatch(rt, match)
	if err != nil {
		return ChildResult{}, err
	}
	out.Content = st
	if match.StrictMissing {
		if in.HasSchemaLocation != nil && in.HasSchemaLocation(in.Name.NS) {
			return ChildResult{}, unsupportedSchemaLocation(in.Context, vocab.XSDElemElement, in.Name)
		}
		return recoverableChild(rt, in.Context, xsderrors.CodeValidationElement, "wildcard requires declared element "+in.Name.Label())
	}
	return out, nil
}

// CompleteInput is the content-model state needed at element end.
type CompleteInput struct {
	Context StartContext
	Scratch runtime.ContentScratch
	Type    runtime.TypeID
	Content runtime.ContentState
	Nilled  bool
}

// ContentComplete validates that an element's content model may end.
func ContentComplete[RT ContentRuntime](rt RT, in CompleteInput) error {
	if in.Nilled || in.Type.Kind != runtime.TypeComplex || !in.Content.HasModel() {
		return nil
	}
	var complete bool
	var ok bool
	if schema, isSchema := any(rt).(*runtime.Schema); isSchema {
		complete, ok = schema.CompletePublishedSchemaContent(in.Content, &in.Scratch)
	} else {
		complete, ok = runtime.CompleteCompiledContent(rt, in.Content, &in.Scratch)
	}
	if !ok {
		return xsderrors.InternalInvariant("content model state is invalid")
	}
	if complete {
		return nil
	}
	return validation(in.Context, xsderrors.CodeValidationContent, "missing required child element")
}

func skippedChild[RT ContentRuntime](rt RT) ChildResult {
	return ChildResult{Element: runtime.NoElement, Type: rt.AnyType(), Skip: true}
}

func recoverableChild[RT ContentRuntime](rt RT, ctx StartContext, code xsderrors.Code, msg string) (ChildResult, error) {
	return recoverableChildError(rt, validation(ctx, code, msg))
}

func recoverableChildError[RT ContentRuntime](rt RT, err error) (ChildResult, error) {
	out := skippedChild(rt)
	out.Recover = true
	return out, err
}

func childFromMatch[RT ContentRuntime](rt RT, m runtime.ContentMatch) (ChildResult, error) {
	if m.Element == runtime.NoElement {
		return ChildResult{Element: runtime.NoElement, Type: rt.AnyType(), Skip: m.Skip}, nil
	}
	typ, ok := rt.DeclaredElementType(m.Element)
	if !ok {
		return ChildResult{}, xsderrors.InternalInvariant("content model matched invalid element declaration")
	}
	return ChildResult{Element: m.Element, Type: typ}, nil
}
