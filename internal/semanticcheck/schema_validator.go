package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/globaldecl"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/qname"
)

// ValidateStructure validates that a parsed schema conforms to XSD structural constraints.
// Reference validation is handled separately during the resolver phase.
func ValidateStructure(schema *parser.Schema) []error {
	var errors []error
	seen := make(map[parser.GlobalDeclKind]map[model.QName]struct{})

	markSeen := func(kind parser.GlobalDeclKind, name model.QName) bool {
		m := seen[kind]
		if m == nil {
			m = make(map[model.QName]struct{})
			seen[kind] = m
		}
		if _, ok := m[name]; ok {
			return false
		}
		m[name] = struct{}{}
		return true
	}

	_ = globaldecl.ForEach(schema, globaldecl.Handlers{
		Element: func(name model.QName, decl *model.ElementDecl) error {
			if !markSeen(parser.GlobalDeclElement, name) {
				return nil
			}
			if decl != nil {
				if err := validateElementDeclStructure(schema, name, decl); err != nil {
					errors = append(errors, fmt.Errorf("element %s: %w", name, err))
				}
			}
			return nil
		},
		Attribute: func(name model.QName, decl *model.AttributeDecl) error {
			if !markSeen(parser.GlobalDeclAttribute, name) {
				return nil
			}
			if decl != nil {
				if err := validateAttributeDeclStructure(schema, name, decl); err != nil {
					errors = append(errors, fmt.Errorf("attribute %s: %w", name, err))
				}
			}
			return nil
		},
		Type: func(name model.QName, typ model.Type) error {
			if !markSeen(parser.GlobalDeclType, name) {
				return nil
			}
			if typ != nil {
				if err := validateTypeDefStructure(schema, name, typ); err != nil {
					errors = append(errors, fmt.Errorf("type %s: %w", name, err))
				}
			}
			return nil
		},
		Group: func(name model.QName, group *model.ModelGroup) error {
			if !markSeen(parser.GlobalDeclGroup, name) {
				return nil
			}
			if group != nil {
				if err := validateGroupStructure(name, group); err != nil {
					errors = append(errors, fmt.Errorf("group %s: %w", name, err))
				}
			}
			return nil
		},
		AttributeGroup: func(name model.QName, group *model.AttributeGroup) error {
			if !markSeen(parser.GlobalDeclAttributeGroup, name) {
				return nil
			}
			if group != nil {
				if err := validateAttributeGroupStructure(schema, name, group); err != nil {
					errors = append(errors, fmt.Errorf("attributeGroup %s: %w", name, err))
				}
			}
			return nil
		},
		Notation: func(name model.QName, _ *model.NotationDecl) error {
			markSeen(parser.GlobalDeclNotation, name)
			return nil
		},
		Unknown: func(kind parser.GlobalDeclKind, name model.QName) error {
			markSeen(kind, name)
			return nil
		},
	})

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

func collectUnseenKeys[T any](kind parser.GlobalDeclKind, seen map[parser.GlobalDeclKind]map[model.QName]struct{}, m map[model.QName]T) []model.QName {
	if len(m) == 0 {
		return nil
	}
	seenKind := seen[kind]
	keys := make([]model.QName, 0, len(m))
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

func sortQNames(keys []model.QName) {
	qname.SortInPlace(keys)
}
