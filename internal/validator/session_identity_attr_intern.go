package validator

import (
	"bytes"
	"slices"
)

func (s *Session) internIdentityAttrName(ns, local []byte) identityAttrNameID {
	if s == nil {
		return 0
	}
	hash := attrNameHash(ns, local)
	if s.identityAttrBuckets == nil {
		s.identityAttrBuckets = make(map[uint64][]identityAttrNameID)
	}
	bucket := s.identityAttrBuckets[hash]
	for _, id := range bucket {
		if id == 0 {
			continue
		}
		entry := s.identityAttrNames[int(id)-1]
		if bytes.Equal(entry.ns, ns) && bytes.Equal(entry.local, local) {
			return id
		}
	}
	id := identityAttrNameID(len(s.identityAttrNames) + 1)
	s.identityAttrNames = append(s.identityAttrNames, identityAttrName{
		ns:    slices.Clone(ns),
		local: slices.Clone(local),
	})
	s.identityAttrBuckets[hash] = append(bucket, id)
	return id
}
