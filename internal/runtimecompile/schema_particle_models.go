package runtimecompile

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemaops"
	"github.com/jacoelho/xsd/internal/types"
)

func (b *schemaBuilder) compileParticleModel(particle types.Particle) (runtime.ModelRef, runtime.ContentKind, error) {
	if particle == nil {
		return runtime.ModelRef{Kind: runtime.ModelNone}, runtime.ContentEmpty, nil
	}
	resolved, err := schemaops.ExpandGroupRefs(particle, b.groupRefExpansionOptions())
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	particle = resolved
	if isEmptyChoice(particle) {
		return b.addRejectAllModel(), runtime.ContentElementOnly, nil
	}
	err = b.validateOccursLimit(particle)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	if group, ok := particle.(*types.ModelGroup); ok && group.Kind == types.AllGroup {
		ref, addErr := b.addAllModel(group)
		if addErr != nil {
			return runtime.ModelRef{}, 0, addErr
		}
		return ref, runtime.ContentAll, nil
	}

	glu, err := models.BuildGlushkov(particle)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	glu, err = models.ExpandSubstitution(glu, b.resolveSubstitutionHead, b.substitutionMembers)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	matchers, err := b.buildMatchers(glu)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	compiled, err := models.Compile(glu, matchers, b.limits)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	switch compiled.Kind {
	case runtime.ModelDFA:
		id := uint32(len(b.rt.Models.DFA))
		b.rt.Models.DFA = append(b.rt.Models.DFA, compiled.DFA)
		return runtime.ModelRef{Kind: runtime.ModelDFA, ID: id}, runtime.ContentElementOnly, nil
	case runtime.ModelNFA:
		id := uint32(len(b.rt.Models.NFA))
		b.rt.Models.NFA = append(b.rt.Models.NFA, compiled.NFA)
		return runtime.ModelRef{Kind: runtime.ModelNFA, ID: id}, runtime.ContentElementOnly, nil
	default:
		return runtime.ModelRef{Kind: runtime.ModelNone}, runtime.ContentEmpty, nil
	}
}

func (b *schemaBuilder) groupRefExpansionOptions() schemaops.ExpandGroupRefsOptions {
	return schemaops.ExpandGroupRefsOptions{
		Lookup: func(ref *types.GroupRef) *types.ModelGroup {
			if ref == nil {
				return nil
			}
			if b != nil && b.refs != nil {
				if group := b.refs.GroupRefs[ref]; group != nil {
					return group
				}
			}
			if b == nil || b.schema == nil {
				return nil
			}
			return b.schema.Groups[ref.RefQName]
		},
		MissingError: func(ref types.QName) error {
			return fmt.Errorf("group ref %s not resolved", ref)
		},
		CycleError: func(ref types.QName) error {
			return fmt.Errorf("group ref cycle detected: %s", ref)
		},
		AllGroupMode: schemaops.AllGroupKeep,
		LeafClone:    schemaops.LeafReuse,
	}
}

func isEmptyChoice(particle types.Particle) bool {
	group, ok := particle.(*types.ModelGroup)
	if !ok || group == nil || group.Kind != types.Choice {
		return false
	}
	for _, child := range group.Particles {
		if child == nil {
			continue
		}
		if child.MaxOcc().IsZero() {
			continue
		}
		return false
	}
	return true
}

func (b *schemaBuilder) validateOccursLimit(particle types.Particle) error {
	if particle == nil || b.maxOccurs == 0 {
		return nil
	}
	if err := b.checkOccursValue("minOccurs", particle.MinOcc()); err != nil {
		return err
	}
	if err := b.checkOccursValue("maxOccurs", particle.MaxOcc()); err != nil {
		return err
	}
	if group, ok := particle.(*types.ModelGroup); ok {
		for _, child := range group.Particles {
			if err := b.validateOccursLimit(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *schemaBuilder) checkOccursValue(attr string, occ types.Occurs) error {
	if b == nil || b.maxOccurs == 0 {
		return nil
	}
	if occ.IsUnbounded() {
		return nil
	}
	if occ.IsOverflow() {
		return fmt.Errorf("%w: %s value %s exceeds uint32", types.ErrOccursOverflow, attr, occ.String())
	}
	if occ.GreaterThanInt(int(b.maxOccurs)) {
		return fmt.Errorf("%w: %s value %s exceeds limit %d", types.ErrOccursTooLarge, attr, occ.String(), b.maxOccurs)
	}
	return nil
}

func (b *schemaBuilder) addAllModel(group *types.ModelGroup) (runtime.ModelRef, error) {
	if group == nil {
		return runtime.ModelRef{Kind: runtime.ModelNone}, nil
	}
	minOccurs, ok := group.MinOccurs.Int()
	if !ok {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group minOccurs too large")
	}
	if group.MaxOccurs.IsUnbounded() {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group maxOccurs unbounded")
	}
	if maxOccurs, ok := group.MaxOccurs.Int(); !ok || maxOccurs > 1 {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group maxOccurs must be <= 1")
	}

	model := runtime.AllModel{
		MinOccurs: uint32(minOccurs),
		Mixed:     false,
	}
	for _, particle := range group.Particles {
		elem, ok := particle.(*types.ElementDecl)
		if !ok || elem == nil {
			return runtime.ModelRef{}, fmt.Errorf("runtime build: all group member must be element")
		}
		elemID, ok := b.runtimeElemID(elem)
		if !ok {
			return runtime.ModelRef{}, fmt.Errorf("runtime build: all group element %s missing ID", elem.Name)
		}
		minOccurs := elem.MinOcc()
		optional := minOccurs.IsZero()
		member := runtime.AllMember{
			Elem:     elemID,
			Optional: optional,
		}
		if elem.IsReference {
			member.AllowsSubst = true
			head := elem
			if resolved := b.resolveSubstitutionHead(elem); resolved != nil {
				head = resolved
			}
			list, err := models.ExpandSubstitutionMembers(head, b.substitutionMembers)
			if err != nil {
				return runtime.ModelRef{}, err
			}
			if len(list) > 0 {
				member.SubstOff = uint32(len(b.rt.Models.AllSubst))
				for _, decl := range list {
					if decl == nil {
						continue
					}
					memberID, ok := b.runtimeElemID(decl)
					if !ok {
						return runtime.ModelRef{}, fmt.Errorf("runtime build: all group substitution element %s missing ID", decl.Name)
					}
					b.rt.Models.AllSubst = append(b.rt.Models.AllSubst, memberID)
				}
				member.SubstLen = uint32(len(b.rt.Models.AllSubst)) - member.SubstOff
			}
		}
		model.Members = append(model.Members, member)
	}

	id := uint32(len(b.rt.Models.All))
	b.rt.Models.All = append(b.rt.Models.All, model)
	return runtime.ModelRef{Kind: runtime.ModelAll, ID: id}, nil
}

func (b *schemaBuilder) addRejectAllModel() runtime.ModelRef {
	id := uint32(len(b.rt.Models.NFA))
	b.rt.Models.NFA = append(b.rt.Models.NFA, runtime.NFAModel{
		Nullable:  false,
		Start:     runtime.BitsetRef{},
		Accept:    runtime.BitsetRef{},
		FollowOff: 0,
		FollowLen: 0,
	})
	return runtime.ModelRef{Kind: runtime.ModelNFA, ID: id}
}

func (b *schemaBuilder) buildMatchers(glu *models.Glushkov) ([]runtime.PosMatcher, error) {
	if glu == nil {
		return nil, fmt.Errorf("runtime build: glushkov model missing")
	}
	matchers := make([]runtime.PosMatcher, len(glu.Positions))
	for i, pos := range glu.Positions {
		switch pos.Kind {
		case models.PositionElement:
			if pos.Element == nil {
				return nil, fmt.Errorf("runtime build: position %d missing element", i)
			}
			elemID, ok := b.runtimeElemID(pos.Element)
			if !ok {
				return nil, fmt.Errorf("runtime build: element %s missing ID", pos.Element.Name)
			}
			sym := b.internQName(pos.Element.Name)
			matchers[i] = runtime.PosMatcher{
				Kind: runtime.PosExact,
				Sym:  sym,
				Elem: elemID,
			}
		case models.PositionWildcard:
			if pos.Wildcard == nil {
				return nil, fmt.Errorf("runtime build: position %d missing wildcard", i)
			}
			rule := b.addWildcardAnyElement(pos.Wildcard)
			matchers[i] = runtime.PosMatcher{
				Kind: runtime.PosWildcard,
				Rule: rule,
			}
		default:
			return nil, fmt.Errorf("runtime build: unknown position kind %d", pos.Kind)
		}
	}
	return matchers, nil
}

func (b *schemaBuilder) addWildcardAnyElement(anyElem *types.AnyElement) runtime.WildcardID {
	if anyElem == nil {
		return 0
	}
	if b.anyElementRules == nil {
		b.anyElementRules = make(map[*types.AnyElement]runtime.WildcardID)
	}
	if id, ok := b.anyElementRules[anyElem]; ok {
		return id
	}
	id := b.addWildcard(anyElem.Namespace, anyElem.NamespaceList, anyElem.TargetNamespace, anyElem.ProcessContents)
	b.anyElementRules[anyElem] = id
	return id
}

func (b *schemaBuilder) addWildcardAnyAttribute(anyAttr *types.AnyAttribute) runtime.WildcardID {
	if anyAttr == nil {
		return 0
	}
	return b.addWildcard(anyAttr.Namespace, anyAttr.NamespaceList, anyAttr.TargetNamespace, anyAttr.ProcessContents)
}

func (b *schemaBuilder) addWildcard(constraint types.NamespaceConstraint, list []types.NamespaceURI, target types.NamespaceURI, pc types.ProcessContents) runtime.WildcardID {
	rule := runtime.WildcardRule{
		PC:       toRuntimeProcessContents(pc),
		TargetNS: b.internNamespace(target),
	}

	off := len(b.wildcardNS)
	switch constraint {
	case types.NSCAny:
		rule.NS.Kind = runtime.NSAny
	case types.NSCOther:
		rule.NS.Kind = runtime.NSOther
		rule.NS.HasTarget = true
	case types.NSCTargetNamespace:
		rule.NS.Kind = runtime.NSEnumeration
		rule.NS.HasTarget = true
	case types.NSCLocal:
		rule.NS.Kind = runtime.NSEnumeration
		rule.NS.HasLocal = true
	case types.NSCNotAbsent:
		rule.NS.Kind = runtime.NSNotAbsent
	case types.NSCList:
		rule.NS.Kind = runtime.NSEnumeration
		for _, ns := range list {
			if ns == types.NamespaceTargetPlaceholder {
				rule.NS.HasTarget = true
				continue
			}
			if ns.IsEmpty() {
				rule.NS.HasLocal = true
				continue
			}
			b.wildcardNS = append(b.wildcardNS, b.internNamespace(ns))
		}
	default:
		rule.NS.Kind = runtime.NSAny
	}

	if rule.NS.Kind == runtime.NSEnumeration {
		ln := len(b.wildcardNS) - off
		if ln > 0 {
			rule.NS.Off = uint32(off)
			rule.NS.Len = uint32(ln)
		}
	}

	b.wildcards = append(b.wildcards, rule)
	return runtime.WildcardID(len(b.wildcards) - 1)
}

func (b *schemaBuilder) substitutionMembers(head *types.ElementDecl) []*types.ElementDecl {
	if head == nil {
		return nil
	}
	queue := []types.QName{head.Name}
	seen := make(map[types.QName]bool)
	seen[head.Name] = true
	out := make([]*types.ElementDecl, 0)

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, memberName := range b.schema.SubstitutionGroups[name] {
			if seen[memberName] {
				continue
			}
			seen[memberName] = true
			decl := b.schema.ElementDecls[memberName]
			if decl == nil {
				continue
			}
			out = append(out, decl)
			queue = append(queue, memberName)
		}
	}
	return out
}

func (b *schemaBuilder) resolveSubstitutionHead(decl *types.ElementDecl) *types.ElementDecl {
	if decl == nil || !decl.IsReference || b == nil || b.schema == nil {
		return decl
	}
	if head := b.schema.ElementDecls[decl.Name]; head != nil {
		return head
	}
	return decl
}
