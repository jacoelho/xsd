package schemaanalysis

import (
	"cmp"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// SortedQNames returns QNames in deterministic order (namespace, local).
func SortedQNames[V any](m map[model.QName]V) []model.QName {
	if len(m) == 0 {
		return nil
	}
	keys := make([]model.QName, 0, len(m))
	for qname := range m {
		keys = append(keys, qname)
	}
	slices.SortFunc(keys, func(a, b model.QName) int {
		if a.Namespace != b.Namespace {
			return cmp.Compare(a.Namespace, b.Namespace)
		}
		return cmp.Compare(a.Local, b.Local)
	})
	return keys
}

// SortedGlobalDecls returns a sorted copy of global declarations.
func SortedGlobalDecls(decls []parser.GlobalDecl) []parser.GlobalDecl {
	if len(decls) == 0 {
		return nil
	}
	out := append([]parser.GlobalDecl(nil), decls...)
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
