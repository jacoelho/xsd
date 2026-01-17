package xmlstream

import (
	"bytes"
	"errors"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

// Common XML namespaces.
const (
	XMLNamespace   = "http://www.w3.org/XML/1998/namespace"
	XMLNSNamespace = "http://www.w3.org/2000/xmlns/"
	XSINamespace   = "http://www.w3.org/2001/XMLSchema-instance"
	XSDNamespace   = "http://www.w3.org/2001/XMLSchema"
)

var errUnboundPrefix = errors.New("unbound namespace prefix")

// ErrUnboundPrefix reports usage of an undeclared namespace prefix.
var ErrUnboundPrefix = errUnboundPrefix

// NamespaceDecl reports a namespace declaration on the current element.
type NamespaceDecl struct {
	Prefix string
	URI    string
}

var (
	xmlnsBytes = []byte("xmlns")
	xmlBytes   = []byte("xml")
)

type nsScope struct {
	prefixes   map[string]string
	defaultNS  string
	decls      []NamespaceDecl
	defaultSet bool
}

type nsStack struct {
	scopes []nsScope
}

func (s *nsStack) push(scope nsScope) int {
	s.scopes = append(s.scopes, scope)
	return len(s.scopes) - 1
}

func (s *nsStack) pop() {
	if len(s.scopes) == 0 {
		return
	}
	s.scopes = s.scopes[:len(s.scopes)-1]
}

func (s *nsStack) depth() int {
	return len(s.scopes)
}

func (s *nsStack) lookup(prefix string, depth int) (string, bool) {
	if prefix == "xml" {
		return XMLNamespace, true
	}
	if depth >= len(s.scopes) {
		depth = len(s.scopes) - 1
	}
	if prefix == "" {
		for i := depth; i >= 0; i-- {
			scope := s.scopes[i]
			if scope.defaultSet {
				return scope.defaultNS, true
			}
		}
		// no default namespace declared; use empty namespace.
		return "", true
	}
	for i := depth; i >= 0; i-- {
		scope := s.scopes[i]
		if ns, ok := scope.prefixes[prefix]; ok {
			return ns, true
		}
	}
	return "", false
}

func collectNamespaceScope(dec *xmltext.Decoder, nsBuf []byte, tok *xmltext.Token) (nsScope, []byte, error) {
	scope := nsScope{}
	for _, attr := range tok.Attrs {
		if isDefaultNamespaceDecl(attr.Name) {
			var value string
			if attr.ValueNeeds {
				var err error
				nsBuf, value, err = decodeNamespaceValueString(dec, nsBuf, attr.Value)
				if err != nil {
					return nsScope{}, nsBuf, err
				}
			} else {
				nsBuf, value = appendNamespaceValue(nsBuf, attr.Value)
			}
			scope.defaultNS = value
			scope.defaultSet = true
			scope.decls = append(scope.decls, NamespaceDecl{Prefix: "", URI: value})
			continue
		}
		if local, ok := prefixedNamespaceDecl(attr.Name); ok {
			if bytes.Equal(local, xmlBytes) || bytes.Equal(local, xmlnsBytes) {
				continue
			}
			var value string
			if attr.ValueNeeds {
				var err error
				nsBuf, value, err = decodeNamespaceValueString(dec, nsBuf, attr.Value)
				if err != nil {
					return nsScope{}, nsBuf, err
				}
			} else {
				nsBuf, value = appendNamespaceValue(nsBuf, attr.Value)
			}
			if scope.prefixes == nil {
				scope.prefixes = make(map[string]string, 1)
			}
			prefix := string(local)
			scope.prefixes[prefix] = value
			scope.decls = append(scope.decls, NamespaceDecl{Prefix: prefix, URI: value})
		}
	}
	return scope, nsBuf, nil
}

func resolveAttrName(dec *xmltext.Decoder, ns *nsStack, name []byte, depth, line, column int) (string, []byte, error) {
	prefix, local, hasPrefix := splitQName(name)
	if !hasPrefix {
		if bytes.Equal(local, xmlnsBytes) {
			return XMLNSNamespace, local, nil
		}
		return "", local, nil
	}
	prefixName := unsafeString(prefix)
	if prefixName == "xmlns" {
		return XMLNSNamespace, local, nil
	}
	namespace, ok := ns.lookup(prefixName, depth)
	if !ok {
		return "", nil, unboundPrefixError(dec, line, column)
	}
	return namespace, local, nil
}

func isDefaultNamespaceDecl(name []byte) bool {
	return bytes.Equal(name, xmlnsBytes)
}

func prefixedNamespaceDecl(name []byte) ([]byte, bool) {
	prefix, local, hasPrefix := splitQName(name)
	if !hasPrefix {
		return nil, false
	}
	if !bytes.Equal(prefix, xmlnsBytes) {
		return nil, false
	}
	if bytes.Equal(local, xmlBytes) {
		return nil, false
	}
	return local, true
}

func unboundPrefixError(dec *xmltext.Decoder, line, column int) error {
	if dec == nil {
		return &xmltext.SyntaxError{Line: line, Column: column, Err: errUnboundPrefix}
	}
	return &xmltext.SyntaxError{
		Offset: dec.InputOffset(),
		Line:   line,
		Column: column,
		Path:   dec.StackPointer(),
		Err:    errUnboundPrefix,
	}
}

func splitQName(name []byte) (prefix, local []byte, hasPrefix bool) {
	if i := bytes.IndexByte(name, ':'); i >= 0 {
		return name[:i], name[i+1:], true
	}
	return nil, name, false
}
