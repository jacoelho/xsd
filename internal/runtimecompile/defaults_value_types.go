package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schemaops"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) compileDefaultFixedValue(lexical string, typ types.Type, ctx map[string]string) (compiledDefaultFixed, error) {
	canon, member, key, err := c.canonicalizeDefaultFixed(lexical, typ, ctx)
	if err != nil {
		return compiledDefaultFixed{}, err
	}
	return compiledDefaultFixed{
		ok:     true,
		ref:    c.values.add(canon),
		key:    key,
		member: member,
	}, nil
}

func (c *compiler) valueTypeForElement(decl *types.ElementDecl) (types.Type, error) {
	if decl == nil || decl.Type == nil {
		return nil, fmt.Errorf("missing type")
	}
	switch typed := decl.Type.(type) {
	case *types.SimpleType, *types.BuiltinType:
		return typed, nil
	case *types.ComplexType:
		textType, err := c.simpleContentTextType(typed)
		if err != nil {
			return nil, err
		}
		if textType == nil {
			return nil, fmt.Errorf("complex type has no simple content")
		}
		return textType, nil
	default:
		return nil, fmt.Errorf("unsupported element type")
	}
}

func (c *compiler) valueTypeForAttribute(decl *types.AttributeDecl) (types.Type, error) {
	if decl == nil {
		return nil, fmt.Errorf("missing attribute")
	}
	if decl.Type != nil {
		return decl.Type, nil
	}
	if decl.IsReference && c.schema != nil {
		if target := c.schema.AttributeDecls[decl.Name]; target != nil && target.Type != nil {
			return target.Type, nil
		}
	}
	return nil, fmt.Errorf("missing attribute type")
}

func (c *compiler) simpleContentTextType(ct *types.ComplexType) (types.Type, error) {
	return schemaops.ResolveSimpleContentTextType(ct, schemaops.SimpleContentTextTypeOptions{
		ResolveQName: c.res.resolveQName,
		Cache:        c.simpleContent,
	})
}
