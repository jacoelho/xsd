package typeresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// TypeReferencePolicy controls how missing type references are handled.
type TypeReferencePolicy int

const (
	// TypeReferenceMustExist requires referenced types to resolve.
	TypeReferenceMustExist TypeReferencePolicy = iota
	// TypeReferenceAllowMissing allows unresolved placeholders to pass through.
	TypeReferenceAllowMissing
)

// ResolveTypeQName resolves a type QName against built-ins and schema model.
func ResolveTypeQName(schema *parser.Schema, qname model.QName, policy TypeReferencePolicy) (model.Type, error) {
	if qname.IsZero() {
		return nil, nil
	}
	if builtinType := builtins.GetNS(qname.Namespace, qname.Local); builtinType != nil {
		return builtinType, nil
	}
	if schema != nil {
		if typeDef, ok := schema.TypeDefs[qname]; ok {
			return typeDef, nil
		}
	}
	if policy == TypeReferenceAllowMissing {
		return nil, nil
	}
	return nil, fmt.Errorf("type %s not found", qname)
}

// ResolveTypeReference resolves a type reference in schema validation contexts.
func ResolveTypeReference(schema *parser.Schema, typ model.Type, policy TypeReferencePolicy) model.Type {
	if typ == nil {
		return nil
	}
	if simpleType, ok := typ.(*model.SimpleType); ok && model.IsPlaceholderSimpleType(simpleType) {
		resolvedType, err := ResolveTypeQName(schema, simpleType.QName, policy)
		if err != nil {
			return nil
		}
		if resolvedType == nil && policy == TypeReferenceAllowMissing {
			return typ
		}
		return resolvedType
	}
	return typ
}

// ResolveSimpleTypeReferenceAllowMissing resolves a simple type QName when present.
func ResolveSimpleTypeReferenceAllowMissing(schema *parser.Schema, qname model.QName) model.Type {
	resolved, err := ResolveTypeQName(schema, qname, TypeReferenceAllowMissing)
	if err != nil {
		return nil
	}
	return resolved
}

// ResolveSimpleContentBaseTypeFromContent resolves the base type of a simpleContent definition.
func ResolveSimpleContentBaseTypeFromContent(schema *parser.Schema, sc *model.SimpleContent) model.Type {
	if sc == nil {
		return nil
	}
	var baseQName model.QName
	if sc.Extension != nil {
		baseQName = sc.Extension.Base
	} else if sc.Restriction != nil {
		baseQName = sc.Restriction.Base
	}
	if baseQName.IsZero() {
		return nil
	}
	if bt := builtins.GetNS(baseQName.Namespace, baseQName.Local); bt != nil {
		return bt
	}
	if schema == nil {
		return nil
	}
	if resolvedType, ok := schema.TypeDefs[baseQName]; ok {
		return resolvedType
	}
	return nil
}

// ResolveUnionMemberTypes returns flattened member types for union simple model.
func ResolveUnionMemberTypes(schema *parser.Schema, st *model.SimpleType) []model.Type {
	return model.UnionMemberTypesWithResolver(st, func(name model.QName) model.Type {
		return ResolveSimpleTypeReferenceAllowMissing(schema, name)
	})
}

// ResolveListItemType returns the list item type for explicit or derived list simple model.
func ResolveListItemType(schema *parser.Schema, st *model.SimpleType) model.Type {
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
func IsIDOnlyDerivedType(schema *parser.Schema, st *model.SimpleType) bool {
	visited := make(map[*model.SimpleType]bool)
	var visit func(*model.SimpleType) bool
	visit = func(current *model.SimpleType) bool {
		if current == nil || current.Restriction == nil {
			return false
		}
		if visited[current] {
			return false
		}
		visited[current] = true
		defer delete(visited, current)

		baseQName := current.Restriction.Base
		if IsIDOnlyType(baseQName) {
			return true
		}

		var baseType model.Type
		if current.ResolvedBase != nil {
			baseType = current.ResolvedBase
		} else if !baseQName.IsZero() {
			baseType = ResolveSimpleTypeReferenceAllowMissing(schema, baseQName)
		}

		switch typed := baseType.(type) {
		case *model.SimpleType:
			return visit(typed)
		case *model.BuiltinType:
			return IsIDOnlyType(typed.Name())
		default:
			return false
		}
	}

	return visit(st)
}
