package runtime

import "bytes"

// Accepts reports whether the namespace is allowed by the wildcard rule.
// nsID==0 triggers byte comparisons against the namespace table.
func (w WildcardRule) Accepts(nsBytes []byte, nsID NamespaceID, nsTable *NamespaceTable, nsList []NamespaceID) bool {
	switch w.NS.Kind {
	case NSAny:
		return true
	case NSOther:
		if matchesNamespace(w.TargetNS, nsBytes, nsID, nsTable) {
			return false
		}
		return !isLocalNamespace(nsBytes, nsID, nsTable)
	case NSNotAbsent:
		return !isLocalNamespace(nsBytes, nsID, nsTable)
	case NSEnumeration:
		if w.NS.HasLocal && isLocalNamespace(nsBytes, nsID, nsTable) {
			return true
		}
		if w.NS.HasTarget && matchesNamespace(w.TargetNS, nsBytes, nsID, nsTable) {
			return true
		}
		for _, id := range nsList {
			if matchesNamespace(id, nsBytes, nsID, nsTable) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// WildcardAccepts reports whether a wildcard rule accepts the namespace.
func (s *Schema) WildcardAccepts(ruleID WildcardID, nsBytes []byte, nsID NamespaceID) bool {
	if s == nil || ruleID == 0 || int(ruleID) >= len(s.Wildcards) {
		return false
	}
	rule := s.Wildcards[ruleID]
	nsList := sliceWildcardNS(rule.NS, s.WildcardNS)
	return rule.Accepts(nsBytes, nsID, &s.Namespaces, nsList)
}

func sliceWildcardNS(ns NSConstraint, list []NamespaceID) []NamespaceID {
	if ns.Len == 0 {
		return nil
	}
	off := ns.Off
	end := off + ns.Len
	if int(off) >= len(list) || int(end) > len(list) {
		return nil
	}
	return list[off:end]
}

func matchesNamespace(id NamespaceID, nsBytes []byte, nsID NamespaceID, nsTable *NamespaceTable) bool {
	if nsID != 0 {
		return nsID == id
	}
	if id == 0 {
		return len(nsBytes) == 0
	}
	if nsTable == nil {
		return false
	}
	stored := nsTable.Bytes(id)
	if stored == nil {
		return false
	}
	return bytes.Equal(stored, nsBytes)
}

func isLocalNamespace(nsBytes []byte, nsID NamespaceID, nsTable *NamespaceTable) bool {
	if nsID != 0 {
		if nsTable == nil {
			return false
		}
		return len(nsTable.Bytes(nsID)) == 0
	}
	return len(nsBytes) == 0
}
