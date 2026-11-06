package schema

import (
	"strings"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"

	"github.com/jacoelho/xsd/internal/uriref"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	wildcardNamespaceAny             = "##any"
	wildcardNamespaceOther           = "##other"
	wildcardNamespaceLocal           = "##local"
	wildcardNamespaceTargetNamespace = "##targetNamespace"
)

// wildcardAttrs is the raw wildcard attribute projection from xs:any or
// xs:anyAttribute.
type wildcardAttrs struct {
	Namespace          string
	ProcessContents    string
	TargetNamespace    string
	HasNamespace       bool
	HasProcessContents bool
}

// parseWildcard parses compile-time wildcard namespace and processContents
// attributes.
func parseWildcard(names nameInterner, attrs wildcardAttrs) (wildcardRuntime, error) {
	wildcard, err := parseWildcardNamespace(names, attrs)
	if err != nil {
		return wildcardRuntime{}, err
	}
	process, err := parseWildcardProcessContents(attrs)
	if err != nil {
		return wildcardRuntime{}, err
	}
	wildcard.Process = process
	return wildcard, nil
}

func parseWildcardNamespace(names nameInterner, attrs wildcardAttrs) (wildcardRuntime, error) {
	nsSpec := wildcardNamespaceAny
	if attrs.HasNamespace {
		nsSpec = attrs.Namespace
	}
	switch nsSpec {
	case wildcardNamespaceAny:
		return wildcardRuntime{Mode: WildcardAny}, nil
	case wildcardNamespaceOther:
		ns, err := internWildcardNamespace(names, attrs.TargetNamespace)
		if err != nil {
			return wildcardRuntime{}, err
		}
		return wildcardRuntime{Mode: WildcardOther, OtherThan: ns}, nil
	case wildcardNamespaceLocal:
		return wildcardRuntime{Mode: WildcardLocal}, nil
	case wildcardNamespaceTargetNamespace:
		ns, err := internWildcardNamespace(names, attrs.TargetNamespace)
		if err != nil {
			return wildcardRuntime{}, err
		}
		return wildcardRuntime{Mode: WildcardTargetNamespace, Namespaces: []namespaceID{ns}}, nil
	default:
		namespaces, err := parseWildcardNamespaceList(names, attrs.TargetNamespace, nsSpec)
		if err != nil {
			return wildcardRuntime{}, err
		}
		return wildcardRuntime{Mode: WildcardList, Namespaces: namespaces}, nil
	}
}

func parseWildcardNamespaceList(names nameInterner, targetNS, nsSpec string) ([]namespaceID, error) {
	var namespaces []namespaceID
	for part := range lex.XMLFieldsSeq(nsSpec) {
		uri, err := wildcardNamespaceURI(part, targetNS)
		if err != nil {
			return nil, err
		}
		ns, err := internWildcardNamespace(names, uri)
		if err != nil {
			return nil, err
		}
		namespaces = append(namespaces, ns)
	}
	return normalizeNamespaceList(namespaces), nil
}

func wildcardNamespaceURI(part, targetNS string) (string, error) {
	switch part {
	case wildcardNamespaceLocal:
		return "", nil
	case wildcardNamespaceTargetNamespace:
		return targetNS, nil
	default:
		if strings.HasPrefix(part, "##") {
			return "", xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid wildcard namespace "+diagnostictext.Quote(part))
		}
		if _, err := uriref.Check(part); err != nil {
			return "", xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid wildcard namespace "+diagnostictext.Quote(part))
		}
		return part, nil
	}
}

func parseWildcardProcessContents(attrs wildcardAttrs) (ProcessContents, error) {
	const processContentsStrict = "strict"

	process := processContentsStrict
	if attrs.HasProcessContents {
		process = attrs.ProcessContents
	}
	switch process {
	case "skip":
		return ProcessSkip, nil
	case "lax":
		return ProcessLax, nil
	case processContentsStrict:
		return processStrict, nil
	default:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid processContents")
	}
}

func internWildcardNamespace(names nameInterner, uri string) (namespaceID, error) {
	if names.table == nil {
		return 0, xsderrors.InternalInvariant("wildcard parser requires namespace interner")
	}
	return names.InternNamespace(uri)
}
