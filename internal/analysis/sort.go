package analysis

import (
	"cmp"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/types"
)

// SortedQNames returns QNames in deterministic order (namespace, local).
func SortedQNames[V any](m map[types.QName]V) []types.QName {
	return qname.SortedMapKeys(m)
}

// SortedGlobalDecls returns a sorted copy of global declarations.
func SortedGlobalDecls(decls []parser.GlobalDecl) []parser.GlobalDecl {
	if len(decls) == 0 {
		return nil
	}
	out := slices.Clone(decls)
	slices.SortFunc(out, compareGlobalDecl)
	return out
}

func compareGlobalDecl(a, b parser.GlobalDecl) int {
	if a.Kind != b.Kind {
		return cmp.Compare(a.Kind, b.Kind)
	}
	if a.Name.Namespace != b.Name.Namespace {
		return cmp.Compare(a.Name.Namespace, b.Name.Namespace)
	}
	return cmp.Compare(a.Name.Local, b.Name.Local)
}
