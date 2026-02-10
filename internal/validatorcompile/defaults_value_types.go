package validatorcompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemaops"
)

func (c *compiler) compileDefaultFixedValue(lexical string, typ model.Type, ctx map[string]string) (compiledDefaultFixed, error) {
	canon, member, key, err := c.canonicalizeDefaultFixed(lexical, typ, ctx)
	if err != nil {
		return compiledDefaultFixed{}, err
	}
	return compiledDefaultFixed{
		ok:     true,
		ref:    c.values.addWithHash(canon, runtime.HashBytes(canon)),
		key:    key,
		member: member,
	}, nil
}

func (c *compiler) valueTypeForElement(decl *model.ElementDecl) (model.Type, error) {
	if decl == nil || decl.Type == nil {
		return nil, fmt.Errorf("missing type")
	}
	switch typed := decl.Type.(type) {
	case *model.SimpleType, *model.BuiltinType:
		return typed, nil
	case *model.ComplexType:
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

func (c *compiler) valueTypeForAttribute(decl *model.AttributeDecl) (model.Type, error) {
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

func (c *compiler) simpleContentTextType(ct *model.ComplexType) (model.Type, error) {
	return schemaops.ResolveSimpleContentTextType(ct, schemaops.SimpleContentTextTypeOptions{
		ResolveQName: c.res.resolveQName,
		Cache:        c.simpleContent,
	})
}
