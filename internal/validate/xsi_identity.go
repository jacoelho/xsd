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

// XSIAttributeIdentityKey returns the identity-field key for an xsi attribute.
func XSIAttributeIdentityKey(rt *runtime.Schema, name xml.Name, lexical string, resolve runtime.ResolveQNameParts, ctx StartContext) (runtime.QName, string, bool, error) {
	rn := ResolveRuntimeName(rt, name)
	if !rn.Known {
		return runtime.QName{}, "", false, nil
	}
	key := runtime.SimpleIdentityKey(runtime.PrimitiveString, lex.CollapseXMLWhitespace(lexical))
	switch name.Local {
	case vocab.XSIAttrNil:
		v, ok := ParseXSINil(lexical)
		if !ok {
			return runtime.QName{}, "", false, validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:nil value")
		}
		key = runtime.SimpleIdentityKey(runtime.PrimitiveBoolean, runtime.BooleanCanonical(v))
	case vocab.XSIAttrType:
		canonical, err := xsiTypeCanonical(lexical, resolve)
		if err != nil {
			return runtime.QName{}, "", false, validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:type: "+err.Error())
		}
		key = runtime.SimpleIdentityKey(runtime.PrimitiveQName, canonical)
	case vocab.XSIAttrNoNamespaceSchemaLocation:
		anyURI, err := xsiAnyURIType(rt)
		if err != nil {
			return runtime.QName{}, "", false, err
		}
		value, err := rt.ValidateSimpleValue(anyURI, lexical, nil, runtime.SimpleNeedIdentity)
		if err != nil {
			return runtime.QName{}, "", false, validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:noNamespaceSchemaLocation URI "+lexical)
		}
		key = value.Identity
	case vocab.XSIAttrSchemaLocation:
		anyURI, err := xsiAnyURIType(rt)
		if err != nil {
			return runtime.QName{}, "", false, err
		}
		var items strings.Builder
		for field := range lex.XMLFieldsSeq(lexical) {
			value, err := rt.ValidateSimpleValue(anyURI, field, nil, runtime.SimpleNeedIdentity)
			if err != nil {
				return runtime.QName{}, "", false, validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:schemaLocation URI "+field)
			}
			if !runtime.AppendSimpleValueListIdentity(&items, value) {
				return runtime.QName{}, "", false, xsderrors.InternalInvariant("xsi:schemaLocation anyURI identity is missing")
			}
		}
		key = runtime.ListSimpleValue(runtime.ListSimpleValueProjection{
			ItemIdentity: items.String(),
			Needs:        runtime.SimpleNeedIdentity,
		}).Identity
	}
	return rn.Name, key, true, nil
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
