package semanticcheck

import "github.com/jacoelho/xsd/internal/types"

func processContentsStrongerOrEqual(derived, base types.ProcessContents) bool {
	switch base {
	case types.Strict:
		return derived == types.Strict
	case types.Lax:
		return derived == types.Lax || derived == types.Strict
	case types.Skip:
		return true
	default:
		return false
	}
}

func namespaceConstraintSubset(
	ns1 types.NamespaceConstraint,
	list1 []types.NamespaceURI,
	target1 types.NamespaceURI,
	ns2 types.NamespaceConstraint,
	list2 []types.NamespaceURI,
	target2 types.NamespaceURI,
) bool {
	if ns2 == types.NSCAny {
		return true
	}

	if ns1 == types.NSCAny {
		return false
	}

	switch ns1 {
	case types.NSCList:
		for _, ns := range list1 {
			resolved := ns
			if ns == types.NamespaceTargetPlaceholder {
				resolved = target1
			}
			if !types.AllowsNamespace(ns2, list2, target2, resolved) {
				return false
			}
		}
		return true
	case types.NSCTargetNamespace:
		return types.AllowsNamespace(ns2, list2, target2, target1)
	case types.NSCLocal:
		return types.AllowsNamespace(ns2, list2, target2, types.NamespaceEmpty)
	case types.NSCOther:
		if ns2 == types.NSCAny || ns2 == types.NSCNotAbsent {
			return true
		}
		if ns2 != types.NSCOther {
			return false
		}
		if target2.IsEmpty() {
			return true
		}
		return target1 == target2
	case types.NSCNotAbsent:
		switch ns2 {
		case types.NSCAny, types.NSCNotAbsent:
			return true
		case types.NSCOther:
			return target2.IsEmpty()
		default:
			return false
		}
	default:
		return false
	}
}

func processContentsName(pc types.ProcessContents) string {
	switch pc {
	case types.Strict:
		return "strict"
	case types.Lax:
		return "lax"
	case types.Skip:
		return "skip"
	default:
		return "unknown"
	}
}
