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
func ResolveLexicalQNameParts(lexical string, lookup NamespaceLookup) (string, string, bool) {
	v := lex.CollapseXMLWhitespace(lexical)
	prefix, local, _, ok := lex.SplitQName(v)
	if !ok {
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

type pathSource interface {
	PathString() string
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

func validateElementEffectiveState(
	decl runtime.ElementStartInfo,
	declared bool,
	typ runtime.TypeID,
	nilled, nilSpecified bool,
	info runtime.TypeInfo,
	infoKnown bool,
	ctx StartContext,
) (runtime.TypeID, bool, error) {
	if !infoKnown {
		return typ, nilled, xsderrors.InternalInvariant("start type metadata is invalid")
	}
	if typ.IsComplex() && info.Abstract {
		return typ, nilled, validation(ctx, xsderrors.CodeValidationType, "complex type is abstract")
	}
	if nilSpecified && declared && !decl.Nillable {
		return typ, nilled, validation(ctx, xsderrors.CodeValidationNil, "element is not nillable")
	}
	if nilled {
		if !declared {
			return typ, nilled, validation(ctx, xsderrors.CodeValidationNil, "element is not nillable")
		}
		if decl.Fixed {
			return typ, nilled, validation(ctx, xsderrors.CodeValidationNil, "nilled element cannot have fixed value")
		}
	}
	return typ, nilled, nil
}

func validateXSITypeOverride(
	rt *runtime.Schema,
	declared, override runtime.TypeID,
	elementBlock runtime.DerivationMask,
	declaredElement bool,
	scratch *runtime.TypeDerivationScratch,
	ctx StartContext,
) error {
	derivation, derived := rt.TypeDerivationWithScratch(override, declared, scratch)
	if !derived {
		return validation(ctx, xsderrors.CodeValidationType, "xsi:type is not derived from declared type")
	}
	if !declaredElement || override == declared {
		return nil
	}
	if elementBlock&runtime.DerivationExtension != 0 && derivation&runtime.DerivationExtension != 0 {
		return validation(ctx, xsderrors.CodeValidationType, "xsi:type extension is blocked")
	}
	if elementBlock&runtime.DerivationRestriction != 0 && derivation&runtime.DerivationRestriction != 0 {
		return validation(ctx, xsderrors.CodeValidationType, "xsi:type restriction is blocked")
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
