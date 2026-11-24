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
	// compiler-synthetic declarations without a global registry binding. A
	// non-global type is named noQName, except the intrinsics
	// intrinsicNamedSimpleType admits.
	declarationScopeNonGlobal
	// declarationScopeGlobal identifies declarations requiring an exact global
	// registry binding.
	declarationScopeGlobal
)

// elementDecl is the runtime record for one element declaration.
type elementDecl struct {
	ValueConstraint *valueConstraint
	Identity        []IdentityConstraintID
	Type            TypeID
	Name            QName
	SubstHead       ElementID
	Nillable        bool
	Abstract        bool
	Block           DerivationMask
	Final           DerivationMask
	Scope           declarationScope
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
	ValueConstraint *valueConstraint
	Name            QName
	Type            SimpleTypeID
}

// validateElementDeclRuntime validates element declaration metadata that can be
// expressed in runtime vocabulary.
func validateElementDeclRuntime(names *nameTable, decl elementDecl, limits declRefLimits) error {
	if names == nil || !names.ValidQName(decl.Name) || !validTypeID(decl.Type, limits.SimpleTypeCount, limits.ComplexTypeCount) {
		return newBuildDefect("element declaration references invalid name or type")
	}
	if !validElementBlockMask(decl.Block) {
		return newBuildDefect("element declaration block mask contains invalid derivation")
	}
	if !validElementFinalMask(decl.Final) {
		return newBuildDefect("element declaration final mask contains invalid derivation")
	}
	if decl.SubstHead != NoElement && !validElementID(decl.SubstHead, limits.ElementCount) {
		return newBuildDefect("element declaration references invalid substitution head")
	}
	return nil
}

// validateAttributeDeclRuntime validates attribute declaration metadata that can
// be expressed in runtime vocabulary.
func validateAttributeDeclRuntime(names *nameTable, decl attributeDecl, limits declRefLimits) error {
	if names == nil || !names.ValidQName(decl.Name) || !validSimpleTypeID(decl.Type, limits.SimpleTypeCount) {
		return newBuildDefect("attribute declaration references invalid name or type")
	}
	if err := validateAttributeName(names, decl.Name); err != nil {
		return err
	}
	// Start-tag admission rejects the name (checkAttributeDeclarationName);
	// a published declaration named xmlns is a corrupted build.
	if names.Local(decl.Name.Local) == vocab.XMLNSPrefix {
		return errors.New("attribute cannot be named xmlns")
	}
	if names.Namespace(decl.Name.Namespace) == vocab.XSINamespaceURI {
		if _, ok := builtinXSIAttributeType(names.Local(decl.Name.Local)); !ok {
			return errors.New("attribute target namespace cannot be XMLSchema-instance")
		}
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
		return newBuildDefect("attribute declaration references invalid name")
	}
	return nil
}

// validateValueConstraintIdentity rejects vc, the value constraint owner
// carries, when typ, the simple type that validates it, is or derives from
// xs:ID.
func validateValueConstraintIdentity(rt *schemaBuild, typ SimpleTypeID, vc *valueConstraint, owner valueConstraintOwner) error {
	if vc == nil || typ == NoSimpleType {
		return nil
	}
	if rt == nil {
		return newBuildDefect("declaration value constraint references invalid type")
	}
	carriesID, ok := rt.SimpleTypeCarriesID(typ)
	if !ok {
		return newBuildDefect("declaration value constraint references invalid type")
	}
	if carriesID {
		return errors.New("ID-typed " + owner.label() + " stores value constraint")
	}
	return nil
}
