package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// ResolveRuntimeName returns name with its runtime QName when the schema knows it.
func ResolveRuntimeName(rt *runtime.Schema, name xml.Name) runtime.RuntimeName {
	q, ok := rt.LookupQName(name.Space, name.Local)
	if ok {
		return runtime.RuntimeName{Name: q, Known: true, NS: name.Space, Local: name.Local}
	}
	return runtime.RuntimeName{Known: false, NS: name.Space, Local: name.Local}
}

// NamespaceLookup resolves an XML namespace prefix to its URI.
type NamespaceLookup func(string) (string, bool)

// ResolveLexicalQNameParts resolves a lexical QName after XML whitespace
// collapse.
func ResolveLexicalQNameParts(lexical string, lookup NamespaceLookup) (namespace, local string, ok bool) {
	v := lex.CollapseXMLWhitespace(lexical)
	parts := lex.SplitQName(v)
	if !parts.Valid {
		return "", "", false
	}
	uri, ok := lookup(parts.Prefix)
	if !ok {
		return "", "", false
	}
	return uri, parts.Local, true
}

// HasSchemaLocation reports whether an xsi:schemaLocation hint was seen for a namespace.
type HasSchemaLocation func(string) bool

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

// RootInput is the root element start-assessment input.
type RootInput struct {
	Name              xml.Name
	RuntimeName       runtime.RuntimeName
	Values            *stream.Cache
	ResolveQNameParts runtime.ResolveQNameParts
	HasSchemaLocation HasSchemaLocation
	Context           StartContext
}

// StartResult is the validated start-element state to push onto the session stack.
type StartResult struct {
	Element runtime.ElementID
	Type    runtime.TypeID
	Skip    bool
	Recover bool
}

// RootStart assesses a document element before element-specific checks.
func RootStart(rt *runtime.Schema, attrs []stream.Attr, in RootInput) (StartResult, error) {
	if id, decl, ok := rt.RootElement(in.RuntimeName); ok {
		return StartResult{Element: id, Type: decl.Type}, nil
	}
	rootType, ok, err := rootTypeFromXSIType(rt, attrs, in)
	if err != nil {
		return StartResult{Element: runtime.NoElement, Type: rt.AnyType(), Skip: true}, err
	}
	if ok {
		return StartResult{Element: runtime.NoElement, Type: rootType}, nil
	}
	if in.HasSchemaLocation != nil && in.HasSchemaLocation(in.RuntimeName.NS) {
		return StartResult{Element: runtime.NoElement, Type: rt.AnyType(), Skip: true},
			unsupportedSchemaLocation(in.Context, vocab.XSDElemElement, in.RuntimeName)
	}
	return StartResult{Element: runtime.NoElement, Type: rt.AnyType(), Skip: true, Recover: true},
		validation(in.Context, xsderrors.CodeValidationRoot, "root element is not declared: "+formatXMLName(in.Name))
}

func rootTypeFromXSIType(rt *runtime.Schema, attrs []stream.Attr, in RootInput) (runtime.TypeID, bool, error) {
	for i := range attrs {
		a := &attrs[i]
		if !IsXSITypeName(a.Name) {
			continue
		}
		typ, err := resolveXSIType(rt, a.StringValue(in.Values), in.ResolveQNameParts, in.HasSchemaLocation, in.Context)
		if err != nil {
			return runtime.TypeID{}, false, err
		}
		return typ, true, nil
	}
	return runtime.TypeID{}, false, nil
}

type startDeclaration struct {
	block    runtime.DerivationMask
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
	typeID      runtime.TypeID
	typeInfo    runtime.TypeInfo
}

const elementNotNillableMessage = "element is not nillable"

func (state elementEffectiveState) issue() validationIssue {
	if issue := elementEffectiveTypeIssue(state.typeID, state.typeInfo); issue.valid() {
		return issue
	}
	if state.nil.specified && state.declaration.present && !state.declaration.nillable {
		return validationIssue{code: xsderrors.CodeValidationNil, message: elementNotNillableMessage}
	}
	if state.nil.value {
		if !state.declaration.present {
			return validationIssue{code: xsderrors.CodeValidationNil, message: elementNotNillableMessage}
		}
		if state.declaration.fixed {
			return validationIssue{code: xsderrors.CodeValidationNil, message: "nilled element cannot have fixed value"}
		}
	}
	return validationIssue{}
}

func elementEffectiveTypeIssue(typeID runtime.TypeID, info runtime.TypeInfo) validationIssue {
	if typeID.IsComplex() && info.Abstract {
		return validationIssue{code: xsderrors.CodeValidationType, message: "complex type is abstract"}
	}
	return validationIssue{}
}

type xsiTypeOverrideInput struct {
	ctx         StartContext
	declaration startDeclaration
	declared    runtime.TypeID
	override    runtime.TypeID
}

func validateXSITypeOverride(
	rt *runtime.Schema,
	scratch *runtime.TypeDerivationScratch,
	input xsiTypeOverrideInput,
) error {
	derivation, derived := rt.TypeDerivationWithScratch(input.override, input.declared, scratch)
	if !derived {
		return validation(input.ctx, xsderrors.CodeValidationType, "xsi:type is not derived from declared type")
	}
	if !input.declaration.present || input.override == input.declared {
		return nil
	}
	if input.declaration.block&runtime.DerivationExtension != 0 && derivation&runtime.DerivationExtension != 0 {
		return validation(input.ctx, xsderrors.CodeValidationType, "xsi:type extension is blocked")
	}
	if input.declaration.block&runtime.DerivationRestriction != 0 && derivation&runtime.DerivationRestriction != 0 {
		return validation(input.ctx, xsderrors.CodeValidationType, "xsi:type restriction is blocked")
	}
	return nil
}

func resolveXSIType(
	rt *runtime.Schema,
	value string,
	resolve runtime.ResolveQNameParts,
	hasSchemaLocation HasSchemaLocation,
	ctx StartContext,
) (runtime.TypeID, error) {
	ns, local, ok := resolve(value)
	if !ok {
		return runtime.TypeID{}, validation(ctx, xsderrors.CodeValidationType, "unknown xsi:type "+value)
	}
	q, knownName := rt.LookupQName(ns, local)
	if knownName {
		if typ, ok := rt.Type(q); ok {
			return typ, nil
		}
		ns = rt.Namespace(q.Namespace)
	}
	if hasSchemaLocation != nil && hasSchemaLocation(ns) {
		return runtime.TypeID{}, unsupportedSchemaLocation(ctx, vocab.XSIAttrType, runtime.RuntimeName{
			Name:  q,
			Known: knownName,
			NS:    ns,
			Local: local,
		})
	}
	return runtime.TypeID{}, validation(ctx, xsderrors.CodeValidationType, "unknown xsi:type "+value)
}

func validation(ctx StartContext, code xsderrors.Code, msg string) error {
	return xsderrors.WithLocation(ctx.PathString(), ctx.Line, ctx.Column, xsderrors.Validation(code, msg, nil))
}

func unsupportedSchemaLocation(ctx StartContext, component string, rn runtime.RuntimeName) error {
	return xsderrors.WithLocation(ctx.PathString(), ctx.Line, ctx.Column,
		xsderrors.Unsupported(
			xsderrors.CodeUnsupportedSchemaHint,
			"xsi:schemaLocation loading is not supported for "+component+" "+rn.Label(),
			nil,
		))
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

func xsiStartAttributeFlagsFor(attrs []stream.Attr) xsiStartAttributeFlags {
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

func formatXMLName(n xml.Name) string {
	return runtime.FormatExpandedName(n.Space, n.Local)
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
