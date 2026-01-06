package validator

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func (r *validationRun) resolveXsiType(elem xml.Element, xsiTypeValue string, declaredType *grammar.CompiledType, elemBlock types.DerivationSet) (*grammar.CompiledType, error) {
	xsiTypeQName, err := r.parseQNameValue(elem, xsiTypeValue)
	if err != nil {
		return nil, fmt.Errorf("invalid xsi:type value '%s': %w", strings.TrimSpace(xsiTypeValue), err)
	}

	xsiType := r.lookupType(xsiTypeQName)
	if xsiType == nil {
		return nil, fmt.Errorf("type '%s' not found in schema", xsiTypeQName.String())
	}

	// Check if type is abstract - abstract types cannot be instantiated
	if xsiType.Abstract {
		return nil, fmt.Errorf("type '%s' is abstract and cannot be used in xsi:type", xsiTypeQName.String())
	}

	// If xsi:type is the same as declared type, no substitution needed
	if xsiType.QName == declaredType.QName {
		return xsiType, nil
	}

	// Special handling for union types: xsi:type can specify any member type
	// or any type derived from a member type. Member types don't derive from
	// union types - they're just members. For nested unions, check recursively.
	if len(declaredType.MemberTypes) > 0 {
		// Check if xsi:type is or derives from any member type (recursively for nested unions)
		if r.isUnionMemberType(xsiType, declaredType) {
			return xsiType, nil
		}
		return nil, fmt.Errorf("type '%s' is not a member type of union '%s'",
			xsiTypeQName.String(), declaredType.QName.Local)
	}

	// Check if xsi:type derives from declared type
	if !r.typeDerivesFrom(xsiType, declaredType) {
		return nil, fmt.Errorf("type '%s' is not derived from '%s'",
			xsiTypeQName.String(), declaredType.QName.Local)
	}

	// Combine element block and type block
	combinedBlock := elemBlock.Add(types.DerivationMethod(declaredType.Block))

	// Check if block prevents this substitution
	if blocked, method := r.isTypeSubstitutionBlocked(xsiType, declaredType, combinedBlock); blocked {
		return nil, fmt.Errorf("type '%s' cannot substitute '%s': %s derivation is blocked",
			xsiTypeQName.String(), declaredType.QName.Local, method)
	}

	return xsiType, nil
}

// resolveXsiTypeOnly resolves an xsi:type attribute without a declared type.
// This is used when no element declaration exists but xsi:type is specified.
func (r *validationRun) resolveXsiTypeOnly(elem xml.Element, xsiTypeValue string) (*grammar.CompiledType, error) {
	xsiTypeQName, err := r.parseQNameValue(elem, xsiTypeValue)
	if err != nil {
		return nil, fmt.Errorf("invalid xsi:type value '%s': %w", strings.TrimSpace(xsiTypeValue), err)
	}

	xsiType := r.lookupType(xsiTypeQName)
	if xsiType == nil {
		return nil, fmt.Errorf("type '%s' not found in schema", xsiTypeQName.String())
	}

	return xsiType, nil
}

// lookupType finds a compiled type by QName, checking both schema types and built-in types.
func (r *validationRun) lookupType(qname types.QName) *grammar.CompiledType {
	// First check the schema's compiled types
	if ct := r.schema.Type(qname); ct != nil {
		return ct
	}

	// Check if it's a built-in type (for xsi:type references to types not explicitly used in schema)
	if bt := types.GetBuiltinNS(qname.Namespace, qname.Local); bt != nil {
		return r.validator.getBuiltinCompiledType(bt)
	}

	return nil
}

// checkElementWithType validates an element using only a type (no element declaration).
// This is used when xsi:type is specified on an undeclared element.
func (r *validationRun) checkElementWithType(elem xml.Element, ct *grammar.CompiledType, path string) []errors.Validation {
	// Use checkElementContent with nil declaration (for default handling)
	return r.checkElementContent(elem, ct, nil, path)
}

// parseQNameValue parses a QName value in the context of an element's namespace declarations.
func (r *validationRun) parseQNameValue(elem xml.Element, value string) (types.QName, error) {
	// Trim whitespace from the value per XSD spec (QName values should be normalized)
	value = strings.TrimSpace(value)

	var prefix, local string
	if before, after, ok := strings.Cut(value, ":"); ok {
		prefix = strings.TrimSpace(before)
		local = strings.TrimSpace(after)
	} else {
		local = value
	}

	var ns types.NamespaceURI
	if prefix != "" {
		nsStr := r.lookupNamespaceURI(elem, prefix)
		if nsStr == "" {
			return types.QName{}, fmt.Errorf("undefined namespace prefix '%s'", prefix)
		}
		ns = types.NamespaceURI(nsStr)
	} else {
		// No prefix - use default namespace if declared, otherwise no namespace
		// Per XSD spec, unprefixed QNames in xsi:type are resolved using namespace declarations
		defaultNS := r.lookupNamespaceURI(elem, "")
		if defaultNS != "" {
			ns = types.NamespaceURI(defaultNS)
		}
		// If no default namespace, ns remains empty (no namespace)
	}

	return types.QName{Namespace: ns, Local: local}, nil
}

// lookupNamespaceURI finds the namespace URI for a prefix by looking at xmlns attributes.
// Searches the element and all its ancestors for namespace declarations.
// Note: Go's encoding/xml reports xmlns declarations with NamespaceURI() == "xmlns"
// (not the full XMLNSNamespace URI), so we check for both.
func (r *validationRun) lookupNamespaceURI(elem xml.Element, prefix string) string {
	// Search this element and all ancestors for the namespace declaration
	for current := elem; current != nil; current = current.Parent() {
		for _, attr := range current.Attributes() {
			if prefix == "" {
				// Default namespace: xmlns="..."
				if attr.LocalName() == "xmlns" &&
					(attr.NamespaceURI() == "" ||
						attr.NamespaceURI() == "xmlns" ||
						attr.NamespaceURI() == xml.XMLNSNamespace) {
					return attr.Value()
				}
			} else {
				// Prefixed namespace: xmlns:prefix="..."
				// Go's encoding/xml reports these with Space="xmlns" (not the full URI)
				if attr.LocalName() == prefix &&
					(attr.NamespaceURI() == "xmlns" || attr.NamespaceURI() == xml.XMLNSNamespace) {
					return attr.Value()
				}
			}
		}
	}

	// For default namespace (prefix == ""):
	// If the element is in a non-empty namespace and we didn't find xmlns="...",
	// the element must be using the default namespace from an ancestor
	if prefix == "" && elem.NamespaceURI() != "" {
		// Check if any xmlns:prefix declaration on any ancestor binds to the element's namespace
		// If so, the element is using that prefix, not the default namespace
		elementNS := elem.NamespaceURI()
		for current := elem; current != nil; current = current.Parent() {
			for _, attr := range current.Attributes() {
				if (attr.NamespaceURI() == "xmlns" || attr.NamespaceURI() == xml.XMLNSNamespace) &&
					attr.LocalName() != "xmlns" && // Not the default namespace declaration
					attr.Value() == elementNS { // Prefixed declaration binds to element's namespace
					// Element has xmlns:prefix matching its namespace - it's using a prefix,
					// so there's no default namespace (from this element's perspective)
					return ""
				}
			}
		}
		// Element is in a namespace but no prefix declaration matches - must be using default NS
		return elementNS
	}

	return ""
}

// typeDerivesFrom checks if derivedType derives from baseType.
func (r *validationRun) typeDerivesFrom(derivedType, baseType *grammar.CompiledType) bool {
	// All types derive from anyType per XSD spec
	if isAnyType(baseType) {
		return true
	}

	// All simple/built-in types derive from anySimpleType
	if isAnySimpleType(baseType) {
		if derivedType.Kind == grammar.TypeKindSimple {
			return true
		}
		if derivedType.Kind == grammar.TypeKindBuiltin {
			return !isAnyType(derivedType)
		}
		// Complex types with simpleContent also derive from anySimpleType
		return derivedType.Kind == grammar.TypeKindComplex && derivedType.SimpleContentType != nil
	}

	for _, ancestor := range derivedType.DerivationChain {
		if ancestor.QName == baseType.QName {
			return true
		}
	}
	return false
}

// isUnionMemberType checks if xsiType is a member of the union type (recursively for nested unions).
func (r *validationRun) isUnionMemberType(xsiType, unionType *grammar.CompiledType) bool {
	if len(unionType.MemberTypes) == 0 {
		return false
	}

	for _, memberType := range unionType.MemberTypes {
		// Direct match or derivation
		if memberType.QName == xsiType.QName || r.typeDerivesFrom(xsiType, memberType) {
			return true
		}
		// Recursively check nested unions
		if len(memberType.MemberTypes) > 0 {
			if r.isUnionMemberType(xsiType, memberType) {
				return true
			}
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
