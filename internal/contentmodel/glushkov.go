package contentmodel

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

const maxInt = int(^uint(0) >> 1)

// PositionKind identifies the kind of Glushkov position.
type PositionKind uint8

const (
	PositionElement PositionKind = iota
	PositionWildcard
)

// Position represents a single element or wildcard occurrence in the model.
type Position struct {
	Element     *model.ElementDecl
	Wildcard    *model.AnyElement
	Kind        PositionKind
	AllowsSubst bool
}

// Glushkov contains the compiled position sets and followpos relations.
type Glushkov struct {
	firstRaw  *bitset
	lastRaw   *bitset
	Positions []Position
	Follow    []runtime.BitsetRef
	Bitsets   runtime.BitsetBlob
	followRaw []*bitset
	First     runtime.BitsetRef
	Last      runtime.BitsetRef
	Nullable  bool
}

// BuildGlushkov compiles a particle into Glushkov positions and followpos sets.
func BuildGlushkov(particle model.Particle) (*Glushkov, error) {
	b := &builder{}
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

type builder struct {
	positions []Position
	follow    []*bitset
	size      int
}

func (b *builder) countPositions(particle model.Particle) (int, error) {
	if particle == nil {
		return 0, nil
	}
	minOccurs, maxOccurs, unbounded, err := occursBounds(particle.MinOcc(), particle.MaxOcc())
	if err != nil {
		return 0, err
	}
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

func (b *builder) countSingle(particle model.Particle) (int, error) {
	switch typed := particle.(type) {
	case *model.ElementDecl:
		return 1, nil
	case *model.AnyElement:
		return 1, nil
	case *model.ModelGroup:
		if typed.Kind == model.AllGroup {
			return 0, fmt.Errorf("all group not supported in Glushkov builder")
		}
		total := 0
		for _, child := range typed.Particles {
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
	case *model.GroupRef:
		return 0, fmt.Errorf("group ref %s not resolved", typed.RefQName)
	default:
		return 0, fmt.Errorf("unsupported particle %T", particle)
	}
}

func (b *builder) buildParticle(particle model.Particle, nextPos *int) (node, error) {
	if particle == nil {
		return nil, nil
	}
	minOccurs, maxOccurs, unbounded, err := occursBounds(particle.MinOcc(), particle.MaxOcc())
	if err != nil {
		return nil, err
	}
	if !unbounded && maxOccurs == 0 {
		return nil, nil
	}
	if unbounded {
		return b.buildUnbounded(particle, minOccurs, nextPos)
	}
	return b.buildBounded(particle, minOccurs, maxOccurs, nextPos)
}

func (b *builder) buildUnbounded(particle model.Particle, minOccurs int, nextPos *int) (node, error) {
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

func (b *builder) buildBounded(particle model.Particle, minOccurs, maxOccurs int, nextPos *int) (node, error) {
	if maxOccurs < minOccurs {
		return nil, fmt.Errorf("particle maxOccurs %d less than minOccurs %d", maxOccurs, minOccurs)
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

func (b *builder) buildSingle(particle model.Particle, nextPos *int) (node, error) {
	switch typed := particle.(type) {
	case *model.ElementDecl:
		pos := *nextPos
		*nextPos = pos + 1
		b.positions = append(b.positions, Position{
			Kind:        PositionElement,
			Element:     typed,
			AllowsSubst: typed.IsReference,
		})
		return newLeaf(pos, b.size), nil
	case *model.AnyElement:
		pos := *nextPos
		*nextPos = pos + 1
		b.positions = append(b.positions, Position{
			Kind:     PositionWildcard,
			Wildcard: typed,
		})
		return newLeaf(pos, b.size), nil
	case *model.ModelGroup:
		if typed.Kind == model.AllGroup {
			return nil, fmt.Errorf("all group not supported in Glushkov builder")
		}
		return b.buildGroup(typed, nextPos)
	case *model.GroupRef:
		return nil, fmt.Errorf("group ref %s not resolved", typed.RefQName)
	default:
		return nil, fmt.Errorf("unsupported particle %T", particle)
	}
}

func (b *builder) buildGroup(group *model.ModelGroup, nextPos *int) (node, error) {
	switch group.Kind {
	case model.Sequence:
		return b.buildSequence(group.Particles, nextPos)
	case model.Choice:
		return b.buildChoice(group.Particles, nextPos)
	default:
		return nil, fmt.Errorf("unsupported group kind %v", group.Kind)
	}
}

func (b *builder) buildSequence(particles []model.Particle, nextPos *int) (node, error) {
	if len(particles) == 0 {
		return nil, nil
	}
	parts := make([]node, 0, len(particles))
	for _, particle := range particles {
		child, err := b.buildParticle(particle, nextPos)
		if err != nil {
			return nil, err
		}
		if child != nil {
			parts = append(parts, child)
		}
	}
	return b.sequence(parts), nil
}

func (b *builder) buildChoice(particles []model.Particle, nextPos *int) (node, error) {
	if len(particles) == 0 {
		return nil, nil
	}
	parts := make([]node, 0, len(particles))
	for _, particle := range particles {
		child, err := b.buildParticle(particle, nextPos)
		if err != nil {
			return nil, err
		}
		if child != nil {
			parts = append(parts, child)
		}
	}
	return b.choice(parts), nil
}

func (b *builder) sequence(nodes []node) node {
	if len(nodes) == 0 {
		return nil
	}
	result := nodes[0]
	for i := 1; i < len(nodes); i++ {
		result = newSeq(result, nodes[i], b.size)
	}
	return result
}

func (b *builder) choice(nodes []node) node {
	if len(nodes) == 0 {
		return nil
	}
	result := nodes[0]
	for i := 1; i < len(nodes); i++ {
		result = newAlt(result, nodes[i], b.size)
	}
	return result
}

func occursBounds(minOccurs, maxOccurs model.Occurs) (int, int, bool, error) {
	if minOccurs.IsUnbounded() {
		return 0, 0, false, fmt.Errorf("minOccurs cannot be unbounded")
	}
	minCount, ok := minOccurs.Int()
	if !ok {
		return 0, 0, false, fmt.Errorf("minOccurs too large")
	}
	if maxOccurs.IsUnbounded() {
		return minCount, 0, true, nil
	}
	maxCount, ok := maxOccurs.Int()
	if !ok {
		return 0, 0, false, fmt.Errorf("maxOccurs too large")
	}
	if maxCount < minCount {
		return 0, 0, false, fmt.Errorf("maxOccurs %d less than minOccurs %d", maxCount, minCount)
	}
	return minCount, maxCount, false, nil
}

func addCount(a, b int) (int, error) {
	if a > maxInt-b {
		return 0, fmt.Errorf("particle size overflow")
	}
	return a + b, nil
}

func mulCount(a, b int) (int, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	if a > maxInt/b {
		return 0, fmt.Errorf("particle size overflow")
	}
	return a * b, nil
}

func packBitset(blob *runtime.BitsetBlob, set *bitset) runtime.BitsetRef {
	if set == nil || len(set.words) == 0 {
		return runtime.BitsetRef{}
	}
	off := uint32(len(blob.Words))
	blob.Words = append(blob.Words, set.words...)
	return runtime.BitsetRef{Off: off, Len: uint32(len(set.words))}
}
