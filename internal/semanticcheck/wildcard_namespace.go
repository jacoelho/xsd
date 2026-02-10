package semanticcheck

import "github.com/jacoelho/xsd/internal/model"

func processContentsStrongerOrEqual(derived, base model.ProcessContents) bool {
	switch base {
	case model.Strict:
		return derived == model.Strict
	case model.Lax:
		return derived == model.Lax || derived == model.Strict
	case model.Skip:
		return true
	default:
		return false
	}
}

func namespaceConstraintSubset(
	ns1 model.NamespaceConstraint,
	list1 []model.NamespaceURI,
	target1 model.NamespaceURI,
	ns2 model.NamespaceConstraint,
	list2 []model.NamespaceURI,
	target2 model.NamespaceURI,
) bool {
	if ns2 == model.NSCAny {
		return true
	}

	if ns1 == model.NSCAny {
		return false
	}

	switch ns1 {
	case model.NSCList:
		for _, ns := range list1 {
			resolved := ns
			if ns == model.NamespaceTargetPlaceholder {
				resolved = target1
			}
			if !model.AllowsNamespace(ns2, list2, target2, resolved) {
				return false
			}
		}
		return true
	case model.NSCTargetNamespace:
		return model.AllowsNamespace(ns2, list2, target2, target1)
	case model.NSCLocal:
		return model.AllowsNamespace(ns2, list2, target2, model.NamespaceEmpty)
	case model.NSCOther:
		if ns2 == model.NSCAny || ns2 == model.NSCNotAbsent {
			return true
		}
		if ns2 != model.NSCOther {
			return false
		}
		if target2 == "" {
			return true
		}
		return target1 == target2
	case model.NSCNotAbsent:
		switch ns2 {
		case model.NSCAny, model.NSCNotAbsent:
			return true
		case model.NSCOther:
			return target2 == ""
		default:
			return false
		}
	default:
		return false
	}
}

func processContentsName(pc model.ProcessContents) string {
	switch pc {
	case model.Strict:
		return "strict"
	case model.Lax:
		return "lax"
	case model.Skip:
		return "skip"
	default:
		return "unknown"
	}
}
