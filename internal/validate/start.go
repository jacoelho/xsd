package validate

import (
	"encoding/xml"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// StartRuntime supplies semantic runtime facts used by start-element validation.
type StartRuntime interface {
	AnyType() runtime.TypeID
	RootElement(name runtime.RuntimeName) (runtime.ElementID, runtime.ElementStartInfo, bool)
	Element(id runtime.ElementID) (runtime.ElementStartInfo, bool)
	Type(name runtime.QName) (runtime.TypeID, bool)
	LookupQName(ns, local string) (runtime.QName, bool)
	Namespace(id runtime.NamespaceID) string
	TypeInfo(id runtime.TypeID) (runtime.TypeInfo, bool)
	TypeDerivation(derived, base runtime.TypeID) (runtime.DerivationMask, bool)
}

// NameRuntime resolves expanded XML names against the runtime name table.
type NameRuntime interface {
	LookupQName(ns, local string) (runtime.QName, bool)
}

// ResolveRuntimeName returns name with its runtime QName when the schema knows it.
func ResolveRuntimeName[RT NameRuntime](rt RT, name xml.Name) runtime.RuntimeName {
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
func ResolveLexicalQNameParts(lexical string, lookup NamespaceLookup) (string, string, bool) {
	v := lex.CollapseXMLWhitespace(lexical)
	prefix, local, ok := strings.Cut(v, ":")
	if !ok {
		local = v
		prefix = ""
	}
	if ok && prefix == "" {
		return "", "", false
	}
	if local == "" || strings.Contains(local, ":") || !lex.IsNCName(local) {
		return "", "", false
	}
	if prefix != "" && !lex.IsNCName(prefix) {
		return "", "", false
	}
	uri, ok := lookup(prefix)
	if !ok {
		return "", "", false
	}
	return uri, local, true
}

// HasSchemaLocation reports whether an xsi:schemaLocation hint was seen for a namespace.
type HasSchemaLocation func(string) bool

// StartContext identifies a validation location.
type StartContext struct {
	session *session
	Path    string
	Line    int
	Column  int
}

// PathString returns the current validation path, materializing it lazily for
// session-owned contexts.
func (ctx StartContext) PathString() string {
	if ctx.Path != "" || ctx.session == nil {
		return ctx.Path
	}
	return ctx.session.pathString()
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

// ElementInput is the declared element start-assessment input.
type ElementInput struct {
	ResolveQNameParts runtime.ResolveQNameParts
	HasSchemaLocation HasSchemaLocation
	Values            *stream.Cache
	Context           StartContext
	Element           runtime.ElementID
	Type              runtime.TypeID
	Skip              bool
}

// StartResult is the validated start-element state to push onto the session stack.
type StartResult struct {
	Element                   runtime.ElementID
	Type                      runtime.TypeID
	Skip                      bool
	Nilled                    bool
	Recover                   bool
	ElementValueKnown         bool
	ElementDeclared           bool
	ElementHasValueConstraint bool
}

// RootStart assesses a document element before element-specific checks.
func RootStart[RT StartRuntime](rt RT, attrs []stream.Attr, in RootInput) (StartResult, error) {
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

// ElementStart applies abstract, xsi:type, and xsi:nil start-element rules.
func ElementStart[RT StartRuntime](rt RT, attrs []stream.Attr, in ElementInput) (StartResult, error) {
	out := StartResult{Element: in.Element, Type: in.Type, Skip: in.Skip}
	if out.Skip {
		return out, nil
	}
	if decl, ok := rt.Element(out.Element); ok && decl.Abstract {
		return StartResult{Element: runtime.NoElement, Type: rt.AnyType(), Skip: true},
			validation(in.Context, xsderrors.CodeValidationElement, "abstract element cannot appear directly")
	}
	typ, nilled, err := effectiveType(rt, attrs, in)
	out.Type = typ
	out.Nilled = nilled
	return out, err
}

func rootTypeFromXSIType[RT StartRuntime](rt RT, attrs []stream.Attr, in RootInput) (runtime.TypeID, bool, error) {
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

func effectiveType[RT StartRuntime](rt RT, attrs []stream.Attr, in ElementInput) (runtime.TypeID, bool, error) {
	typ := in.Type
	nilled := false
	nilSpecified := false
	for i := range attrs {
		a := &attrs[i]
		if a.Name.Space != vocab.XSINamespaceURI {
			continue
		}
		value := a.StringValue(in.Values)
		switch a.Name.Local {
		case vocab.XSIAttrNil:
			nilSpecified = true
			parsed, ok := ParseXSINil(value)
			if !ok {
				return typ, false, validation(in.Context, xsderrors.CodeValidationNil, "invalid xsi:nil value")
			}
			nilled = parsed
		case vocab.XSIAttrType:
			override, err := resolveXSIType(rt, value, in.ResolveQNameParts, in.HasSchemaLocation, in.Context)
			if err != nil {
				return typ, nilled, err
			}
			if err := validateXSITypeOverride(rt, in.Element, typ, override, in.Context); err != nil {
				return typ, nilled, err
			}
			typ = override
		}
	}
	info, ok := rt.TypeInfo(typ)
	if !ok {
		return typ, nilled, xsderrors.InternalInvariant("start type metadata is invalid")
	}
	if info.Abstract {
		return typ, nilled, validation(in.Context, xsderrors.CodeValidationType, "complex type is abstract")
	}
	decl, declared := rt.Element(in.Element)
	if nilSpecified && declared && !decl.Nillable {
		return typ, nilled, validation(in.Context, xsderrors.CodeValidationNil, "element is not nillable")
	}
	if nilled {
		if !declared {
			return typ, nilled, validation(in.Context, xsderrors.CodeValidationNil, "element is not nillable")
		}
		if decl.Fixed {
			return typ, nilled, validation(in.Context, xsderrors.CodeValidationNil, "nilled element cannot have fixed value")
		}
	}
	return typ, nilled, nil
}

func validateXSITypeOverride[RT StartRuntime](rt RT, elem runtime.ElementID, declared, override runtime.TypeID, ctx StartContext) error {
	derivation, derived := rt.TypeDerivation(override, declared)
	if !derived {
		return validation(ctx, xsderrors.CodeValidationType, "xsi:type is not derived from declared type")
	}
	if elem == runtime.NoElement || override == declared {
		return nil
	}
	var block runtime.DerivationMask
	if decl, declaredElement := rt.Element(elem); declaredElement {
		block |= decl.Block
	}
	info, validType := rt.TypeInfo(declared)
	if !validType {
		return xsderrors.InternalInvariant("declared type metadata is invalid")
	}
	block |= info.Block
	if block&runtime.DerivationExtension != 0 && derivation&runtime.DerivationExtension != 0 {
		return validation(ctx, xsderrors.CodeValidationType, "xsi:type extension is blocked")
	}
	if block&runtime.DerivationRestriction != 0 && derivation&runtime.DerivationRestriction != 0 {
		return validation(ctx, xsderrors.CodeValidationType, "xsi:type restriction is blocked")
	}
	return nil
}

func resolveXSIType[RT StartRuntime](
	rt RT,
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
			Local: value,
		})
	}
	return runtime.TypeID{}, validation(ctx, xsderrors.CodeValidationType, "unknown xsi:type "+value)
}

func validation(ctx StartContext, code xsderrors.Code, msg string) error {
	return xsderrors.Validation(code, ctx.Line, ctx.Column, ctx.PathString(), msg)
}

func unsupportedSchemaLocation(ctx StartContext, component string, rn runtime.RuntimeName) error {
	return xsderrors.UnsupportedAt(
		xsderrors.CodeUnsupportedSchemaHint,
		ctx.Line,
		ctx.Column,
		ctx.PathString(),
		"xsi:schemaLocation loading is not supported for "+component+" "+rn.Label(),
		nil,
	)
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

func (f xsiStartAttributeFlags) hasStartAttribute() bool {
	return f.Type || f.Nil
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

// HasXSITypeAttribute reports whether attrs contains an xsi:type attribute.
func HasXSITypeAttribute(attrs []stream.Attr) bool {
	return xsiStartAttributeFlagsFor(attrs).Type
}

func formatXMLName(n xml.Name) string {
	return runtime.FormatExpandedName(n.Space, n.Local)
}

// ParseXSINil parses an xsi:nil attribute value after XML whitespace collapse.
func ParseXSINil(lexical string) (bool, bool) {
	switch lex.CollapseXMLWhitespace(lexical) {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	default:
		return false, false
	}
}
