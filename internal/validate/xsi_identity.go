package validate

import (
	"encoding/xml"
	"errors"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// XSIAttributeIdentityKey returns the identity-field key for an xsi attribute.
func XSIAttributeIdentityKey(rt NameRuntime, name xml.Name, lexical string, resolve runtime.ResolveQNameParts, ctx StartContext) (runtime.QName, string, bool, error) {
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
	}
	return rn.Name, key, true, nil
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
