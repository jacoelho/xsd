package schema

import (
	"github.com/jacoelho/xsd/xsderrors"
)

// schemaComponentKind identifies named schema components for compile-time
// reference diagnostics.
type schemaComponentKind uint8

const (
	schemaComponentSimpleTypeLabel     = "simple type"
	schemaComponentComplexTypeLabel    = "complex type"
	schemaComponentAttributeLabel      = "attribute" //nolint:goconst // Diagnostic labels remain separate from XML child vocabulary.
	schemaComponentElementLabel        = "element"   //nolint:goconst // Diagnostic labels remain separate from XML child vocabulary.
	schemaComponentAttributeGroupLabel = "attribute group"
	schemaComponentModelGroupLabel     = "model group"
	schemaComponentTypeLabel           = "type"
	schemaComponentLabel               = "schema component"
)

const (
	// schemaComponentSimpleType identifies global simple type declarations.
	schemaComponentSimpleType schemaComponentKind = iota
	// schemaComponentComplexType identifies global complex type declarations.
	schemaComponentComplexType
	// schemaComponentAttribute identifies global attribute declarations.
	schemaComponentAttribute
	// schemaComponentElement identifies global element declarations.
	schemaComponentElement
	// schemaComponentAttributeGroup identifies global attribute group declarations.
	schemaComponentAttributeGroup
	// schemaComponentModelGroup identifies global model group declarations.
	schemaComponentModelGroup
	// schemaComponentType identifies global type references that may resolve to simple or complex types.
	schemaComponentType
)

func (k schemaComponentKind) missingLabel() string {
	switch k {
	case schemaComponentSimpleType:
		return schemaComponentSimpleTypeLabel
	case schemaComponentComplexType:
		return schemaComponentComplexTypeLabel
	case schemaComponentAttribute:
		return schemaComponentAttributeLabel
	case schemaComponentElement:
		return schemaComponentElementLabel
	case schemaComponentAttributeGroup:
		return schemaComponentAttributeGroupLabel
	case schemaComponentModelGroup:
		return schemaComponentModelGroupLabel
	case schemaComponentType:
		return schemaComponentTypeLabel
	default:
		return schemaComponentLabel
	}
}

func (k schemaComponentKind) cycleLabel() string {
	switch k {
	case schemaComponentAttribute:
		return "attribute declaration"
	case schemaComponentElement:
		return "element declaration"
	case schemaComponentSimpleType, schemaComponentComplexType, schemaComponentAttributeGroup, schemaComponentModelGroup, schemaComponentType:
		return k.missingLabel()
	default:
	}
	return k.missingLabel()
}

// addSchemaComponent inserts one named schema component and rejects duplicates.
func addSchemaComponent[T any](components map[QName]T, name QName, component T, label string) error {
	if _, exists := components[name]; exists {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate schema component "+label)
	}
	components[name] = component
	return nil
}

// schemaComponentCycleError reports recursive compilation of one named schema
// component.
func schemaComponentCycleError(kind schemaComponentKind, label string) error {
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "cyclic "+kind.cycleLabel()+" "+label)
}

// schemaComponentRecursionError reports a recursive reference to one named
// schema component.
func schemaComponentRecursionError(kind schemaComponentKind, label string) error {
	msg := "recursive " + kind.missingLabel()
	if label == "" {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, msg)
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, msg+" "+label)
}

// schemaComponentMissingError reports an unknown named schema component.
func schemaComponentMissingError(kind schemaComponentKind, label string) error {
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unknown "+kind.missingLabel()+" "+label)
}

// schemaTypeNameConflictError reports a name already present in the shared
// simple/complex type symbol space.
func schemaTypeNameConflictError(label string) error {
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate type "+label)
}

// addNotation inserts one top-level notation declaration and rejects
// duplicate notation names.
func addNotation(notations map[QName]bool, name QName, label string) error {
	if notations[name] {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate notation "+label)
	}
	notations[name] = true
	return nil
}
