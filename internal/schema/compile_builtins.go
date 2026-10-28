package schema

import (
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) addBuiltins() error {
	// The value package owns the fixed type IDs and definitions. Schema keeps
	// only declaration records needed for QName lookup, derivation policy, and
	// publication; it never allocates or completes a value builtin.
	c.rt.AnyType = NoComplexType
	if err := c.addBuiltinSimpleTypes(); err != nil {
		return err
	}
	if err := c.addBuiltinXSIAttributes(); err != nil {
		return err
	}
	return c.addBuiltinAnyType()
}

func (c *compiler) addBuiltinXSIAttributes() error {
	name, err := c.rt.internQName(vocab.XSINamespaceURI, "$schemaLocation")
	if err != nil {
		return err
	}
	list := SimpleType{Name: name, ValueSpec: builtinSchemaLocationTypeSpec()}
	id, err := c.addSimpleType(list)
	if err != nil {
		return err
	}
	if id != builtinSchemaLocationTypeID {
		return xsderrors.InternalInvariant("intrinsic schemaLocation type ID diverged")
	}
	if err := c.completeSimpleType(id, list); err != nil {
		return err
	}
	for _, attribute := range builtinXSIAttributes {
		q, err := c.rt.internQName(vocab.XSINamespaceURI, attribute.local)
		if err != nil {
			return err
		}
		if _, err := c.registerGlobalAttribute(q, AttributeDecl{Name: q, Type: attribute.typ}); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) addBuiltinSimpleTypes() error {
	builder := c.ensureValueBuilder()
	for raw := range value.BuiltinTypeCount {
		if err := c.addBuiltinSimpleType(builder, raw); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) addBuiltinSimpleType(builder *value.Builder, id SimpleTypeID) error {
	name, ok := value.BuiltinTypeName(id)
	if !ok {
		return xsderrors.InternalInvariant("canonical builtin type has no schema declaration")
	}
	q, err := c.rt.internQName(vocab.XSDNamespaceURI, name)
	if err != nil {
		return err
	}
	view, ok := builder.TypeView(id)
	if !ok {
		return xsderrors.InternalInvariant("canonical builtin type has no value metadata: " + name)
	}
	st := SimpleType{
		ValueSpec: value.TypeSpec{
			Variety:           view.Variety,
			Primitive:         view.Primitive,
			Whitespace:        view.Whitespace,
			WhitespacePresent: true,
			Builtin:           view.Builtin,
			Base:              view.Base,
			ListItem:          view.ListItem,
			Union:             append([]value.TypeID(nil), view.Union...),
		},
		Name:  q,
		Scope: DeclarationScopeGlobal,
	}
	got, err := c.addSimpleType(st)
	if err != nil {
		return err
	}
	if got != id {
		return xsderrors.InternalInvariant("schema and canonical builtin type IDs diverged")
	}
	// addSimpleType starts declarations as non-global; builtins have global
	// names fixed by the value program.
	c.rt.SimpleTypes[id].Scope = DeclarationScopeGlobal
	c.rt.GlobalTypes[q] = SimpleRef(id)
	return nil
}

func (c *compiler) addBuiltinAnyType() error {
	anyWildcard, err := c.appendWildcard(BuiltinAnyTypeWildcard())
	if err != nil {
		return err
	}
	attrSet, err := c.addAttributeUseSet(BuiltinAnyTypeAttributeUseSet(anyWildcard))
	if err != nil {
		return err
	}
	modelID, err := c.addModel(BuiltinAnyTypeContentModel())
	if err != nil {
		return err
	}
	q, err := c.rt.internQName(vocab.XSDNamespaceURI, BuiltinAnyTypeLocalName())
	if err != nil {
		return err
	}
	_, err = c.registerBuiltinAnyType(q, BuiltinAnyTypeComplexType(q, modelID, attrSet))
	return err
}

func (c *compiler) missingSimpleType() (SimpleTypeID, error) {
	if c.missingSimple != NoSimpleType {
		return c.missingSimple, nil
	}
	q, err := c.rt.internQName(vocab.EmptyNamespaceURI, MissingSimpleTypeLocalName())
	if err != nil {
		return NoSimpleType, err
	}
	typ := MissingSimpleType(q, value.AnySimpleTypeID)
	id, err := c.addSimpleType(typ)
	if err != nil {
		return NoSimpleType, err
	}
	if err := c.completeSimpleType(id, typ); err != nil {
		return NoSimpleType, err
	}
	c.missingSimple = id
	return id, nil
}
