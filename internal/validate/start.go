package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/lex"
	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	xsdValue "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

// ResolveRuntimeName returns name with its runtime QName when the schema knows it.
func ResolveRuntimeName(rt *xsdSchema.Schema, name xml.Name) xsdSchema.RuntimeName {
	q, ok := rt.LookupQName(name.Space, name.Local)
	if ok {
		return xsdSchema.RuntimeName{Name: q, Known: true, NS: name.Space, Local: name.Local}
	}
	return xsdSchema.RuntimeName{Known: false, NS: name.Space, Local: name.Local}
}

// NamespaceLookup resolves an XML namespace prefix to its URI.
type NamespaceLookup func(string) (string, bool)

// ResolveLexicalQName resolves a lexical QName after XML whitespace collapse.
func ResolveLexicalQName(lexical string, lookup NamespaceLookup) (xsdValue.ExpandedName, bool) {
	v := lex.CollapseXMLWhitespace(lexical)
	parts := lex.SplitQName(v)
	if !parts.Valid {
		return xsdValue.ExpandedName{}, false
	}
	uri, ok := lookup(parts.Prefix)
	if !ok {
		return xsdValue.ExpandedName{}, false
	}
	return xsdValue.ExpandedName{Namespace: uri, Local: parts.Local}, true
}

type pathSource interface {
	PathString() string
	PathStringAtDepth(depth int) string
	retainPathAtDepth(depth int) retainedPath
}

// StartContext identifies a validation location.
type StartContext struct {
	document pathSource
	Path     string
	Line     int
	Column   int
}

// PathString returns the current validation path, materializing it lazily for
// document-owned contexts.
func (ctx StartContext) PathString() string {
	if ctx.Path != "" || ctx.document == nil {
		return ctx.Path
	}
	return ctx.document.PathString()
}

// PathStringAtDepth returns the validation path at depth. Explicit contexts
// already represent their requested location.
func (ctx StartContext) PathStringAtDepth(depth int) string {
	if ctx.Path != "" || ctx.document == nil {
		return ctx.Path
	}
	return ctx.document.PathStringAtDepth(depth)
}

func (ctx StartContext) retainPathAtDepth(depth int) retainedPath {
	if ctx.document == nil {
		panic("retained XML path requires a document context")
	}
	return ctx.document.retainPathAtDepth(depth)
}

type startDeclaration struct {
	block    xsdSchema.DerivationMask
	present  bool
	abstract bool
	nillable bool
	fixed    bool
}

type assessedNilValue struct {
	value     bool
	specified bool
}

type elementEffectiveState struct {
	declaration startDeclaration
	nil         assessedNilValue
	typeID      xsdSchema.TypeID
	typeInfo    xsdSchema.TypeInfo
}

func (state elementEffectiveState) issue() validationIssue {
	if issue := elementEffectiveTypeIssue(state.typeID, state.typeInfo); issue.valid() {
		return issue
	}
	if state.nil.specified && state.declaration.present && !state.declaration.nillable {
		return validationIssue{code: xsderrors.CodeValidationNil, message: elementNotNillableMessage}
	}
	if state.nilled() && state.declaration.fixed {
		return validationIssue{code: xsderrors.CodeValidationNil, message: msgNilledFixed}
	}
	return validationIssue{}
}

// nilled reports whether the element is nilled. Only a governing element
// declaration can nil an element (cvc-elt 3.2); without one, xsi:nil is
// checked as an xs:boolean and the element is assessed by its type alone.
func (state elementEffectiveState) nilled() bool {
	return state.nil.value && state.declaration.present
}

func elementEffectiveTypeIssue(typeID xsdSchema.TypeID, info xsdSchema.TypeInfo) validationIssue {
	if typeID.IsComplex() && info.Abstract {
		return validationIssue{code: xsderrors.CodeValidationType, message: msgAbstractType}
	}
	return validationIssue{}
}

type xsiTypeOverrideInput struct {
	declared    xsdSchema.TypeID
	override    xsdSchema.TypeID
	declaration startDeclaration
	// declaredBlock is the declared type's {prohibited substitutions}; it is
	// kept apart from the declaration's mask so a rejection names its owner.
	declaredBlock xsdSchema.DerivationMask
}

func validateXSITypeOverride(
	rt *xsdSchema.Schema,
	scratch *xsdSchema.TypeDerivationScratch,
	input xsiTypeOverrideInput,
) error {
	derivation, derived := rt.TypeDerivationWithScratch(input.override, input.declared, scratch)
	if !derived {
		return xsderrors.Validation(xsderrors.CodeValidationType, xsiTypeDerivationMessage(rt, input.override, input.declared), nil)
	}
	if !input.declaration.present || input.override == input.declared {
		return nil
	}
	// cvc-elt 4.3 blocks the union of both masks; the declaration is named
	// first when both block the same method.
	for _, method := range [...]struct {
		name string
		mask xsdSchema.DerivationMask
	}{{"extension", xsdSchema.DerivationExtension}, {"restriction", xsdSchema.DerivationRestriction}} {
		if derivation&method.mask == 0 {
			continue
		}
		if input.declaration.block&method.mask != 0 {
			return xsderrors.Validation(xsderrors.CodeValidationType, xsiTypeBlockedByDeclarationMessage(rt, input.override, method.name), nil)
		}
		if input.declaredBlock&method.mask != 0 {
			return xsderrors.Validation(xsderrors.CodeValidationType, xsiTypeBlockedByTypeMessage(rt, input.override, input.declared, method.name), nil)
		}
	}
	return nil
}

func resolveXSIType(
	rt *xsdSchema.Schema,
	value string,
	resolve xsdValue.QNameResolver,
) (xsdSchema.TypeID, error) {
	name, ok := resolve(value)
	if !ok {
		return xsdSchema.TypeID{}, xsderrors.Validation(xsderrors.CodeValidationType, invalidXSITypeQNameMessage(value), nil)
	}
	if q, knownName := rt.LookupQName(name.Namespace, name.Local); knownName {
		if typ, ok := rt.Type(q); ok {
			return typ, nil
		}
	}
	return xsdSchema.TypeID{}, xsderrors.Validation(xsderrors.CodeValidationType, unknownXSITypeMessage(value, name), nil)
}

// IsXSITypeName reports whether name is the xsi:type attribute.
func IsXSITypeName(name xml.Name) bool {
	return name.Space == vocab.XSINamespaceURI && name.Local == vocab.XSIAttrType
}

type xsiStartAttributeFlags struct {
	Type           bool
	Nil            bool
	SchemaLocation bool
}

func xsiStartAttributeFlagsFor(attrs []xmlstream.Attr) xsiStartAttributeFlags {
	var flags xsiStartAttributeFlags
	for i := range attrs {
		if attrs[i].Name.Space != vocab.XSINamespaceURI {
			continue
		}
		switch attrs[i].Name.Local {
		case vocab.XSIAttrType:
			flags.Type = true
		case vocab.XSIAttrNil:
			flags.Nil = true
		case vocab.XSIAttrSchemaLocation, vocab.XSIAttrNoNamespaceSchemaLocation:
			flags.SchemaLocation = true
		}
	}
	return flags
}

// ParseXSINil parses an xsi:nil attribute value after XML whitespace collapse.
func ParseXSINil(lexical string) (value, valid bool) {
	switch lex.CollapseXMLWhitespace(lexical) {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	default:
		return false, false
	}
}
