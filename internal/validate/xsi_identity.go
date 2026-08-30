package validate

import (
	"encoding/xml"
	"errors"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// xsiAttributeIdentityKey returns the identity-field key for an xsi attribute.
type xsiIdentityKey struct {
	key     string
	name    runtime.QName
	present bool
}

func xsiAttributeIdentityKey(rt *runtime.Schema, name xml.Name, lexical string, resolve runtime.ResolveQNameParts, ctx StartContext) (xsiIdentityKey, error) {
	rn := ResolveRuntimeName(rt, name)
	if !rn.Known {
		return xsiIdentityKey{}, nil
	}
	key, err := xsiAttributeIdentity(rt, name.Local, lexical, resolve, ctx)
	if err != nil {
		return xsiIdentityKey{}, err
	}
	return xsiIdentityKey{name: rn.Name, key: key, present: true}, nil
}

func xsiAttributeIdentity(rt *runtime.Schema, local, lexical string, resolve runtime.ResolveQNameParts, ctx StartContext) (string, error) {
	switch local {
	case vocab.XSIAttrNil:
		return xsiNilIdentity(lexical, ctx)
	case vocab.XSIAttrType:
		return xsiTypeIdentity(lexical, resolve, ctx)
	case vocab.XSIAttrNoNamespaceSchemaLocation:
		return xsiURIIdentity(rt, lexical, "invalid xsi:noNamespaceSchemaLocation URI "+lexical, ctx)
	case vocab.XSIAttrSchemaLocation:
		return xsiSchemaLocationIdentity(rt, lexical, ctx)
	default:
		return runtime.SimpleIdentityKey(runtime.PrimitiveString, lex.CollapseXMLWhitespace(lexical)), nil
	}
}

func xsiNilIdentity(lexical string, ctx StartContext) (string, error) {
	v, ok := ParseXSINil(lexical)
	if !ok {
		return "", validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:nil value")
	}
	return runtime.SimpleIdentityKey(runtime.PrimitiveBoolean, runtime.BooleanCanonical(v)), nil
}

func xsiTypeIdentity(lexical string, resolve runtime.ResolveQNameParts, ctx StartContext) (string, error) {
	canonical, err := xsiTypeCanonical(lexical, resolve)
	if err != nil {
		return "", validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:type: "+err.Error())
	}
	return runtime.SimpleIdentityKey(runtime.PrimitiveQName, canonical), nil
}

func xsiURIIdentity(rt *runtime.Schema, lexical, message string, ctx StartContext) (string, error) {
	anyURI, err := xsiAnyURIType(rt)
	if err != nil {
		return "", err
	}
	value, err := rt.ValidateSimpleValue(anyURI, lexical, nil, runtime.SimpleNeedIdentity)
	if err != nil {
		return "", validation(ctx, xsderrors.CodeValidationAttribute, message)
	}
	return value.Identity, nil
}

func xsiSchemaLocationIdentity(rt *runtime.Schema, lexical string, ctx StartContext) (string, error) {
	anyURI, err := xsiAnyURIType(rt)
	if err != nil {
		return "", err
	}
	var items strings.Builder
	for field := range lex.XMLFieldsSeq(lexical) {
		value, err := rt.ValidateSimpleValue(anyURI, field, nil, runtime.SimpleNeedIdentity)
		if err != nil {
			return "", validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:schemaLocation URI "+field)
		}
		if !runtime.AppendSimpleValueListIdentity(&items, value) {
			return "", xsderrors.InternalInvariant("xsi:schemaLocation anyURI identity is missing")
		}
	}
	return runtime.ListSimpleValue(runtime.ListSimpleValueProjection{
		ItemIdentity: items.String(),
		Needs:        runtime.SimpleNeedIdentity,
	}).Identity, nil
}

func xsiAnyURIType(rt *runtime.Schema) (runtime.SimpleTypeID, error) {
	name, ok := rt.LookupQName(vocab.XSDNamespaceURI, vocab.XSDValueAnyURI)
	if !ok {
		return runtime.NoSimpleType, xsderrors.InternalInvariant("xs:anyURI name is missing")
	}
	typ, ok := rt.Type(name)
	if !ok {
		return runtime.NoSimpleType, xsderrors.InternalInvariant("xs:anyURI type is missing")
	}
	id, ok := typ.Simple()
	if !ok {
		return runtime.NoSimpleType, xsderrors.InternalInvariant("xs:anyURI is not a simple type")
	}
	return id, nil
}

func xsiTypeCanonical(lexical string, resolve runtime.ResolveQNameParts) (string, error) {
	lexical = lex.CollapseXMLWhitespace(lexical)
	if resolve == nil {
		ns, local, ok := ResolveLexicalQNameParts(lexical, func(prefix string) (string, bool) {
			return "", prefix == ""
		})
		if !ok {
			return "", errors.New("invalid QName")
		}
		return runtime.FormatExpandedName(ns, local), nil
	}
	ns, local, ok := resolve(lexical)
	if !ok {
		return "", errors.New("unresolved QName")
	}
	return runtime.FormatExpandedName(ns, local), nil
}
