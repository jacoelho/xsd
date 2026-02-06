package contentmodel

func (b *builder) computeFollowPos(n node) {
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
