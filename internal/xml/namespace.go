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
	for i, attr := range tok.Attrs {
		if isDefaultNamespaceDeclSpan(dec.dec, attr.Name) {
			value, err := dec.namespaceValueString(attr.ValueSpan, attrDecodeMode(tok, i))
			if err != nil {
				return nsScope{}, err
			}
			scope.defaultNS = value
			scope.defaultSet = true
			continue
		}
		if isPrefixedNamespaceDeclSpan(dec.dec, attr.Name) {
			local := spanString(dec.dec, attr.Name.Local)
			if local == "xml" || local == "xmlns" {
				continue
			}
			value, err := dec.namespaceValueString(attr.ValueSpan, attrDecodeMode(tok, i))
			if err != nil {
				return nsScope{}, err
			}
			if scope.prefixes == nil {
				scope.prefixes = make(map[string]string, 1)
			}
			scope.prefixes[local] = value
		}
	}
	return scope, nil
}

func resolveAttrName(dec *xmltext.Decoder, ns *nsStack, name xmltext.QNameSpan, depth, line, column int) (string, string, error) {
	local := spanString(dec, name.Local)
	if !name.HasPrefix {
		if local == "xmlns" {
			return XMLNSNamespace, local, nil
		}
		return "", local, nil
	}
	prefix := spanString(dec, name.Prefix)
	if prefix == "xmlns" {
		return XMLNSNamespace, local, nil
	}
	namespace, ok := ns.lookup(prefix, depth)
	if !ok {
		return "", "", unboundPrefixError(dec, line, column)
	}
	return namespace, local, nil
}

func spanString(dec *xmltext.Decoder, span xmltext.Span) string {
	if dec == nil {
		return ""
	}
	return dec.SpanString(span)
}

func isDefaultNamespaceDeclSpan(dec *xmltext.Decoder, name xmltext.QNameSpan) bool {
	if name.HasPrefix {
		return false
	}
	return spanEquals(dec, name.Local, xmlnsBytes)
}

func isPrefixedNamespaceDeclSpan(dec *xmltext.Decoder, name xmltext.QNameSpan) bool {
	if !name.HasPrefix {
		return false
	}
	if !spanEquals(dec, name.Prefix, xmlnsBytes) {
		return false
	}
	return !spanEquals(dec, name.Local, xmlBytes)
}

func spanEquals(dec *xmltext.Decoder, span xmltext.Span, literal []byte) bool {
	if dec == nil {
		return false
	}
	data := dec.SpanBytes(span)
	return bytes.Equal(data, literal)
}

func unboundPrefixError(dec *xmltext.Decoder, line, column int) error {
	if dec == nil {
		return &xmltext.SyntaxError{Line: line, Column: column, Err: errUnboundPrefix}
	}
	return &xmltext.SyntaxError{
		Offset: dec.InputOffset(),
		Line:   line,
		Column: column,
		Path:   dec.StackPath(nil),
		Err:    errUnboundPrefix,
	}
}
