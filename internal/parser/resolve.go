package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

func lookupTypeQName(schema *Schema, qname model.QName) model.Type {
	if builtinType := model.GetBuiltinNS(qname.Namespace, qname.Local); builtinType != nil {
		return builtinType
	}
	if schema == nil {
		return nil
	}
	if typeDef, ok := schema.TypeDefs[qname]; ok {
		return typeDef
	}
	return nil
}

// ResolveTypeQName resolves a type QName against built-ins and schema model.
func ResolveTypeQName(schema *Schema, qname model.QName) (model.Type, error) {
	if qname.IsZero() {
		return nil, nil
	}
	if resolvedType := lookupTypeQName(schema, qname); resolvedType != nil {
		return resolvedType, nil
	}
	return nil, fmt.Errorf("type %s not found", qname)
}

// ResolveTypeQNameAllowMissing resolves a type QName and returns nil when missing.
func ResolveTypeQNameAllowMissing(schema *Schema, qname model.QName) model.Type {
	if qname.IsZero() {
		return nil
	}
	return lookupTypeQName(schema, qname)
}

// ValidateTypeQName checks that qname resolves to a known type.
func ValidateTypeQName(schema *Schema, qname model.QName) error {
	if qname.IsZero() {
		return nil
	}
	_, err := ResolveTypeQName(schema, qname)
	if err == nil {
		return nil
	}
	if qname.Namespace == model.XSDNamespace {
		return fmt.Errorf("type '%s' not found in XSD namespace", qname.Local)
	}
	return err
}

// ResolveTypeReference resolves a type reference in schema validation contexts.
func ResolveTypeReference(schema *Schema, typ model.Type) model.Type {
	if typ == nil {
		return nil
	}
	simpleType, ok := typ.(*model.SimpleType)
	if !ok || !model.IsPlaceholderSimpleType(simpleType) {
		return typ
	}
	resolvedType, err := ResolveTypeQName(schema, simpleType.QName)
	if err != nil {
		return nil
	}
	return resolvedType
}

// ResolveTypeReferenceAllowMissing resolves a type reference and keeps unresolved placeholders.
func ResolveTypeReferenceAllowMissing(schema *Schema, typ model.Type) model.Type {
	if typ == nil {
		return nil
	}
	simpleType, ok := typ.(*model.SimpleType)
	if !ok || !model.IsPlaceholderSimpleType(simpleType) {
		return typ
	}
	if resolvedType := ResolveTypeQNameAllowMissing(schema, simpleType.QName); resolvedType != nil {
		return resolvedType
	}
	return typ
}

// ResolveSimpleTypeReferenceAllowMissing resolves a simple type QName when present.
func ResolveSimpleTypeReferenceAllowMissing(schema *Schema, qname model.QName) model.Type {
	return ResolveTypeQNameAllowMissing(schema, qname)
}

// ResolveSimpleContentBaseTypeFromContent resolves the base type of a simpleContent definition.
func ResolveSimpleContentBaseTypeFromContent(schema *Schema, sc *model.SimpleContent) model.Type {
	if sc == nil {
		return nil
	}
	var baseQName model.QName
	if sc.Extension != nil {
		baseQName = sc.Extension.Base
	} else if sc.Restriction != nil {
		baseQName = sc.Restriction.Base
	}
	return ResolveTypeQNameAllowMissing(schema, baseQName)
}

// ResolveUnionMemberTypes returns flattened member types for union simple model.
func ResolveUnionMemberTypes(schema *Schema, st *model.SimpleType) []model.Type {
	return model.UnionMemberTypesWithResolver(st, func(name model.QName) model.Type {
		return ResolveSimpleTypeReferenceAllowMissing(schema, name)
	})
}

// ResolveListItemType returns the list item type for explicit or derived list simple model.
func ResolveListItemType(schema *Schema, st *model.SimpleType) model.Type {
	if st == nil {
		return nil
	}
	if itemType, ok := model.ListItemTypeWithResolver(st, func(name model.QName) model.Type {
		return ResolveSimpleTypeReferenceAllowMissing(schema, name)
	}); ok {
		return itemType
	}
	return nil
}

// IsIDOnlyType reports whether the QName identifies xs:ID.
func IsIDOnlyType(qname model.QName) bool {
	return qname.Namespace == model.XSDNamespace && qname.Local == string(model.TypeNameID)
}

// IsIDOnlyDerivedType reports whether a simple type derives from xs:ID.
func IsIDOnlyDerivedType(schema *Schema, st *model.SimpleType) bool {
	if st == nil {
		return false
	}
	found := false
	Walk(st, func(current model.Type) model.Type {
		simple, ok := model.AsSimpleType(current)
		if !ok || simple == nil || simple.Restriction == nil {
			return nil
		}
		if simple.ResolvedBase != nil {
			return simple.ResolvedBase
		}
		baseQName := simple.Restriction.Base
		if baseQName.IsZero() {
			return nil
		}
		return ResolveSimpleTypeReferenceAllowMissing(schema, baseQName)
	}, func(current model.Type) bool {
		switch typed := current.(type) {
		case *model.BuiltinType:
			found = IsIDOnlyType(typed.Name())
			return false
		case *model.SimpleType:
			if typed.Restriction == nil {
				return false
			}
			if IsIDOnlyType(typed.Restriction.Base) {
				found = true
				return false
			}
			return true
		default:
			return false
		}
	})
	return found
}
