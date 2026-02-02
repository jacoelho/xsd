package schemacheck

import (
	"fmt"
	"sort"

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

	collectSorted := func(keys []types.QName) []types.QName {
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].Namespace != keys[j].Namespace {
				return keys[i].Namespace < keys[j].Namespace
			}
			return keys[i].Local < keys[j].Local
		})
		return keys
	}

	var elementKeys []types.QName
	for qname := range schema.ElementDecls {
		if seen[parser.GlobalDeclElement] != nil {
			if _, ok := seen[parser.GlobalDeclElement][qname]; ok {
				continue
			}
		}
		elementKeys = append(elementKeys, qname)
	}
	for _, qname := range collectSorted(elementKeys) {
		if err := validateElementDeclStructure(schema, qname, schema.ElementDecls[qname]); err != nil {
			errors = append(errors, fmt.Errorf("element %s: %w", qname, err))
		}
	}

	var attrKeys []types.QName
	for qname := range schema.AttributeDecls {
		if seen[parser.GlobalDeclAttribute] != nil {
			if _, ok := seen[parser.GlobalDeclAttribute][qname]; ok {
				continue
			}
		}
		attrKeys = append(attrKeys, qname)
	}
	for _, qname := range collectSorted(attrKeys) {
		if err := validateAttributeDeclStructure(schema, qname, schema.AttributeDecls[qname]); err != nil {
			errors = append(errors, fmt.Errorf("attribute %s: %w", qname, err))
		}
	}

	var typeKeys []types.QName
	for qname := range schema.TypeDefs {
		if seen[parser.GlobalDeclType] != nil {
			if _, ok := seen[parser.GlobalDeclType][qname]; ok {
				continue
			}
		}
		typeKeys = append(typeKeys, qname)
	}
	for _, qname := range collectSorted(typeKeys) {
		if err := validateTypeDefStructure(schema, qname, schema.TypeDefs[qname]); err != nil {
			errors = append(errors, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	var groupKeys []types.QName
	for qname := range schema.Groups {
		if seen[parser.GlobalDeclGroup] != nil {
			if _, ok := seen[parser.GlobalDeclGroup][qname]; ok {
				continue
			}
		}
		groupKeys = append(groupKeys, qname)
	}
	for _, qname := range collectSorted(groupKeys) {
		if err := validateGroupStructure(qname, schema.Groups[qname]); err != nil {
			errors = append(errors, fmt.Errorf("group %s: %w", qname, err))
		}
	}

	var attrGroupKeys []types.QName
	for qname := range schema.AttributeGroups {
		if seen[parser.GlobalDeclAttributeGroup] != nil {
			if _, ok := seen[parser.GlobalDeclAttributeGroup][qname]; ok {
				continue
			}
		}
		attrGroupKeys = append(attrGroupKeys, qname)
	}
	for _, qname := range collectSorted(attrGroupKeys) {
		if err := validateAttributeGroupStructure(schema, qname, schema.AttributeGroups[qname]); err != nil {
			errors = append(errors, fmt.Errorf("attributeGroup %s: %w", qname, err))
		}
	}

	return errors
}
