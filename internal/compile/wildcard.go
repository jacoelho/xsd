package compile

import (
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/uriref"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	wildcardNamespaceAny             = "##any"
	wildcardNamespaceOther           = "##other"
	wildcardNamespaceLocal           = "##local"
	wildcardNamespaceTargetNamespace = "##targetNamespace"
)

// NamespaceInterner interns namespace URIs while parsing wildcard namespace
// declarations.
type NamespaceInterner interface {
	InternNamespace(uri string) (runtime.NamespaceID, error)
}

// WildcardAttrs is the raw wildcard attribute projection from xs:any or
// xs:anyAttribute.
type WildcardAttrs struct {
	Namespace          string
	ProcessContents    string
	TargetNamespace    string
	HasNamespace       bool
	HasProcessContents bool
}

// ParseWildcard parses compile-time wildcard namespace and processContents
// attributes.
func ParseWildcard(names NamespaceInterner, attrs WildcardAttrs) (runtime.Wildcard, error) {
	wildcard, err := parseWildcardNamespace(names, attrs)
	if err != nil {
		return runtime.Wildcard{}, err
	}
	process, err := parseWildcardProcessContents(attrs)
	if err != nil {
		return runtime.Wildcard{}, err
	}
	wildcard.Process = process
	return wildcard, nil
}

func parseWildcardNamespace(names NamespaceInterner, attrs WildcardAttrs) (runtime.Wildcard, error) {
	nsSpec := wildcardNamespaceAny
	if attrs.HasNamespace {
		nsSpec = attrs.Namespace
	}
	switch nsSpec {
	case wildcardNamespaceAny:
		return runtime.Wildcard{Mode: runtime.WildcardAny}, nil
	case wildcardNamespaceOther:
		ns, err := internWildcardNamespace(names, attrs.TargetNamespace)
		if err != nil {
			return runtime.Wildcard{}, err
		}
		return runtime.Wildcard{Mode: runtime.WildcardOther, OtherThan: ns}, nil
	case wildcardNamespaceLocal:
		return runtime.Wildcard{Mode: runtime.WildcardLocal}, nil
	case wildcardNamespaceTargetNamespace:
		ns, err := internWildcardNamespace(names, attrs.TargetNamespace)
		if err != nil {
			return runtime.Wildcard{}, err
		}
		return runtime.Wildcard{Mode: runtime.WildcardTargetNamespace, Namespaces: []runtime.NamespaceID{ns}}, nil
	default:
		namespaces, err := parseWildcardNamespaceList(names, attrs.TargetNamespace, nsSpec)
		if err != nil {
			return runtime.Wildcard{}, err
		}
		return runtime.Wildcard{Mode: runtime.WildcardList, Namespaces: namespaces}, nil
	}
}

func parseWildcardNamespaceList(names NamespaceInterner, targetNS, nsSpec string) ([]runtime.NamespaceID, error) {
	var namespaces []runtime.NamespaceID
	for part := range lex.XMLFieldsSeq(nsSpec) {
		var uri string
		switch part {
		case wildcardNamespaceLocal:
			uri = ""
		case wildcardNamespaceTargetNamespace:
			uri = targetNS
		default:
			if strings.HasPrefix(part, "##") {
				return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid wildcard namespace "+part)
			}
			if _, err := uriref.Check(part); err != nil {
				return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid wildcard namespace "+part)
			}
			uri = part
		}
		ns, err := internWildcardNamespace(names, uri)
		if err != nil {
			return nil, err
		}
		namespaces = append(namespaces, ns)
	}
	return runtime.NormalizeNamespaceList(namespaces), nil
}

func parseWildcardProcessContents(attrs WildcardAttrs) (runtime.ProcessContents, error) {
	process := "strict"
	if attrs.HasProcessContents {
		process = attrs.ProcessContents
	}
	switch process {
	case "skip":
		return runtime.ProcessSkip, nil
	case "lax":
		return runtime.ProcessLax, nil
	case "strict":
		return runtime.ProcessStrict, nil
	default:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "invalid processContents")
	}
}

func internWildcardNamespace(names NamespaceInterner, uri string) (runtime.NamespaceID, error) {
	if names == nil {
		return 0, xsderrors.InternalInvariant("wildcard parser requires namespace interner")
	}
	return names.InternNamespace(uri)
}
