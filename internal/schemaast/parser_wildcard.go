package schemaast

import (
	"fmt"
	"strings"
)

func parseNamespaceConstraint(value string) (NamespaceConstraint, []NamespaceURI, error) {
	tokens := strings.Fields(ApplyWhiteSpace(value, WhiteSpaceCollapse))
	if len(tokens) == 0 {
		return NSCLocal, nil, nil
	}
	if len(tokens) == 1 {
		switch tokens[0] {
		case "##any":
			return NSCAny, nil, nil
		case "##other":
			return NSCOther, nil, nil
		case "##targetNamespace":
			return NSCTargetNamespace, nil, nil
		case "##local":
			return NSCLocal, nil, nil
		}
	}

	list := make([]NamespaceURI, 0, len(tokens))
	seen := make(map[NamespaceURI]bool, len(tokens))
	for _, token := range tokens {
		var ns NamespaceURI
		switch token {
		case "##any", "##other":
			return NSCInvalid, nil, fmt.Errorf("%s cannot appear in a namespace list", token)
		case "##targetNamespace":
			ns = NamespaceTargetPlaceholder
		case "##local":
			ns = NamespaceEmpty
		default:
			if strings.HasPrefix(token, "##") {
				return NSCInvalid, nil, fmt.Errorf("unknown namespace token %s", token)
			}
			ns = NamespaceURI(token)
		}
		if seen[ns] {
			continue
		}
		seen[ns] = true
		list = append(list, ns)
	}
	return NSCList, list, nil
}
