package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func newCompilerSchemaBuild(names runtime.NameTable) runtime.SchemaBuild {
	return runtime.SchemaBuild{
		Names:            names,
		GlobalElements:   make(map[runtime.QName]runtime.ElementID),
		GlobalAttributes: make(map[runtime.QName]runtime.AttributeID, runtime.BuiltinAttributeCount()),
		GlobalTypes:      make(map[runtime.QName]runtime.TypeID, runtime.BuiltinGlobalTypeCount()),
		GlobalIdentities: make(map[runtime.QName]runtime.IdentityConstraintID),
		Notations:        make(map[runtime.QName]bool),
		Substitutions:    make(map[runtime.ElementID][]runtime.ElementID),
		SimpleTypes:      make([]runtime.SimpleType, 0, runtime.BuiltinSimpleTypeCount()),
		Attributes:       make([]runtime.AttributeDecl, 0, runtime.BuiltinAttributeCount()),
		ComplexTypes:     make([]runtime.ComplexType, 0, runtime.BuiltinComplexTypeCount()),
		Wildcards:        make([]runtime.Wildcard, 0, 1),
		AttributeUseSets: make([]runtime.AttributeUseSet, 0, 1),
		Models:           make([]runtime.ContentModel, 0, 1),
	}
}

func (c *compiler) registerGlobalElement(q runtime.QName, decl runtime.ElementDecl) (runtime.ElementID, error) {
	id, err := c.addElement(decl)
	if err != nil {
		return runtime.NoElement, err
	}
	c.rt.GlobalElements[q] = id
	return id, nil
}

func (c *compiler) registerGlobalAttribute(q runtime.QName, decl runtime.AttributeDecl) (runtime.AttributeID, error) {
	id, err := NextAttributeID(len(c.rt.Attributes))
	if err != nil {
		return 0, err
	}
	c.rt.Attributes = append(c.rt.Attributes, decl)
	c.rt.GlobalAttributes[q] = id
	return id, nil
}

func (c *compiler) registerGlobalComplexType(q runtime.QName, typ runtime.ComplexType) (runtime.ComplexTypeID, error) {
	id, err := c.addComplexType(typ)
	if err != nil {
		return runtime.NoComplexType, err
	}
	c.rt.GlobalTypes[q] = runtime.ComplexRef(id)
	return id, nil
}

func (c *compiler) registerGlobalSimpleType(q runtime.QName, typ runtime.SimpleType) (runtime.SimpleTypeID, error) {
	id, err := c.addSimpleType(typ)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	c.rt.GlobalTypes[q] = runtime.SimpleRef(id)
	return id, nil
}

func (c *compiler) registerGlobalIdentity(q runtime.QName, identity runtime.IdentityConstraint) (runtime.IdentityConstraintID, error) {
	id, err := NextIdentityConstraintID(len(c.rt.Identities))
	if err != nil {
		return runtime.NoIdentityConstraint, err
	}
	c.rt.Identities = append(c.rt.Identities, identity)
	c.rt.GlobalIdentities[q] = id
	return id, nil
}

func (c *compiler) addElement(decl runtime.ElementDecl) (runtime.ElementID, error) {
	id, err := NextElementID(len(c.rt.Elements))
	if err != nil {
		return runtime.NoElement, err
	}
	c.rt.Elements = append(c.rt.Elements, decl)
	return id, nil
}

func (c *compiler) completeElement(id runtime.ElementID, decl runtime.ElementDecl) {
	c.rt.Elements[id] = decl
}

func (c *compiler) addComplexType(typ runtime.ComplexType) (runtime.ComplexTypeID, error) {
	id, err := NextComplexTypeID(len(c.rt.ComplexTypes))
	if err != nil {
		return runtime.NoComplexType, err
	}
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, typ)
	return id, nil
}

func (c *compiler) completeComplexType(id runtime.ComplexTypeID, typ runtime.ComplexType) {
	c.rt.ComplexTypes[id] = typ
}

func (c *compiler) addSimpleType(typ runtime.SimpleType) (runtime.SimpleTypeID, error) {
	id, err := NextSimpleTypeID(len(c.rt.SimpleTypes))
	if err != nil {
		return runtime.NoSimpleType, err
	}
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, typ)
	return id, nil
}

func (c *compiler) completeSimpleType(id runtime.SimpleTypeID, typ runtime.SimpleType) {
	c.rt.SimpleTypes[id] = typ
}

func (c *compiler) completeIdentity(id runtime.IdentityConstraintID, identity runtime.IdentityConstraint) {
	c.rt.Identities[id] = identity
}

func (c *compiler) addWildcard(wildcard runtime.Wildcard) (runtime.WildcardID, error) {
	id, err := NextWildcardID(len(c.rt.Wildcards))
	if err != nil {
		return runtime.NoWildcard, err
	}
	c.rt.Wildcards = append(c.rt.Wildcards, wildcard)
	return id, nil
}

func (c *compiler) addAttributeUseSet(set runtime.AttributeUseSet) (runtime.AttributeUseSetID, error) {
	id, err := NextAttributeUseSetID(len(c.rt.AttributeUseSets))
	if err != nil {
		return runtime.NoAttributeUseSet, err
	}
	c.rt.AttributeUseSets = append(c.rt.AttributeUseSets, set)
	return id, nil
}

func (c *compiler) addModel(model runtime.ContentModel) (runtime.ContentModelID, error) {
	id, err := NextContentModelID(len(c.rt.Models))
	if err != nil {
		return runtime.NoContentModel, err
	}
	c.rt.Models = append(c.rt.Models, model)
	return id, nil
}

func (c *compiler) completeModel(id runtime.ContentModelID, model runtime.ContentModel) {
	c.rt.Models[id] = model
}

func (c *compiler) installCompiledModels(models []runtime.CompiledModel) error {
	if len(models) != len(c.rt.Models) {
		return xsderrors.InternalInvariant("compiled content model count does not match source models")
	}
	c.rt.CompiledModels = models
	return nil
}

func (c *compiler) installSubstitutions(substitutions map[runtime.ElementID][]runtime.ElementID) {
	c.rt.Substitutions = substitutions
	c.rt.SubstitutionIndex = runtime.BuildSubstitutionIndex(&c.rt, c.rt.Elements, substitutions)
}

func (c *compiler) indexGlobalAttribute(q runtime.QName, component rawComponent, label string) error {
	return AddGlobalAttributeComponent(c.attributeRaw, c.rt.GlobalAttributes, q, component, label)
}

func (c *compiler) addNotation(q runtime.QName, label string) error {
	return AddNotation(c.rt.Notations, q, label)
}

func (c *compiler) registerBuiltinSimpleType(seed *runtime.BuiltinSimpleSeed, q runtime.QName, typ runtime.SimpleType) (runtime.SimpleTypeID, error) {
	id, err := c.registerGlobalSimpleType(q, typ)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	seed.RecordID(&c.rt.Builtin, id)
	return id, nil
}

func (c *compiler) registerBuiltinAnyType(q runtime.QName, typ runtime.ComplexType) (runtime.ComplexTypeID, error) {
	id, err := c.registerGlobalComplexType(q, typ)
	if err != nil {
		return runtime.NoComplexType, err
	}
	c.rt.Builtin.AnyType = id
	return id, nil
}

func (c *compiler) publishSchema() (*runtime.Schema, error) {
	published, err := runtime.PublishSchema(&c.rt)
	if err != nil {
		return nil, err
	}
	c.names = NameInterner{}
	return published, nil
}
