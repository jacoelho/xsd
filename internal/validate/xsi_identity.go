package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/lex"
	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	xsdValue "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// xsiAttributeIdentityKey returns the identity-field key for an xsi attribute.
type xsiIdentityKey struct {
	key       string
	name      xsdSchema.QName
	textBytes int
	present   bool
}

func xsiAttributeIdentityKey(ctx startContext, rt *xsdSchema.Schema, name xml.Name, lexical string, resolve xsdValue.QNameResolver, eval xsdValue.EvaluationLimits) (xsiIdentityKey, error) {
	q, known := rt.ResolveName(name.Space, name.Local).QName()
	if !known {
		return xsiIdentityKey{}, nil
	}
	key, textBytes, err := xsiAttributeIdentity(ctx, rt, name.Local, lexical, resolve, eval)
	if err != nil {
		return xsiIdentityKey{}, err
	}
	return xsiIdentityKey{name: q, key: key, textBytes: textBytes, present: true}, nil
}

func xsiAttributeIdentity(ctx startContext, rt *xsdSchema.Schema, local, lexical string, resolve xsdValue.QNameResolver, eval xsdValue.EvaluationLimits) (string, int, error) {
	switch local {
	case vocab.XSIAttrNil:
		return xsiNilIdentity(ctx, rt, lexical, eval)
	case vocab.XSIAttrType:
		return xsiTypeIdentity(ctx, rt, lexical, resolve, eval)
	case vocab.XSIAttrNoNamespaceSchemaLocation:
		return xsiURIIdentity(ctx, rt, lexical, "xsi:noNamespaceSchemaLocation", eval)
	case vocab.XSIAttrSchemaLocation:
		return xsiSchemaLocationIdentity(ctx, rt, lexical, eval)
	default:
		collapsed := lex.CollapseXMLWhitespace(lexical)
		return xsdValue.PrimitiveIdentityKey(xsdValue.PrimitiveString, collapsed), len(collapsed), nil
	}
}

func xsiNilIdentity(ctx startContext, rt *xsdSchema.Schema, lexical string, eval xsdValue.EvaluationLimits) (string, int, error) {
	v, err := validateXSIValue(rt, vocab.XSDValueBoolean, lexical, xsdValue.Resolver{}, eval, ctx)
	if err != nil {
		return "", 0, simpleValueError(ctx, xsderrors.CodeValidationAttribute, msgInvalidXSINil, err, eval)
	}
	return v.IdentityKey(), v.IdentityTextBytes(), nil
}

// xsiTypeIdentity validates xsi:type only as a lexical QName. A bound QName
// naming an unknown or non-derived type is still an identity value; malformed
// or unbound QNames invalidate the matched field. Semantic xsi:type
// diagnostics belong to start assessment and are never repeated here.
func xsiTypeIdentity(ctx startContext, rt *xsdSchema.Schema, lexical string, resolve xsdValue.QNameResolver, eval xsdValue.EvaluationLimits) (string, int, error) {
	resolver := xsdValue.Resolver{}
	if resolve != nil {
		resolver.QName = resolve
	}
	v, err := validateXSIValue(rt, vocab.XSDValueQName, lexical, resolver, eval, ctx)
	if err != nil {
		return "", 0, simpleValueError(ctx, xsderrors.CodeValidationAttribute, msgInvalidXSIType, err, eval)
	}
	return v.IdentityKey(), v.IdentityTextBytes(), nil
}

func validateXSIValue(rt *xsdSchema.Schema, local, lexical string, resolver xsdValue.Resolver, eval xsdValue.EvaluationLimits, _ startContext) (xsdValue.Value, error) {
	id, err := xsiBuiltinType(rt, local)
	if err != nil {
		return xsdValue.Value{}, err
	}
	return rt.ValueProgram().Validate(id, lexical, resolver, xsdValue.NeedIdentity, eval, nil)
}

func xsiURIIdentity(ctx startContext, rt *xsdSchema.Schema, lexical, attribute string, eval xsdValue.EvaluationLimits) (string, int, error) {
	anyURI, err := xsiAnyURIType(rt)
	if err != nil {
		return "", 0, err
	}
	result, err := rt.ValueProgram().Validate(anyURI, lexical, xsdValue.Resolver{}, xsdValue.NeedIdentity, eval, nil)
	if err != nil {
		if terminalErr := terminalValueError(ctx, err, eval); terminalErr != nil {
			return "", 0, terminalErr
		}
		return "", 0, validation(ctx, xsderrors.CodeValidationAttribute, invalidURIMessage(attribute, lexical))
	}
	return result.IdentityKey(), result.IdentityTextBytes(), nil
}

// xsiSchemaLocationIdentity evaluates each URI item separately under the
// per-value work limit. The aggregate is bounded only by the attribute's token
// bytes: no hint limit exists; MaxInstanceTokenBytes bounds the attribute.
func xsiSchemaLocationIdentity(ctx startContext, rt *xsdSchema.Schema, lexical string, eval xsdValue.EvaluationLimits) (string, int, error) {
	anyURI, err := xsiAnyURIType(rt)
	if err != nil {
		return "", 0, err
	}
	items := make([]string, 0, 4)
	textBytes := 0
	for field := range lex.XMLFieldsSeq(lexical) {
		key, itemBytes, err := xsiSchemaLocationItemIdentity(ctx, rt, anyURI, field, eval)
		if err != nil {
			return "", 0, err
		}
		if len(items) > 0 {
			textBytes++
		}
		textBytes += itemBytes
		items = append(items, key)
	}
	return xsdValue.ListIdentityKey(items), textBytes, nil
}

func xsiSchemaLocationItemIdentity(ctx startContext, rt *xsdSchema.Schema, anyURI xsdSchema.SimpleTypeID, field string, eval xsdValue.EvaluationLimits) (string, int, error) {
	item, err := rt.ValueProgram().Validate(anyURI, field, xsdValue.Resolver{}, xsdValue.NeedIdentity, eval, nil)
	if err != nil {
		if terminalErr := terminalValueError(ctx, err, eval); terminalErr != nil {
			return "", 0, terminalErr
		}
		return "", 0, validation(ctx, xsderrors.CodeValidationAttribute, invalidURIMessage("xsi:schemaLocation", field))
	}
	if !item.HasIdentity() {
		return "", 0, xsderrors.InternalInvariant("xsi:schemaLocation anyURI identity is missing", nil)
	}
	return item.IdentityKey(), item.IdentityTextBytes(), nil
}

func xsiAnyURIType(rt *xsdSchema.Schema) (xsdSchema.SimpleTypeID, error) {
	return xsiBuiltinType(rt, vocab.XSDValueAnyURI)
}

func xsiBuiltinType(rt *xsdSchema.Schema, local string) (xsdSchema.SimpleTypeID, error) {
	name, ok := rt.LookupQName(vocab.XSDNamespaceURI, local)
	if !ok {
		return xsdSchema.NoSimpleType, xsderrors.InternalInvariant("xs:"+local+" name is missing", nil)
	}
	typ, ok := rt.Type(name)
	if !ok {
		return xsdSchema.NoSimpleType, xsderrors.InternalInvariant("xs:"+local+" type is missing", nil)
	}
	id, ok := typ.Simple()
	if !ok {
		return xsdSchema.NoSimpleType, xsderrors.InternalInvariant("xs:"+local+" is not a simple type", nil)
	}
	return id, nil
}
