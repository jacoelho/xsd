package semantics

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// DetectCycles validates that type derivation, group refs, attribute group refs,
// and substitution groups are acyclic.
func DetectCycles(schema *parser.Schema) error {
	if err := requireResolved(schema); err != nil {
		return err
	}
	if err := validatePreparedSchemaInput(schema); err != nil {
		return err
	}

	if err := detectTypeCycles(schema); err != nil {
		return err
	}
	if err := detectGroupCycles(schema); err != nil {
		return err
	}
	if err := detectAttributeGroupCycles(schema); err != nil {
		return err
	}
	return detectSubstitutionGroupCycles(schema)
}

func detectTypeCycles(schema *parser.Schema) error {
	starts := make([]model.QName, 0, len(schema.TypeDefs))
	if err := parser.ForEachGlobalDecl(&schema.SchemaGraph, parser.GlobalDeclHandlers{
		Type: func(name model.QName, typ model.Type) error {
			if typ == nil {
				return fmt.Errorf("missing global type %s", name)
			}
			starts = append(starts, name)
			return nil
		},
	}); err != nil {
		return err
	}

	err := DetectGraphCycle(GraphCycleConfig[model.QName]{
		Starts:  starts,
		Missing: GraphCycleMissingPolicyError,
		Exists: func(name model.QName) bool {
			return schema.TypeDefs[name] != nil
		},
		Next: func(name model.QName) ([]model.QName, error) {
			typ := schema.TypeDefs[name]
			if typ == nil {
				return nil, nil
			}
			baseType, _, err := baseTypeFor(schema, typ)
			if err != nil {
				return nil, fmt.Errorf("type %s: %w", name, err)
			}
			if baseType == nil {
				return nil, nil
			}
			baseName := baseType.Name()
			if baseName.IsZero() || baseName.Namespace == model.XSDNamespace {
				return nil, nil
			}
			return []model.QName{baseName}, nil
		},
	})
	if err == nil {
		return nil
	}
	var cycleErr GraphCycleError[model.QName]
	if errors.As(err, &cycleErr) {
		return fmt.Errorf("type cycle detected at %s", cycleErr.Key)
	}
	var missingErr GraphMissingError[model.QName]
	if errors.As(err, &missingErr) {
		return fmt.Errorf("missing global type %s", missingErr.Key)
	}
	return err
}

func detectGroupCycles(schema *parser.Schema) error {
	starts := make([]model.QName, 0, len(schema.Groups))
	if err := parser.ForEachGlobalDecl(&schema.SchemaGraph, parser.GlobalDeclHandlers{
		Group: func(name model.QName, group *model.ModelGroup) error {
			if group == nil {
				return fmt.Errorf("missing group %s", name)
			}
			starts = append(starts, name)
			return nil
		},
	}); err != nil {
		return err
	}

	err := DetectGraphCycle(GraphCycleConfig[model.QName]{
		Starts:  starts,
		Missing: GraphCycleMissingPolicyError,
		Exists: func(name model.QName) bool {
			return schema.Groups[name] != nil
		},
		Next: func(name model.QName) ([]model.QName, error) {
			group := schema.Groups[name]
			if group == nil {
				return nil, nil
			}
			refs := collectGroupRefs(group)
			out := make([]model.QName, 0, len(refs))
			for _, ref := range refs {
				out = append(out, ref.RefQName)
			}
			return out, nil
		},
	})
	if err == nil {
		return nil
	}
	var cycleErr GraphCycleError[model.QName]
	if errors.As(err, &cycleErr) {
		return fmt.Errorf("group cycle detected at %s", cycleErr.Key)
	}
	var missingErr GraphMissingError[model.QName]
	if errors.As(err, &missingErr) {
		return fmt.Errorf("group %s ref %s not found", missingErr.From, missingErr.Key)
	}
	return err
}

func collectGroupRefs(group *model.ModelGroup) []*model.GroupRef {
	if group == nil {
		return nil
	}
	var refs []*model.GroupRef
	for _, particle := range group.Particles {
		refs = collectGroupRefsFromParticle(particle, refs)
	}
	return refs
}

func collectGroupRefsFromParticle(particle model.Particle, refs []*model.GroupRef) []*model.GroupRef {
	switch typed := particle.(type) {
	case *model.GroupRef:
		return append(refs, typed)
	case *model.ModelGroup:
		for _, child := range typed.Particles {
			refs = collectGroupRefsFromParticle(child, refs)
		}
	case *model.ElementDecl, *model.AnyElement:
		return refs
	}
	return refs
}

func detectAttributeGroupCycles(schema *parser.Schema) error {
	ctx := NewAttributeGroupContext(schema, AttributeGroupWalkOptions{
		Missing: MissingError,
		Cycles:  CyclePolicyError,
	})

	return parser.ForEachGlobalDecl(&schema.SchemaGraph, parser.GlobalDeclHandlers{
		AttributeGroup: func(name model.QName, group *model.AttributeGroup) error {
			if group == nil {
				return fmt.Errorf("missing attributeGroup %s", name)
			}
			if err := ctx.Walk([]model.QName{name}, nil); err != nil {
				var cycleErr AttributeGroupCycleError
				if errors.As(err, &cycleErr) {
					return fmt.Errorf("attributeGroup cycle detected at %s", cycleErr.QName)
				}
				var missingErr AttributeGroupMissingError
				if errors.As(err, &missingErr) {
					return fmt.Errorf("attributeGroup %s ref %s not found", name, missingErr.QName)
				}
				return err
			}
			return nil
		},
	})
}

func detectSubstitutionGroupCycles(schema *parser.Schema) error {
	starts := make([]model.QName, 0, len(schema.ElementDecls))
	if err := parser.ForEachGlobalDecl(&schema.SchemaGraph, parser.GlobalDeclHandlers{
		Element: func(name model.QName, _ *model.ElementDecl) error {
			starts = append(starts, name)
			return nil
		},
	}); err != nil {
		return err
	}

	err := DetectGraphCycle(GraphCycleConfig[model.QName]{
		Starts:  starts,
		Missing: GraphCycleMissingPolicyError,
		Exists: func(name model.QName) bool {
			return schema.ElementDecls[name] != nil
		},
		Next: func(name model.QName) ([]model.QName, error) {
			decl := schema.ElementDecls[name]
			if decl == nil || decl.SubstitutionGroup.IsZero() {
				return nil, nil
			}
			return []model.QName{decl.SubstitutionGroup}, nil
		},
	})
	if err == nil {
		return nil
	}
	var cycleErr GraphCycleError[model.QName]
	if errors.As(err, &cycleErr) {
		return fmt.Errorf("substitution group cycle detected at %s", cycleErr.Key)
	}
	var missingErr GraphMissingError[model.QName]
	if errors.As(err, &missingErr) {
		if missingErr.From.IsZero() {
			return fmt.Errorf("element %s not found", missingErr.Key)
		}
		return fmt.Errorf("element %s substitutionGroup %s not found", missingErr.From, missingErr.Key)
	}
	return err
}

func baseTypeFor(schema *parser.Schema, typ model.Type) (model.Type, model.DerivationMethod, error) {
	switch typed := typ.(type) {
	case *model.SimpleType:
		return baseTypeForSimpleType(schema, typed)
	case *model.ComplexType:
		return baseTypeForComplexType(schema, typed)
	}
	return nil, 0, nil
}

func baseTypeForSimpleType(schema *parser.Schema, st *model.SimpleType) (model.Type, model.DerivationMethod, error) {
	if st == nil {
		return nil, 0, nil
	}
	if st.List != nil {
		return model.GetBuiltin(model.TypeNameAnySimpleType), model.DerivationList, nil
	}
	if st.Union != nil {
		return model.GetBuiltin(model.TypeNameAnySimpleType), model.DerivationUnion, nil
	}
	if st.Restriction != nil {
		if st.Restriction.SimpleType != nil {
			return st.Restriction.SimpleType, model.DerivationRestriction, nil
		}
		if !st.Restriction.Base.IsZero() {
			base, err := parser.ResolveTypeQName(schema, st.Restriction.Base)
			if err != nil {
				return nil, 0, err
			}
			return base, model.DerivationRestriction, nil
		}
	}
	if st.ResolvedBase != nil {
		return st.ResolvedBase, model.DerivationRestriction, nil
	}
	return model.GetBuiltin(model.TypeNameAnySimpleType), model.DerivationRestriction, nil
}

func baseTypeForComplexType(schema *parser.Schema, ct *model.ComplexType) (model.Type, model.DerivationMethod, error) {
	if ct == nil {
		return nil, 0, nil
	}
	baseQName := model.QName{}
	if content := ct.Content(); content != nil {
		baseQName = content.BaseTypeQName()
	}
	if baseQName.IsZero() {
		if ct.QName.Namespace == model.XSDNamespace && ct.QName.Local == "anyType" {
			return nil, 0, nil
		}
		return model.GetBuiltin(model.TypeNameAnyType), model.DerivationRestriction, nil
	}
	method := ct.DerivationMethod
	if method == 0 {
		method = model.DerivationRestriction
	}
	base, err := parser.ResolveTypeQName(schema, baseQName)
	if err != nil {
		return nil, 0, err
	}
	return base, method, nil
}

func requireResolved(schema *parser.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if parser.HasPlaceholders(schema) {
		return fmt.Errorf("schema has unresolved placeholders")
	}
	return nil
}

func validatePreparedSchemaInput(schema *parser.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if len(schema.GlobalDecls) == 0 && hasGlobalDecls(schema) {
		return fmt.Errorf("schema global declaration order missing")
	}
	return nil
}

func hasGlobalDecls(schema *parser.Schema) bool {
	return len(schema.ElementDecls) > 0 || len(schema.TypeDefs) > 0 ||
		len(schema.AttributeDecls) > 0 || len(schema.AttributeGroups) > 0 ||
		len(schema.Groups) > 0 || len(schema.NotationDecls) > 0
}
