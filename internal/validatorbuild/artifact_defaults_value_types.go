package validatorbuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *artifactCompiler) compileDefaultFixedValue(lexical string, typ model.Type, ctx map[string]string) (compiledDefaultFixed, error) {
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

func (c *artifactCompiler) valueTypeForElement(decl *model.ElementDecl) (model.Type, error) {
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

func (c *artifactCompiler) valueTypeForAttribute(decl *model.AttributeDecl) (model.Type, error) {
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

func (c *artifactCompiler) simpleContentTextType(ct *model.ComplexType) (model.Type, error) {
	if ct == nil {
		return nil, nil
	}
	if c == nil || c.complexTypes == nil {
		return nil, fmt.Errorf("complex types are nil")
	}
	textType, ok := c.complexTypes.SimpleContentType(ct)
	if !ok {
		return nil, fmt.Errorf("complex type %s missing prepared simple-content entry", ct.QName)
	}
	return textType, nil
}
