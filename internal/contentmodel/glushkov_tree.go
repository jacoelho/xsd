package contentmodel

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

type TreeParticleKind uint8

const (
	TreeElement TreeParticleKind = iota
	TreeWildcard
	TreeGroup
)

type TreeGroupKind uint8

const (
	TreeSequence TreeGroupKind = iota
	TreeChoice
	TreeAll
)

type TreeOccurs struct {
	Value     uint32
	Unbounded bool
}

type TreeParticle struct {
	Children           []TreeParticle
	ElementID          uint32
	WildcardID         uint32
	Min                TreeOccurs
	Max                TreeOccurs
	Kind               TreeParticleKind
	Group              TreeGroupKind
	AllowsSubstitution bool
	RuntimeRule        bool
}

func BuildGlushkovTree(particle TreeParticle) (*Glushkov, error) {
	b := &treeBuilder{}
	size, err := b.countPositions(particle)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return &Glushkov{Nullable: true}, nil
	}
	b.size = size
	b.positions = make([]Position, 0, size)
	b.follow = make([]*bitset, size)
	for i := range b.follow {
		b.follow[i] = newBitset(size)
	}

	nextPos := 0
	root, err := b.buildParticle(particle, &nextPos)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return &Glushkov{Nullable: true}, nil
	}
	if nextPos != size {
		return nil, fmt.Errorf("glushkov position count mismatch: got %d, want %d", nextPos, size)
	}

	b.computeFollowPos(root)
	first := root.firstPos()
	last := root.lastPos()

	var blob runtime.BitsetBlob
	firstRef := packBitset(&blob, first)
	lastRef := packBitset(&blob, last)
	followRefs := make([]runtime.BitsetRef, len(b.follow))
	for i, set := range b.follow {
		followRefs[i] = packBitset(&blob, set)
	}

	return &Glushkov{
		Positions: b.positions,
		First:     firstRef,
		Last:      lastRef,
		Follow:    followRefs,
		Nullable:  root.nullable(),
		Bitsets:   blob,
		firstRaw:  first,
		lastRaw:   last,
		followRaw: b.follow,
	}, nil
}

type treeBuilder struct {
	positions []Position
	follow    []*bitset
	size      int
}

func (b *treeBuilder) computeFollowPos(n node) {
	switch v := n.(type) {
	case *seqNode:
		b.computeFollowPos(v.left)
		b.computeFollowPos(v.right)
		v.left.lastPos().forEach(func(pos int) {
			b.follow[pos].or(v.right.firstPos())
		})
	case *altNode:
		b.computeFollowPos(v.left)
		b.computeFollowPos(v.right)
	case *starNode:
		b.computeFollowPos(v.child)
		v.child.lastPos().forEach(func(pos int) {
			b.follow[pos].or(v.child.firstPos())
		})
	case *plusNode:
		b.computeFollowPos(v.child)
		v.child.lastPos().forEach(func(pos int) {
			b.follow[pos].or(v.child.firstPos())
		})
	case *optNode:
		b.computeFollowPos(v.child)
	case *leafNode:
		return
	}
}

func (b *treeBuilder) countPositions(particle TreeParticle) (int, error) {
	minOccurs, maxOccurs, unbounded := treeOccursBounds(particle.Min, particle.Max)
	if !unbounded && maxOccurs == 0 {
		return 0, nil
	}
	base, err := b.countSingle(particle)
	if err != nil {
		return 0, err
	}
	if base == 0 {
		return 0, nil
	}
	if unbounded {
		occ := minOccurs
		if occ == 0 {
			occ = 1
		}
		return mulCount(base, occ)
	}
	return mulCount(base, maxOccurs)
}

func (b *treeBuilder) countSingle(particle TreeParticle) (int, error) {
	switch particle.Kind {
	case TreeElement, TreeWildcard:
		return 1, nil
	case TreeGroup:
		if particle.Group == TreeAll {
			return 0, fmt.Errorf("all group not supported in Glushkov builder")
		}
		total := 0
		for _, child := range particle.Children {
			count, err := b.countPositions(child)
			if err != nil {
				return 0, err
			}
			total, err = addCount(total, count)
			if err != nil {
				return 0, err
			}
		}
		return total, nil
	default:
		return 0, fmt.Errorf("unsupported tree particle kind %d", particle.Kind)
	}
}

func (b *treeBuilder) buildParticle(particle TreeParticle, nextPos *int) (node, error) {
	minOccurs, maxOccurs, unbounded := treeOccursBounds(particle.Min, particle.Max)
	if !unbounded && maxOccurs == 0 {
		return nil, nil
	}
	if unbounded {
		return b.buildUnbounded(particle, minOccurs, nextPos)
	}
	return b.buildBounded(particle, minOccurs, maxOccurs, nextPos)
}

func (b *treeBuilder) buildUnbounded(particle TreeParticle, minOccurs int, nextPos *int) (node, error) {
	if minOccurs == 0 {
		child, err := b.buildSingle(particle, nextPos)
		if err != nil {
			return nil, err
		}
		if child == nil {
			return nil, nil
		}
		return newStar(child, b.size), nil
	}
	if minOccurs == 1 {
		child, err := b.buildSingle(particle, nextPos)
		if err != nil {
			return nil, err
		}
		if child == nil {
			return nil, nil
		}
		return newPlus(child, b.size), nil
	}

	nodes := make([]node, 0, minOccurs)
	for i := 0; i < minOccurs-1; i++ {
		child, err := b.buildSingle(particle, nextPos)
		if err != nil {
			return nil, err
		}
		if child != nil {
			nodes = append(nodes, child)
		}
	}
	child, err := b.buildSingle(particle, nextPos)
	if err != nil {
		return nil, err
	}
	if child != nil {
		nodes = append(nodes, newPlus(child, b.size))
	}
	return b.sequence(nodes), nil
}

func (b *treeBuilder) buildBounded(particle TreeParticle, minOccurs, maxOccurs int, nextPos *int) (node, error) {
	if maxOccurs < minOccurs {
		return nil, fmt.Errorf("maxOccurs %d less than minOccurs %d", maxOccurs, minOccurs)
	}
	if maxOccurs == 0 {
		return nil, nil
	}
	nodes := make([]node, 0, maxOccurs)
	for range minOccurs {
		child, err := b.buildSingle(particle, nextPos)
		if err != nil {
			return nil, err
		}
		if child != nil {
			nodes = append(nodes, child)
		}
	}
	for i := minOccurs; i < maxOccurs; i++ {
		child, err := b.buildSingle(particle, nextPos)
		if err != nil {
			return nil, err
		}
		if child != nil {
			nodes = append(nodes, newOpt(child, b.size))
		}
	}
	return b.sequence(nodes), nil
}

func (b *treeBuilder) buildSingle(particle TreeParticle, nextPos *int) (node, error) {
	switch particle.Kind {
	case TreeElement:
		pos := *nextPos
		*nextPos = pos + 1
		b.positions = append(b.positions, Position{
			Kind:        PositionElement,
			ElementID:   particle.ElementID,
			AllowsSubst: particle.AllowsSubstitution,
		})
		return newLeaf(pos, b.size), nil
	case TreeWildcard:
		pos := *nextPos
		*nextPos = pos + 1
		b.positions = append(b.positions, Position{
			Kind:        PositionWildcard,
			WildcardID:  particle.WildcardID,
			RuntimeRule: particle.RuntimeRule,
		})
		return newLeaf(pos, b.size), nil
	case TreeGroup:
		return b.buildGroup(particle, nextPos)
	default:
		return nil, fmt.Errorf("unsupported tree particle kind %d", particle.Kind)
	}
}

func (b *treeBuilder) buildGroup(particle TreeParticle, nextPos *int) (node, error) {
	switch particle.Group {
	case TreeSequence:
		return b.buildSequence(particle.Children, nextPos)
	case TreeChoice:
		return b.buildChoice(particle.Children, nextPos)
	default:
		return nil, fmt.Errorf("unsupported tree group kind %d", particle.Group)
	}
}

func (b *treeBuilder) buildSequence(particles []TreeParticle, nextPos *int) (node, error) {
	nodes := make([]node, 0, len(particles))
	for _, particle := range particles {
		child, err := b.buildParticle(particle, nextPos)
		if err != nil {
			return nil, err
		}
		if child != nil {
			nodes = append(nodes, child)
		}
	}
	return b.sequence(nodes), nil
}

func (b *treeBuilder) buildChoice(particles []TreeParticle, nextPos *int) (node, error) {
	nodes := make([]node, 0, len(particles))
	for _, particle := range particles {
		child, err := b.buildParticle(particle, nextPos)
		if err != nil {
			return nil, err
		}
		if child != nil {
			nodes = append(nodes, child)
		}
	}
	return b.choice(nodes), nil
}

func (b *treeBuilder) sequence(nodes []node) node {
	if len(nodes) == 0 {
		return nil
	}
	result := nodes[0]
	for i := 1; i < len(nodes); i++ {
		result = newSeq(result, nodes[i], b.size)
	}
	return result
}

func (b *treeBuilder) choice(nodes []node) node {
	if len(nodes) == 0 {
		return nil
	}
	result := nodes[0]
	for i := 1; i < len(nodes); i++ {
		result = newAlt(result, nodes[i], b.size)
	}
	return result
}

func treeOccursBounds(minOccurs, maxOccurs TreeOccurs) (int, int, bool) {
	minCount := int(minOccurs.Value)
	if maxOccurs.Unbounded {
		return minCount, 0, true
	}
	return minCount, int(maxOccurs.Value), false
}
