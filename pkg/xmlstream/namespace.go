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
	defaultNS  string
	prevNS     string
	declStart  int
	declLen    int
	undoStart  int
	undoLen    int
	defaultSet bool
	prevSet    bool
}

type nsStack struct {
	prefixes   map[string]string
	defaultNS  string
	scopes     []nsScope
	undo       []nsUndo
	decls      []NamespaceDecl
	defaultSet bool
}

type nsUndo struct {
	prefix  string
	prev    string
	hadPrev bool
}

func (s *nsStack) push(scope nsScope) int {
	if scope.defaultSet {
		scope.prevNS = s.defaultNS
		scope.prevSet = s.defaultSet
		s.defaultNS = scope.defaultNS
		s.defaultSet = true
	}
	if scope.declLen > 0 {
		if s.prefixes == nil {
			s.prefixes = make(map[string]string, scope.declLen)
		}
		start := len(s.undo)
		decls := s.decls[scope.declStart : scope.declStart+scope.declLen]
		for _, decl := range decls {
			if decl.Prefix == "" {
				continue
			}
			prev, ok := s.prefixes[decl.Prefix]
			s.undo = append(s.undo, nsUndo{prefix: decl.Prefix, prev: prev, hadPrev: ok})
			s.prefixes[decl.Prefix] = decl.URI
		}
		scope.undoStart = start
		scope.undoLen = len(s.undo) - start
	}
	s.scopes = append(s.scopes, scope)
	return len(s.scopes) - 1
}

func (s *nsStack) pop() {
	if len(s.scopes) == 0 {
		return
	}
	scope := s.scopes[len(s.scopes)-1]
	s.scopes = s.scopes[:len(s.scopes)-1]
	if scope.defaultSet {
		s.defaultNS = scope.prevNS
		s.defaultSet = scope.prevSet
	}
	if scope.undoLen > 0 {
		for i := scope.undoStart + scope.undoLen - 1; i >= scope.undoStart; i-- {
			undo := s.undo[i]
			if undo.hadPrev {
				s.prefixes[undo.prefix] = undo.prev
			} else {
				delete(s.prefixes, undo.prefix)
			}
		}
		s.undo = s.undo[:scope.undoStart]
	}
	if scope.declLen > 0 {
		s.decls = s.decls[:scope.declStart]
	}
}

func (s *nsStack) depth() int {
	return len(s.scopes)
}

func (s *nsStack) reset() {
	if s == nil {
		return
	}
	s.scopes = s.scopes[:0]
	if s.prefixes != nil {
		clear(s.prefixes)
	}
	s.defaultNS = ""
	s.defaultSet = false
	s.undo = s.undo[:0]
	s.decls = s.decls[:0]
}

func (s *nsStack) lookup(prefix string, depth int) (string, bool) {
	if prefix == "xml" {
		return XMLNamespace, true
	}
	if depth >= len(s.scopes) {
		depth = len(s.scopes) - 1
	}
	if prefix == "" {
		if depth == len(s.scopes)-1 {
			if s.defaultSet {
				return s.defaultNS, true
			}
			// no default namespace declared; use empty namespace.
			return "", true
		}
		for i := depth; i >= 0; i-- {
			scope := s.scopes[i]
			if scope.defaultSet {
				return scope.defaultNS, true
			}
		}
		return "", true
	}
	if depth == len(s.scopes)-1 {
		ns, ok := s.prefixes[prefix]
		return ns, ok
	}
	for i := depth; i >= 0; i-- {
		scope := s.scopes[i]
		for j := scope.declStart + scope.declLen - 1; j >= scope.declStart; j-- {
			decl := s.decls[j]
			if decl.Prefix == prefix {
				return decl.URI, true
			}
		}
	}
	return "", false
}

func collectNamespaceScope(dec *xmltext.Decoder, nsBuf []byte, declBuf []NamespaceDecl, tok *xmltext.RawTokenSpan) (nsScope, []byte, []NamespaceDecl, error) {
	scope := nsScope{}
	for i := 0; i < tok.AttrCount(); i++ {
		attrName := tok.AttrName(i)
		if isDefaultNamespaceDecl(attrName) {
			var value string
			if tok.AttrValueNeeds(i) {
				var err error
				nsBuf, value, err = decodeNamespaceValueString(dec, nsBuf, tok.AttrValue(i))
				if err != nil {
					return nsScope{}, nsBuf, declBuf, err
				}
			} else {
				nsBuf, value = appendNamespaceValue(nsBuf, tok.AttrValue(i))
			}
			scope.defaultNS = value
			scope.defaultSet = true
			declBuf = append(declBuf, NamespaceDecl{Prefix: "", URI: value})
			continue
		}
		if local, ok := prefixedNamespaceDecl(attrName); ok {
			if bytes.Equal(local, xmlBytes) || bytes.Equal(local, xmlnsBytes) {
				continue
			}
			var value string
			if tok.AttrValueNeeds(i) {
				var err error
				nsBuf, value, err = decodeNamespaceValueString(dec, nsBuf, tok.AttrValue(i))
				if err != nil {
					return nsScope{}, nsBuf, declBuf, err
				}
			} else {
				nsBuf, value = appendNamespaceValue(nsBuf, tok.AttrValue(i))
			}
			prefix := string(local)
			declBuf = append(declBuf, NamespaceDecl{Prefix: prefix, URI: value})
		}
	}
	return scope, nsBuf, declBuf, nil
}

func resolveAttrName(dec *xmltext.Decoder, ns *nsStack, name []byte, nameColon, depth, line, column int) (string, []byte, error) {
	prefix, local, hasPrefix := splitQNameWithColon(name, nameColon)
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

func splitQNameWithColon(name []byte, colon int) (prefix, local []byte, hasPrefix bool) {
	if len(name) == 0 {
		return nil, nil, false
	}
	if colon <= 0 || colon >= len(name)-1 {
		return nil, name, false
	}
	return name[:colon], name[colon+1:], true
}
