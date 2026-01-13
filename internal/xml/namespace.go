package xsdxml

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

var (
	xmlnsBytes = []byte("xmlns")
	xmlBytes   = []byte("xml")
)

type nsScope struct {
	prefixes   map[string]string
	defaultNS  string
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
	if prefix == "" {
		for i := depth; i >= 0; i-- {
			if i >= len(s.scopes) {
				continue
			}
			scope := s.scopes[i]
			if scope.defaultSet {
				return scope.defaultNS, true
			}
		}
		return "", true
	}
	for i := depth; i >= 0; i-- {
		if i >= len(s.scopes) {
			continue
		}
		scope := s.scopes[i]
		if ns, ok := scope.prefixes[prefix]; ok {
			return ns, true
		}
	}
	return "", false
}

func collectNamespaceScope(dec *StreamDecoder, tok *xmltext.Token) (nsScope, error) {
	scope := nsScope{}
	for _, attr := range tok.Attrs {
		if isDefaultNamespaceDecl(attr.Name) {
			value, err := dec.namespaceValueString(attr.Value, attr.ValueNeeds)
			if err != nil {
				return nsScope{}, err
			}
			scope.defaultNS = value
			scope.defaultSet = true
			continue
		}
		if local, ok := prefixedNamespaceDecl(attr.Name); ok {
			if bytes.Equal(local, xmlBytes) || bytes.Equal(local, xmlnsBytes) {
				continue
			}
			value, err := dec.namespaceValueString(attr.Value, attr.ValueNeeds)
			if err != nil {
				return nsScope{}, err
			}
			if scope.prefixes == nil {
				scope.prefixes = make(map[string]string, 1)
			}
			scope.prefixes[string(local)] = value
		}
	}
	return scope, nil
}

func resolveAttrName(dec *xmltext.Decoder, ns *nsStack, name []byte, depth, line, column int) (string, string, error) {
	prefix, local, hasPrefix := splitQName(name)
	localName := string(local)
	if !hasPrefix {
		if localName == "xmlns" {
			return XMLNSNamespace, localName, nil
		}
		return "", localName, nil
	}
	prefixName := string(prefix)
	if prefixName == "xmlns" {
		return XMLNSNamespace, localName, nil
	}
	namespace, ok := ns.lookup(prefixName, depth)
	if !ok {
		return "", "", unboundPrefixError(dec, line, column)
	}
	return namespace, localName, nil
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
	for i, b := range name {
		if b == ':' {
			return name[:i], name[i+1:], true
		}
	}
	return nil, name, false
}
