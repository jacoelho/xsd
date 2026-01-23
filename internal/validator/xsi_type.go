package validator

import (
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

// lookupType finds a compiled type by QName, checking both schema types and built-in types.
func (r *validationRun) lookupType(qname types.QName) *grammar.CompiledType {
	if ct := r.schema.Type(qname); ct != nil {
		return ct
	}

	// check if it's a built-in type (for xsi:type references to types not explicitly used in schema)
	if bt := types.GetBuiltinNS(qname.Namespace, qname.Local); bt != nil {
		return r.validator.getBuiltinCompiledType(bt)
	}

	return nil
}

// typeDerivesFrom checks if derivedType derives from baseType.
func (r *validationRun) typeDerivesFrom(derivedType, baseType *grammar.CompiledType) bool {
	// all types derive from anyType per XSD spec
	if isAnyType(baseType) {
		return true
	}

	// all simple/built-in types derive from anySimpleType
	if isAnySimpleType(baseType) {
		if derivedType.Kind == grammar.TypeKindSimple {
			return true
		}
		if derivedType.Kind == grammar.TypeKindBuiltin {
			return !isAnyType(derivedType)
		}
		// complex types with simpleContent also derive from anySimpleType
		return derivedType.Kind == grammar.TypeKindComplex && derivedType.SimpleContentType != nil
	}

	for _, ancestor := range derivedType.DerivationChain {
		if ancestor.QName == baseType.QName {
			return true
		}
	}
	return false
}

// isTypeSubstitutionBlocked checks if block prevents using xsiType to substitute declaredType.
func (r *validationRun) isTypeSubstitutionBlocked(xsiType, declaredType *grammar.CompiledType, block types.DerivationSet) (bool, string) {
	current := xsiType
	for current != nil && current.QName != declaredType.QName {
		if block.Has(current.DerivationMethod) {
			switch current.DerivationMethod {
			case types.DerivationExtension:
				return true, "extension"
			case types.DerivationRestriction:
				return true, "restriction"
			}
		}
		current = current.BaseType
	}
	return false, ""
}
