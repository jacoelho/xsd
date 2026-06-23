package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// SchemaComponentKind identifies named schema components for compile-time
// reference diagnostics.
type SchemaComponentKind uint8

const (
	schemaComponentSimpleTypeLabel     = "simple type"
	schemaComponentComplexTypeLabel    = "complex type"
	schemaComponentAttributeLabel      = "attribute"
	schemaComponentElementLabel        = "element"
	schemaComponentAttributeGroupLabel = "attribute group"
	schemaComponentModelGroupLabel     = "model group"
	schemaComponentTypeLabel           = "type"
	schemaComponentLabel               = "schema component"
)

const (
	// SchemaComponentSimpleType identifies global simple type declarations.
	SchemaComponentSimpleType SchemaComponentKind = iota
	// SchemaComponentComplexType identifies global complex type declarations.
	SchemaComponentComplexType
	// SchemaComponentAttribute identifies global attribute declarations.
	SchemaComponentAttribute
	// SchemaComponentElement identifies global element declarations.
	SchemaComponentElement
	// SchemaComponentAttributeGroup identifies global attribute group declarations.
	SchemaComponentAttributeGroup
	// SchemaComponentModelGroup identifies global model group declarations.
	SchemaComponentModelGroup
	// SchemaComponentType identifies global type references that may resolve to simple or complex types.
	SchemaComponentType
)

func (k SchemaComponentKind) missingLabel() string {
	switch k {
	case SchemaComponentSimpleType:
		return schemaComponentSimpleTypeLabel
	case SchemaComponentComplexType:
		return schemaComponentComplexTypeLabel
	case SchemaComponentAttribute:
		return schemaComponentAttributeLabel
	case SchemaComponentElement:
		return schemaComponentElementLabel
	case SchemaComponentAttributeGroup:
		return schemaComponentAttributeGroupLabel
	case SchemaComponentModelGroup:
		return schemaComponentModelGroupLabel
	case SchemaComponentType:
		return schemaComponentTypeLabel
	default:
		return schemaComponentLabel
	}
}

func (k SchemaComponentKind) cycleLabel() string {
	switch k {
	case SchemaComponentAttribute:
		return "attribute declaration"
	case SchemaComponentElement:
		return "element declaration"
	default:
		return k.missingLabel()
	}
}

// AddSchemaComponent inserts one named schema component and rejects duplicates.
func AddSchemaComponent[T any](components map[runtime.QName]T, name runtime.QName, component T, label string) error {
	if _, exists := components[name]; exists {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate schema component "+label)
	}
	components[name] = component
	return nil
}

// AddGlobalAttributeComponent inserts one top-level attribute declaration.
// Existing runtime global attributes are builtin declarations and take
// precedence over schema redeclarations.
func AddGlobalAttributeComponent[T any](
	components map[runtime.QName]T,
	globals map[runtime.QName]runtime.AttributeID,
	name runtime.QName,
	component T,
	label string,
) error {
	if _, exists := globals[name]; exists {
		return nil
	}
	return AddSchemaComponent(components, name, component, label)
}

// CheckSchemaComponentCycle rejects recursive compilation of one named schema
// component.
func CheckSchemaComponentCycle(kind SchemaComponentKind, compiling bool, label string) error {
	if compiling {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "cyclic "+kind.cycleLabel()+" "+label)
	}
	return nil
}

// CheckSchemaComponentRecursion rejects recursive references to one named
// schema component.
func CheckSchemaComponentRecursion(kind SchemaComponentKind, recursive bool, label string) error {
	if recursive {
		msg := "recursive " + kind.missingLabel()
		if label != "" {
			msg += " " + label
		}
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, msg)
	}
	return nil
}

// CheckSchemaComponentExists rejects references to unknown named schema
// components.
func CheckSchemaComponentExists(kind SchemaComponentKind, exists bool, label string) error {
	if !exists {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unknown "+kind.missingLabel()+" "+label)
	}
	return nil
}

// CheckSchemaTypeNameAvailable rejects a type name that is already used by the
// opposite simple/complex type table.
func CheckSchemaTypeNameAvailable(exists bool, label string) error {
	if exists {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate type "+label)
	}
	return nil
}

// AddNotation inserts one top-level notation declaration and rejects
// duplicate notation names.
func AddNotation(notations map[runtime.QName]bool, name runtime.QName, label string) error {
	if notations[name] {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate notation "+label)
	}
	notations[name] = true
	return nil
}
