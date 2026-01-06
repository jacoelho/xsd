package contentmodel

// computeFollowPos computes the follow positions for all positions in the tree.
// This is the key insight of Glushkov construction.
//
// Rules:
//  1. For sequence a·b: for each p in lastPos(a), add firstPos(b) to followPos(p)
//  2. For repetition a* or a+: for each p in lastPos(a), add firstPos(a) to followPos(p)
func (b *Builder) computeFollowPos(n node) {
	switch v := n.(type) {
	case *seqNode:
		b.computeFollowPos(v.left)
		b.computeFollowPos(v.right)
		// Rule 1: sequence
		v.left.lastPos().forEach(func(pos int) {
			b.followPos[pos].or(v.right.firstPos())
		})

	case *altNode:
		b.computeFollowPos(v.left)
		b.computeFollowPos(v.right)

	case *starNode:
		b.computeFollowPos(v.child)
		// Rule 2: repetition loops back
		v.child.lastPos().forEach(func(pos int) {
			b.followPos[pos].or(v.child.firstPos())
		})

	case *plusNode:
		b.computeFollowPos(v.child)
		// Rule 2: repetition loops back
		v.child.lastPos().forEach(func(pos int) {
			b.followPos[pos].or(v.child.firstPos())
		})

	case *optNode:
		b.computeFollowPos(v.child)

	case *leafNode:
		// Leaves don't contribute followPos rules
	}
}
