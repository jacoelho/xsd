package schemacheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// ValidateUPA validates Unique Particle Attribution for a content model.
// UPA requires that no element can be matched by more than one particle.
func ValidateUPA(schema *parser.Schema, content types.Content, targetNS types.NamespaceURI) error {
	var particle types.Particle
	var baseParticle types.Particle

	switch c := content.(type) {
	case *types.ElementContent:
		particle = c.Particle
	case *types.ComplexContent:
		if c.Extension != nil {
			particle = c.Extension.Particle
			if !c.Extension.Base.IsZero() {
				if baseCT, ok := lookupComplexType(schema, c.Extension.Base); ok {
					if baseEC, ok := baseCT.Content().(*types.ElementContent); ok {
						baseParticle = baseEC.Particle
					}
				}
			}
		}
		if c.Restriction != nil {
			particle = c.Restriction.Particle
		}
	}

	if particle == nil && baseParticle == nil {
		return nil
	}

	if particle != nil {
		expanded, err := expandGroupRefs(schema, particle, make(map[types.QName]bool))
		if err != nil {
			return err
		}
		particle = expanded
	}
	if baseParticle != nil {
		expanded, err := expandGroupRefs(schema, baseParticle, make(map[types.QName]bool))
		if err != nil {
			return err
		}
		baseParticle = expanded
	}

	if baseParticle != nil && particle != nil {
		particle = &types.ModelGroup{
			Kind:      types.Sequence,
			MinOccurs: types.OccursFromInt(1),
			MaxOccurs: types.OccursFromInt(1),
			Particles: []types.Particle{baseParticle, particle},
		}
	} else if particle == nil {
		particle = baseParticle
	}

	if particle == nil {
		return nil
	}

	return validateUPADeterminism(schema, particle, targetNS)
}

func validateUPADeterminism(schema *parser.Schema, particle types.Particle, targetNS types.NamespaceURI) error {
	adapter := buildUPAParticleAdapter(particle)
	if adapter == nil {
		return nil
	}

	elementFormDefault := types.FormUnqualified
	if schema != nil && schema.ElementFormDefault == parser.Qualified {
		elementFormDefault = types.FormQualified
	}

	builder := grammar.NewBuilder([]*grammar.ParticleAdapter{adapter}, string(targetNS), elementFormDefault)
	automaton, err := builder.Build()
	if err != nil {
		return fmt.Errorf("content model automaton: %w", err)
	}

	overlap := func(left, right *grammar.Symbol) bool {
		return symbolsOverlap(schema, left, right)
	}
	if err := automaton.CheckDeterminism(overlap); err != nil {
		return err
	}
	return nil
}

func buildUPAParticleAdapter(particle types.Particle) *grammar.ParticleAdapter {
	switch p := particle.(type) {
	case *types.ElementDecl:
		if p.MaxOcc().IsZero() {
			return nil
		}
		return &grammar.ParticleAdapter{
			Original:          p,
			Kind:              grammar.ParticleElement,
			MinOccurs:         p.MinOccurs,
			MaxOccurs:         p.MaxOccurs,
			AllowSubstitution: p.IsReference,
		}
	case *types.AnyElement:
		if p.MaxOcc().IsZero() {
			return nil
		}
		return &grammar.ParticleAdapter{
			Original:  p,
			Wildcard:  p,
			Kind:      grammar.ParticleWildcard,
			MinOccurs: p.MinOccurs,
			MaxOccurs: p.MaxOccurs,
		}
	case *types.ModelGroup:
		if p.MaxOcc().IsZero() {
			return nil
		}
		children := make([]*grammar.ParticleAdapter, 0, len(p.Particles))
		for _, child := range p.Particles {
			if adapter := buildUPAParticleAdapter(child); adapter != nil {
				children = append(children, adapter)
			}
		}
		if len(children) == 0 {
			return nil
		}
		groupKind := p.Kind
		if groupKind == types.AllGroup {
			// For UPA determinism, all-groups can be treated as choice groups.
			groupKind = types.Choice
		}
		return &grammar.ParticleAdapter{
			Kind:      grammar.ParticleGroup,
			GroupKind: groupKind,
			MinOccurs: p.MinOccurs,
			MaxOccurs: p.MaxOccurs,
			Children:  children,
		}
	default:
		return nil
	}
}

func symbolsOverlap(schema *parser.Schema, left, right *grammar.Symbol) bool {
	if left == nil || right == nil {
		return false
	}
	if left.Kind == grammar.KindElement && right.Kind == grammar.KindElement {
		return elementSymbolsOverlap(schema, left, right)
	}
	if left.Kind == grammar.KindElement {
		return elementWildcardOverlap(schema, left, right)
	}
	if right.Kind == grammar.KindElement {
		return elementWildcardOverlap(schema, right, left)
	}
	return wildcardSymbolsOverlap(left, right)
}

func elementSymbolsOverlap(schema *parser.Schema, left, right *grammar.Symbol) bool {
	if left.QName == right.QName {
		return true
	}
	if schema == nil {
		return false
	}
	if left.AllowSubstitution && isSubstitutableElement(schema, left.QName, right.QName) {
		return true
	}
	if right.AllowSubstitution && isSubstitutableElement(schema, right.QName, left.QName) {
		return true
	}
	return false
}

func elementWildcardOverlap(schema *parser.Schema, elem, wildcard *grammar.Symbol) bool {
	if wildcardMatchesQName(wildcard, elem.QName) {
		return true
	}
	if schema == nil || !elem.AllowSubstitution {
		return false
	}
	for _, member := range substitutionMembers(schema, elem.QName) {
		if !isSubstitutableElement(schema, elem.QName, member) {
			continue
		}
		if wildcardMatchesQName(wildcard, member) {
			return true
		}
	}
	return false
}

func wildcardSymbolsOverlap(left, right *grammar.Symbol) bool {
	w1 := symbolToAnyElement(left)
	w2 := symbolToAnyElement(right)
	if w1 == nil || w2 == nil {
		return false
	}
	return types.IntersectAnyElement(w1, w2) != nil
}

func wildcardMatchesQName(symbol *grammar.Symbol, qname types.QName) bool {
	if symbol == nil {
		return false
	}
	switch symbol.Kind {
	case grammar.KindAny:
		return true
	case grammar.KindAnyNS:
		return qname.Namespace.String() == symbol.NS
	case grammar.KindAnyOther:
		if symbol.NS == "" {
			return !qname.Namespace.IsEmpty()
		}
		return qname.Namespace.String() != symbol.NS
	case grammar.KindAnyNSList:
		if slices.Contains(symbol.NSList, qname.Namespace) {
			return true
		}
	}
	return false
}

func symbolToAnyElement(symbol *grammar.Symbol) *types.AnyElement {
	if symbol == nil {
		return nil
	}
	switch symbol.Kind {
	case grammar.KindAny:
		return &types.AnyElement{Namespace: types.NSCAny}
	case grammar.KindAnyNS:
		if symbol.NS == "" {
			return &types.AnyElement{Namespace: types.NSCLocal}
		}
		return &types.AnyElement{Namespace: types.NSCTargetNamespace, TargetNamespace: types.NamespaceURI(symbol.NS)}
	case grammar.KindAnyOther:
		if symbol.NS == "" {
			return &types.AnyElement{Namespace: types.NSCNotAbsent}
		}
		return &types.AnyElement{Namespace: types.NSCOther, TargetNamespace: types.NamespaceURI(symbol.NS)}
	case grammar.KindAnyNSList:
		return &types.AnyElement{Namespace: types.NSCList, NamespaceList: symbol.NSList}
	default:
		return nil
	}
}

func substitutionMembers(schema *parser.Schema, head types.QName) []types.QName {
	if schema == nil {
		return nil
	}
	visited := make(map[types.QName]bool)
	queue := []types.QName{head}
	visited[head] = true
	var out []types.QName
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, member := range schema.SubstitutionGroups[current] {
			if visited[member] {
				continue
			}
			visited[member] = true
			out = append(out, member)
			queue = append(queue, member)
		}
	}
	return out
}

func expandGroupRefs(schema *parser.Schema, particle types.Particle, stack map[types.QName]bool) (types.Particle, error) {
	switch p := particle.(type) {
	case *types.GroupRef:
		if stack[p.RefQName] {
			return nil, fmt.Errorf("circular group reference detected for %s", p.RefQName)
		}
		groupDef, exists := schema.Groups[p.RefQName]
		if !exists {
			return nil, fmt.Errorf("group '%s' not found", p.RefQName)
		}
		stack[p.RefQName] = true
		defer delete(stack, p.RefQName)

		groupCopy := &types.ModelGroup{
			Kind:      groupDef.Kind,
			MinOccurs: p.MinOccurs,
			MaxOccurs: p.MaxOccurs,
			Particles: make([]types.Particle, 0, len(groupDef.Particles)),
		}
		for _, child := range groupDef.Particles {
			expanded, err := expandGroupRefs(schema, child, stack)
			if err != nil {
				return nil, err
			}
			groupCopy.Particles = append(groupCopy.Particles, expanded)
		}
		return groupCopy, nil
	case *types.ModelGroup:
		groupCopy := &types.ModelGroup{
			Kind:      p.Kind,
			MinOccurs: p.MinOccurs,
			MaxOccurs: p.MaxOccurs,
			Particles: make([]types.Particle, 0, len(p.Particles)),
		}
		for _, child := range p.Particles {
			expanded, err := expandGroupRefs(schema, child, stack)
			if err != nil {
				return nil, err
			}
			groupCopy.Particles = append(groupCopy.Particles, expanded)
		}
		return groupCopy, nil
	default:
		return particle, nil
	}
}
