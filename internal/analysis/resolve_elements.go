package analysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

func (r *referenceResolver) resolveGlobalElement(decl *types.ElementDecl) error {
	if decl != nil && !decl.SubstitutionGroup.IsZero() {
		if _, ok := r.schema.ElementDecls[decl.SubstitutionGroup]; !ok {
			return fmt.Errorf("element %s substitutionGroup %s not found", decl.Name, decl.SubstitutionGroup)
		}
	}
	return r.resolveElement(decl)
}

func (r *referenceResolver) resolveElement(decl *types.ElementDecl) error {
	if decl == nil {
		return nil
	}
	return r.elementState.Resolve(decl, nil, func() error {
		if decl.IsReference {
			if err := r.resolveElementReference(decl); err != nil {
				return err
			}
			return nil
		}
		if decl.Type == nil {
			return nil
		}
		if st, ok := decl.Type.(*types.SimpleType); ok && types.IsPlaceholderSimpleType(st) {
			if err := r.resolveTypeQName(st.QName); err != nil {
				return fmt.Errorf("element %s: %w", decl.Name, err)
			}
			return nil
		}
		if err := r.resolveType(decl.Type); err != nil {
			return fmt.Errorf("element %s: %w", decl.Name, err)
		}
		return nil
	})
}

func (r *referenceResolver) resolveElementReference(decl *types.ElementDecl) error {
	target := r.schema.ElementDecls[decl.Name]
	if target == nil {
		return fmt.Errorf("element ref %s not found", decl.Name)
	}
	id, ok := r.registry.Elements[decl.Name]
	if !ok {
		return fmt.Errorf("element ref %s missing ID", decl.Name)
	}
	if existing, exists := r.refs.ElementRefs[decl.Name]; exists && existing != id {
		return fmt.Errorf("element ref %s resolved inconsistently (%d != %d)", decl.Name, existing, id)
	}
	r.refs.ElementRefs[decl.Name] = id
	return nil
}

func (r *referenceResolver) resolveModelGroup(group *types.ModelGroup) error {
	if group == nil {
		return nil
	}
	return r.modelGroupState.Resolve(group, nil, func() error {
		for _, particle := range group.Particles {
			if err := r.resolveParticle(particle); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *referenceResolver) resolveParticle(particle types.Particle) error {
	switch typed := particle.(type) {
	case *types.ElementDecl:
		return r.resolveElement(typed)
	case *types.ModelGroup:
		return r.resolveModelGroup(typed)
	case *types.GroupRef:
		return r.resolveGroupRef(typed)
	case *types.AnyElement:
		return nil
	}
	return nil
}

func (r *referenceResolver) resolveGroupRef(ref *types.GroupRef) error {
	group := r.schema.Groups[ref.RefQName]
	if group == nil {
		return fmt.Errorf("group ref %s not found", ref.RefQName)
	}
	targetName := ref.RefQName
	if existing, exists := r.refs.GroupRefs[ref.RefQName]; exists && existing != targetName {
		return fmt.Errorf("group ref %s resolved inconsistently (%s != %s)", ref.RefQName, existing, targetName)
	}
	r.refs.GroupRefs[ref.RefQName] = targetName
	return nil
}
