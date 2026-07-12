package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) addBuiltins() error {
	if err := c.addBuiltinSimpleTypes(); err != nil {
		return err
	}
	if err := c.addBuiltinXMLAttributes(); err != nil {
		return err
	}
	return c.addBuiltinAnyType()
}

func (c *compiler) addBuiltinSimpleTypes() error {
	for i := range runtime.BuiltinSimpleSeedCount() {
		seed, ok := runtime.BuiltinSimpleSeedAt(i)
		if !ok {
			return xsderrors.InternalInvariant("builtin simple type seed index is invalid")
		}
		if _, err := c.addBuiltinSimpleSeed(seed); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) addBuiltinSimpleSeed(seed *runtime.BuiltinSimpleSeed) (runtime.SimpleTypeID, error) {
	q, err := c.rt.internQName(seed.Namespace, seed.Local)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	id, err := c.registerBuiltinSimpleType(seed, q, c.builtinFacets.SimpleType(seed, q, seed.Base, seed.ListItem))
	if err != nil {
		return runtime.NoSimpleType, err
	}
	c.simpleDone[q] = id
	return id, nil
}

func (c *compiler) addBuiltinXMLAttributes() error {
	internal, err := c.addBuiltinAttributeSimpleTypes()
	if err != nil {
		return err
	}
	for i := range runtime.BuiltinAttributeCount() {
		attr, ok := runtime.BuiltinAttributeSeedAt(i)
		if !ok {
			return xsderrors.InternalInvariant("builtin attribute seed index is invalid")
		}
		typ, ok := attr.TypeID(c.rt.builtinIDs(), internal)
		if !ok {
			return xsderrors.InternalInvariant("builtin attribute references missing type: " + attr.Local)
		}
		if err := c.addBuiltinAttribute(attr.Namespace, attr.Local, typ); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) addBuiltinAttributeSimpleTypes() (runtime.BuiltinAttributeInternalTypes, error) {
	var internal runtime.BuiltinAttributeInternalTypes
	for i := range runtime.BuiltinAttributeSimpleSeedCount() {
		seed, ok := runtime.BuiltinAttributeSimpleSeedAt(i)
		if !ok {
			return runtime.BuiltinAttributeInternalTypes{}, xsderrors.InternalInvariant("builtin attribute simple type seed index is invalid")
		}
		id, err := c.addBuiltinAttributeSimpleSeed(seed)
		if err != nil {
			return runtime.BuiltinAttributeInternalTypes{}, err
		}
		seed.RecordID(&internal, id)
	}
	return internal, nil
}

func (c *compiler) addBuiltinAttributeSimpleSeed(seed runtime.BuiltinAttributeSimpleSeed) (runtime.SimpleTypeID, error) {
	base, ok := seed.BaseID(c.rt.builtinIDs())
	if !ok {
		return runtime.NoSimpleType, xsderrors.InternalInvariant("builtin attribute simple type references missing base: " + seed.Local)
	}
	q, err := c.rt.internQName(seed.Namespace, seed.Local)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	return c.addSimpleType(seed.SimpleType(q, base))
}

func (c *compiler) addBuiltinAnyType() error {
	anyWildcard, err := c.addWildcard(runtime.BuiltinAnyTypeWildcard())
	if err != nil {
		return err
	}
	attrSet, err := c.addAttributeUseSet(runtime.BuiltinAnyTypeAttributeUseSet(anyWildcard))
	if err != nil {
		return err
	}
	modelID, err := c.addModel(runtime.BuiltinAnyTypeContentModel())
	if err != nil {
		return err
	}
	q, err := c.rt.internQName(runtime.XSDNamespaceURI, runtime.BuiltinAnyTypeLocalName())
	if err != nil {
		return err
	}
	complexID, err := c.registerBuiltinAnyType(q, runtime.BuiltinAnyTypeComplexType(q, modelID, attrSet))
	if err != nil {
		return err
	}
	c.complexDone[q] = complexID
	return nil
}

func (c *compiler) addBuiltinAttribute(ns, local string, typ runtime.SimpleTypeID) error {
	q, err := c.rt.internQName(ns, local)
	if err != nil {
		return err
	}
	id, err := c.registerGlobalAttribute(q, runtime.AttributeDecl{Name: q, Type: typ})
	if err != nil {
		return err
	}
	c.attributeDone[q] = id
	return nil
}

func (c *compiler) missingSimpleType() (runtime.SimpleTypeID, error) {
	if c.missingSimple != runtime.NoSimpleType {
		return c.missingSimple, nil
	}
	q, err := c.rt.internQName(runtime.EmptyNamespaceURI, runtime.MissingSimpleTypeLocalName())
	if err != nil {
		return runtime.NoSimpleType, err
	}
	id, err := c.addSimpleType(runtime.MissingSimpleType(q, c.rt.builtinIDs().AnySimpleType))
	if err != nil {
		return runtime.NoSimpleType, err
	}
	c.missingSimple = id
	return id, nil
}
