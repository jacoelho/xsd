package semanticcheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateStructure validates that a parsed schema conforms to XSD structural constraints.
// Reference validation is handled separately during the resolver phase.
func ValidateStructure(schema *parser.Schema) []error {
	var errors []error
	seen := make(map[parser.GlobalDeclKind]map[types.QName]struct{})

	markSeen := func(kind parser.GlobalDeclKind, name types.QName) bool {
		m := seen[kind]
		if m == nil {
			m = make(map[types.QName]struct{})
			seen[kind] = m
		}
		if _, ok := m[name]; ok {
			return false
		}
		m[name] = struct{}{}
		return true
	}

	for _, decl := range schema.GlobalDecls {
		if !markSeen(decl.Kind, decl.Name) {
			continue
		}
		switch decl.Kind {
		case parser.GlobalDeclElement:
			if def, ok := schema.ElementDecls[decl.Name]; ok {
				if err := validateElementDeclStructure(schema, decl.Name, def); err != nil {
					errors = append(errors, fmt.Errorf("element %s: %w", decl.Name, err))
				}
			}
		case parser.GlobalDeclAttribute:
			if def, ok := schema.AttributeDecls[decl.Name]; ok {
				if err := validateAttributeDeclStructure(schema, decl.Name, def); err != nil {
					errors = append(errors, fmt.Errorf("attribute %s: %w", decl.Name, err))
				}
			}
		case parser.GlobalDeclType:
			if def, ok := schema.TypeDefs[decl.Name]; ok {
				if err := validateTypeDefStructure(schema, decl.Name, def); err != nil {
					errors = append(errors, fmt.Errorf("type %s: %w", decl.Name, err))
				}
			}
		case parser.GlobalDeclGroup:
			if def, ok := schema.Groups[decl.Name]; ok {
				if err := validateGroupStructure(decl.Name, def); err != nil {
					errors = append(errors, fmt.Errorf("group %s: %w", decl.Name, err))
				}
			}
		case parser.GlobalDeclAttributeGroup:
			if def, ok := schema.AttributeGroups[decl.Name]; ok {
				if err := validateAttributeGroupStructure(schema, decl.Name, def); err != nil {
					errors = append(errors, fmt.Errorf("attributeGroup %s: %w", decl.Name, err))
				}
			}
		}
	}

	elementKeys := collectUnseenKeys(parser.GlobalDeclElement, seen, schema.ElementDecls)
	for _, qname := range elementKeys {
		if err := validateElementDeclStructure(schema, qname, schema.ElementDecls[qname]); err != nil {
			errors = append(errors, fmt.Errorf("element %s: %w", qname, err))
		}
	}

	attrKeys := collectUnseenKeys(parser.GlobalDeclAttribute, seen, schema.AttributeDecls)
	for _, qname := range attrKeys {
		if err := validateAttributeDeclStructure(schema, qname, schema.AttributeDecls[qname]); err != nil {
			errors = append(errors, fmt.Errorf("attribute %s: %w", qname, err))
		}
	}

	typeKeys := collectUnseenKeys(parser.GlobalDeclType, seen, schema.TypeDefs)
	for _, qname := range typeKeys {
		if err := validateTypeDefStructure(schema, qname, schema.TypeDefs[qname]); err != nil {
			errors = append(errors, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	groupKeys := collectUnseenKeys(parser.GlobalDeclGroup, seen, schema.Groups)
	for _, qname := range groupKeys {
		if err := validateGroupStructure(qname, schema.Groups[qname]); err != nil {
			errors = append(errors, fmt.Errorf("group %s: %w", qname, err))
		}
	}

	attrGroupKeys := collectUnseenKeys(parser.GlobalDeclAttributeGroup, seen, schema.AttributeGroups)
	for _, qname := range attrGroupKeys {
		if err := validateAttributeGroupStructure(schema, qname, schema.AttributeGroups[qname]); err != nil {
			errors = append(errors, fmt.Errorf("attributeGroup %s: %w", qname, err))
		}
	}

	return errors
}

func collectUnseenKeys[T any](kind parser.GlobalDeclKind, seen map[parser.GlobalDeclKind]map[types.QName]struct{}, m map[types.QName]T) []types.QName {
	if len(m) == 0 {
		return nil
	}
	seenKind := seen[kind]
	keys := make([]types.QName, 0, len(m))
	for qname := range m {
		if seenKind != nil {
			if _, ok := seenKind[qname]; ok {
				continue
			}
		}
		keys = append(keys, qname)
	}
	sortQNames(keys)
	return keys
}

func sortQNames(keys []types.QName) {
	slices.SortFunc(keys, types.CompareQName)
}
