package runtime

import (
	"bytes"
)

// Accepts reports whether the namespace is allowed by the wildcard rule.
// nsID==0 triggers byte comparisons against the namespace table.
func (w WildcardRule) Accepts(nsBytes []byte, nsID NamespaceID, nsTable *NamespaceTable, nsList []NamespaceID) bool {
	localMatch := isLocalNamespace(nsBytes, nsID, nsTable)
	targetMatch := matchesNamespace(w.TargetNS, nsBytes, nsID, nsTable)
	enumMatch := false
	for _, id := range nsList {
		if matchesNamespace(id, nsBytes, nsID, nsTable) {
			enumMatch = true
			break
		}
	}
	switch w.NS.Kind {
	case NSAny:
		return true
	case NSOther:
		return !targetMatch && !localMatch
	case NSNotAbsent:
		return !localMatch
	case NSEnumeration:
		if w.NS.HasLocal && localMatch {
			return true
		}
		if w.NS.HasTarget && targetMatch {
			return true
		}
		return enumMatch
	default:
		return false
	}
}

// WildcardAccepts reports whether a wildcard rule accepts the namespace.
func (s *Schema) WildcardAccepts(ruleID WildcardID, nsBytes []byte, nsID NamespaceID) bool {
	rule, ok := s.Wildcard(ruleID)
	if !ok {
		return false
	}
	nsList := s.WildcardNamespaceSpan(rule.NS)
	namespaces := s.NamespaceTable()
	return rule.Accepts(nsBytes, nsID, &namespaces, nsList)
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
