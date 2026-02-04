package schema

import (
	"sort"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// SortedQNames returns QNames in deterministic order (namespace, local).
func SortedQNames[V any](m map[types.QName]V) []types.QName {
	if len(m) == 0 {
		return nil
	}
	keys := make([]types.QName, 0, len(m))
	for qname := range m {
		keys = append(keys, qname)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Namespace != keys[j].Namespace {
			return keys[i].Namespace < keys[j].Namespace
		}
		return keys[i].Local < keys[j].Local
	})
	return keys
}

// SortedGlobalDecls returns a sorted copy of global declarations.
func SortedGlobalDecls(decls []parser.GlobalDecl) []parser.GlobalDecl {
	if len(decls) == 0 {
		return nil
	}
	out := append([]parser.GlobalDecl(nil), decls...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Name.Namespace != out[j].Name.Namespace {
			return out[i].Name.Namespace < out[j].Name.Namespace
		}
		return out[i].Name.Local < out[j].Name.Local
	})
	return out
}
