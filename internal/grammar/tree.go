package grammar

import "github.com/jacoelho/xsd/internal/types"

// node is the interface for syntax tree nodes used during DFA construction.
// Implementations compute nullable, firstPos, and lastPos lazily.
type node interface {
	nullable() bool
	firstPos() *bitset
	lastPos() *bitset
}

// leafNode represents a terminal (element or wildcard).
type leafNode struct {
	particle types.Particle
	first    *bitset
	pos      int
	min      types.Occurs
	max      types.Occurs
	size     int
}

func newLeaf(pos int, particle types.Particle, minOccurs, maxOccurs types.Occurs, size int) *leafNode {
	return &leafNode{pos: pos, particle: particle, min: minOccurs, max: maxOccurs, size: size}
}

func (n *leafNode) nullable() bool {
	return n.min.IsZero()
}

func (n *leafNode) firstPos() *bitset {
	if n.first == nil {
		n.first = newBitset(n.size)
		n.first.set(n.pos)
	}
	return n.first
}

func (n *leafNode) lastPos() *bitset {
	return n.firstPos() // same for leaves
}

// seqNode represents a sequence (concatenation) of two nodes.
type seqNode struct {
	left  node
	right node
	first *bitset
	last  *bitset
	size  int
}

func newSeq(left, right node, size int) *seqNode {
	return &seqNode{left: left, right: right, size: size}
}

func (n *seqNode) nullable() bool {
	return n.left.nullable() && n.right.nullable()
}

func (n *seqNode) firstPos() *bitset {
	if n.first != nil {
		return n.first
	}
	n.first = newBitset(n.size)
	n.first.or(n.left.firstPos())
	if n.left.nullable() {
		n.first.or(n.right.firstPos())
	}
	return n.first
}

func (n *seqNode) lastPos() *bitset {
	if n.last != nil {
		return n.last
	}
	n.last = newBitset(n.size)
	n.last.or(n.right.lastPos())
	if n.right.nullable() {
		n.last.or(n.left.lastPos())
	}
	return n.last
}

// altNode represents a choice (alternation) between two nodes.
type altNode struct {
	left  node
	right node
	first *bitset
	last  *bitset
	size  int
}

func newAlt(left, right node, size int) *altNode {
	return &altNode{left: left, right: right, size: size}
}

func (n *altNode) nullable() bool {
	return n.left.nullable() || n.right.nullable()
}

func (n *altNode) firstPos() *bitset {
	if n.first != nil {
		return n.first
	}
	n.first = newBitset(n.size)
	n.first.or(n.left.firstPos())
	n.first.or(n.right.firstPos())
	return n.first
}

func (n *altNode) lastPos() *bitset {
	if n.last != nil {
		return n.last
	}
	n.last = newBitset(n.size)
	n.last.or(n.left.lastPos())
	n.last.or(n.right.lastPos())
	return n.last
}

func ensureFirstPos(child node, cached *bitset) *bitset {
	if cached == nil {
		return child.firstPos().clone()
	}
	return cached
}

func ensureLastPos(child node, cached *bitset) *bitset {
	if cached == nil {
		return child.lastPos().clone()
	}
	return cached
}

// starNode represents zero-or-more repetition (Kleene star).
type starNode struct {
	child node
	first *bitset
	last  *bitset
	size  int
}

func newStar(child node, size int) *starNode {
	return &starNode{child: child, size: size}
}

func (n *starNode) nullable() bool { return true }

func (n *starNode) firstPos() *bitset {
	n.first = ensureFirstPos(n.child, n.first)
	return n.first
}

func (n *starNode) lastPos() *bitset {
	n.last = ensureLastPos(n.child, n.last)
	return n.last
}

// plusNode represents one-or-more repetition.
type plusNode struct {
	child node
	first *bitset
	last  *bitset
	size  int
}

func newPlus(child node, size int) *plusNode {
	return &plusNode{child: child, size: size}
}

func (n *plusNode) nullable() bool {
	return n.child.nullable()
}

func (n *plusNode) firstPos() *bitset {
	n.first = ensureFirstPos(n.child, n.first)
	return n.first
}

func (n *plusNode) lastPos() *bitset {
	n.last = ensureLastPos(n.child, n.last)
	return n.last
}

// optNode represents zero-or-one (optional).
type optNode struct {
	child node
	first *bitset
	last  *bitset
	size  int
}

func newOpt(child node, size int) *optNode {
	return &optNode{child: child, size: size}
}

func (n *optNode) nullable() bool { return true }

func (n *optNode) firstPos() *bitset {
	n.first = ensureFirstPos(n.child, n.first)
	return n.first
}

func (n *optNode) lastPos() *bitset {
	n.last = ensureLastPos(n.child, n.last)
	return n.last
}
