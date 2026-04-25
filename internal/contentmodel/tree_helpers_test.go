package contentmodel

func elem(_ string, minOccurs, maxOccurs int) TreeParticle {
	id := nextTestElementID()
	return TreeParticle{
		Kind:      TreeElement,
		ElementID: id,
		Min:       testOccurs(minOccurs),
		Max:       testOccurs(maxOccurs),
	}
}

func wildcard(minOccurs, maxOccurs int) TreeParticle {
	return TreeParticle{
		Kind:       TreeWildcard,
		WildcardID: 1,
		Min:        testOccurs(minOccurs),
		Max:        testOccurs(maxOccurs),
	}
}

func sequence(particles ...TreeParticle) TreeParticle {
	return TreeParticle{
		Kind:     TreeGroup,
		Group:    TreeSequence,
		Min:      testOccurs(1),
		Max:      testOccurs(1),
		Children: particles,
	}
}

var testElementID uint32

func nextTestElementID() uint32 {
	testElementID++
	return testElementID
}

func testOccurs(value int) TreeOccurs {
	if value < 0 {
		return TreeOccurs{Unbounded: true}
	}
	return TreeOccurs{Value: uint32(value)}
}
