package xmlstream

import "iter"

func (r *Reader) LookupNamespace(prefix string) (string, bool) {
	if r == nil {
		return "", false
	}
	return r.ns.lookup(prefix, r.ns.depth()-1)
}

func (r *Reader) LookupNamespaceBytes(prefix []byte) (string, bool) {
	if r == nil {
		return "", false
	}
	return r.LookupNamespaceBytesAt(prefix, r.ns.depth()-1)
}

func (r *Reader) LookupNamespaceBytesAt(prefix []byte, depth int) (string, bool) {
	if r == nil {
		return "", false
	}
	return r.ns.lookup(unsafeString(prefix), depth)
}

func (r *Reader) LookupNamespaceAt(prefix string, depth int) (string, bool) {
	if r == nil {
		return "", false
	}
	return r.ns.lookup(prefix, depth)
}

func (r *Reader) NamespaceDeclsSeq(depth int) iter.Seq[NamespaceDecl] {
	return func(yield func(NamespaceDecl) bool) {
		decls := r.NamespaceDecls(depth)
		for _, decl := range decls {
			if !yield(decl) {
				return
			}
		}
	}
}

func (r *Reader) NamespaceDecls(depth int) []NamespaceDecl {
	if r == nil || len(r.ns.scopes) == 0 || depth < 0 {
		return nil
	}
	if depth >= len(r.ns.scopes) {
		depth = len(r.ns.scopes) - 1
	}
	scope := r.ns.scopes[depth]
	if scope.declLen == 0 {
		return nil
	}
	return r.ns.decls[scope.declStart : scope.declStart+scope.declLen]
}

func (r *Reader) CurrentNamespaceDeclsSeq() iter.Seq[NamespaceDecl] {
	if r == nil {
		return func(func(NamespaceDecl) bool) {}
	}
	return r.NamespaceDeclsSeq(r.ns.depth() - 1)
}

func (r *Reader) popElementName() (elementStackEntry, error) {
	var name elementStackEntry
	var err error
	name, r.elemStack, err = popElementStack(r.elemStack, r.ns.depth())
	return name, err
}

func (r *Reader) currentScopeDepth() int {
	depth := r.ns.depth() - 1
	if depth < 0 {
		return 0
	}
	return depth
}
