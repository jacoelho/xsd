package runtime

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/schemaast"
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
	return schemaast.AllowsRuntimeNamespace(
		schemaast.RuntimeNamespaceConstraint{
			Kind:      runtimeConstraintToPolicy(w.NS.Kind),
			HasTarget: w.NS.HasTarget,
			HasLocal:  w.NS.HasLocal,
		},
		targetMatch,
		localMatch,
		enumMatch,
	)
}

func runtimeConstraintToPolicy(kind NSConstraintKind) schemaast.RuntimeNamespaceConstraintKind {
	switch kind {
	case NSAny:
		return schemaast.RuntimeNamespaceAny
	case NSOther:
		return schemaast.RuntimeNamespaceOther
	case NSEnumeration:
		return schemaast.RuntimeNamespaceEnumeration
	case NSNotAbsent:
		return schemaast.RuntimeNamespaceNotAbsent
	default:
		return schemaast.RuntimeNamespaceAny + 255
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
