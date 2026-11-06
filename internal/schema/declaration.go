package schema

import (
	"errors"

	"github.com/jacoelho/xsd/internal/vocab"
)

// declRefLimits are table sizes used to validate declaration references in a
// frozen runtime schema.
type declRefLimits struct {
	SimpleTypeCount  int
	ComplexTypeCount int
	ElementCount     int
}

// declarationScope records whether a compiler-owned declaration must have an
// exact global registry binding. Publication erases this construction
// provenance from validation-facing read projections.
type declarationScope uint8

const (
	// declarationScopeInvalid is the unset construction-provenance state.
	declarationScopeInvalid declarationScope = iota
	// declarationScopeNonGlobal identifies local, anonymous, sentinel, or
	// compiler-synthetic declarations without a global registry binding.
	declarationScopeNonGlobal
	// declarationScopeGlobal identifies declarations requiring an exact global
	// registry binding.
	declarationScopeGlobal
)

// elementDecl is the runtime record for one element declaration.
type elementDecl struct {
	Default   *valueConstraint
	Fixed     *valueConstraint
	Identity  []IdentityConstraintID
	Type      TypeID
	Name      QName
	SubstHead ElementID
	Nillable  bool
	Abstract  bool
	Block     DerivationMask
	Final     DerivationMask
	Scope     declarationScope
}

// elementDeclByID resolves an element declaration ID against an element table.
func elementDeclByID(decls []elementDecl, id ElementID) (*elementDecl, bool) {
	if !validElementID(id, len(decls)) {
		return nil, false
	}
	return &decls[id], true
}

// elementTypeByID resolves an element declaration's type against an element
// table.
func elementTypeByID(decls []elementDecl, id ElementID) (TypeID, bool) {
	decl, ok := elementDeclByID(decls, id)
	if !ok {
		return TypeID{}, false
	}
	return decl.Type, true
}

// attributeDecl is the runtime record for one attribute declaration.
type attributeDecl struct {
	Default *valueConstraint
	Fixed   *valueConstraint
	Name    QName
	Type    SimpleTypeID
}

// validateElementDeclRuntime validates element declaration metadata that can be
// expressed in runtime vocabulary.
func validateElementDeclRuntime(names *nameTable, decl elementDecl, limits declRefLimits) error {
	if names == nil || !names.ValidQName(decl.Name) || !validTypeID(decl.Type, limits.SimpleTypeCount, limits.ComplexTypeCount) {
		return errors.New("element declaration references invalid name or type")
	}
	if !validElementBlockMask(decl.Block) {
		return errors.New("element declaration block mask contains invalid derivation")
	}
	if !validElementFinalMask(decl.Final) {
		return errors.New("element declaration final mask contains invalid derivation")
	}
	if decl.SubstHead != NoElement && !validElementID(decl.SubstHead, limits.ElementCount) {
		return errors.New("element declaration references invalid substitution head")
	}
	if decl.Default != nil && decl.Fixed != nil {
		return errors.New("element declaration stores both default and fixed value constraints")
	}
	return nil
}

// validateAttributeDeclRuntime validates attribute declaration metadata that can
// be expressed in runtime vocabulary.
func validateAttributeDeclRuntime(names *nameTable, decl attributeDecl, limits declRefLimits) error {
	if names == nil || !names.ValidQName(decl.Name) || !validSimpleTypeID(decl.Type, limits.SimpleTypeCount) {
		return errors.New("attribute declaration references invalid name or type")
	}
	if err := validateAttributeName(names, decl.Name); err != nil {
		return err
	}
	if names.Namespace(decl.Name.Namespace) == vocab.XSINamespaceURI {
		if _, ok := builtinXSIAttributeType(names.Local(decl.Name.Local)); !ok {
			return errors.New("attribute target namespace cannot be XMLSchema-instance")
		}
	}
	if decl.Default != nil && decl.Fixed != nil {
		return errors.New("attribute declaration stores both default and fixed value constraints")
	}
	return nil
}

// validateAttributeDeclName validates reserved attribute declaration names.
func validateAttributeDeclName(names *nameTable, name QName) error {
	if err := validateAttributeName(names, name); err != nil {
		return err
	}
	if names.Namespace(name.Namespace) == vocab.XSINamespaceURI {
		return errors.New("attribute target namespace cannot be XMLSchema-instance")
	}
	return nil
}

func validateAttributeName(names *nameTable, name QName) error {
	if names == nil || !names.ValidQName(name) {
		return errors.New("attribute declaration references invalid name")
	}
	if names.Local(name.Local) == vocab.XMLNSPrefix {
		return errors.New("attribute cannot be named xmlns")
	}
	return nil
}

// declarationValueConstraint classifies the value constraints on a
// declaration.
type declarationValueConstraint uint8

const (
	// declarationValueConstraintNone means the declaration has no constraint.
	declarationValueConstraintNone declarationValueConstraint = iota
	// declarationValueConstraintDefault means the declaration has a default.
	declarationValueConstraintDefault
	// declarationValueConstraintFixed means the declaration has a fixed value.
	declarationValueConstraintFixed
	// declarationValueConstraintConflict means both mutually exclusive
	// constraints were supplied.
	declarationValueConstraintConflict
)

func (c declarationValueConstraint) present() bool {
	return c == declarationValueConstraintDefault || c == declarationValueConstraintFixed
}

// declarationValueConstraintOf classifies declaration value-constraint values.
func declarationValueConstraintOf[T any](defaultValue, fixedValue *T) declarationValueConstraint {
	switch {
	case defaultValue != nil && fixedValue != nil:
		return declarationValueConstraintConflict
	case defaultValue != nil:
		return declarationValueConstraintDefault
	case fixedValue != nil:
		return declarationValueConstraintFixed
	default:
		return declarationValueConstraintNone
	}
}

// validateElementDeclValueConstraintRuntime validates element declaration
// value-constraint rules that depend on the constraint's simple owner type.
func validateElementDeclValueConstraintRuntime(rt *schemaBuild, typ SimpleTypeID, constraint declarationValueConstraint) error {
	return validateDeclValueConstraintIdentity(rt, typ, constraint, "ID-typed element declaration stores value constraint")
}

// validateAttributeDeclValueConstraintRuntime validates attribute declaration
// value-constraint rules that depend on the declaration's simple type.
func validateAttributeDeclValueConstraintRuntime(rt *schemaBuild, typ SimpleTypeID, constraint declarationValueConstraint) error {
	return validateDeclValueConstraintIdentity(rt, typ, constraint, "ID-typed attribute declaration stores value constraint")
}

func validateDeclValueConstraintIdentity(rt *schemaBuild, typ SimpleTypeID, constraint declarationValueConstraint, msg string) error {
	switch constraint {
	case declarationValueConstraintNone:
		return nil
	case declarationValueConstraintDefault, declarationValueConstraintFixed:
	case declarationValueConstraintConflict:
		return errors.New("declaration stores conflicting value constraints")
	default:
		return errors.New("declaration value constraint kind is unknown")
	}
	if typ == NoSimpleType {
		return nil
	}
	if rt == nil {
		return errors.New("declaration value constraint references invalid type")
	}
	carriesID, ok := rt.SimpleTypeCarriesID(typ)
	if !ok {
		return errors.New("declaration value constraint references invalid type")
	}
	if carriesID {
		return errors.New(msg)
	}
	return nil
}
