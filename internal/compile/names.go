package compile

import (
	"cmp"
	"errors"
	"maps"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// NameInterner adapts runtime name interning to compile-phase diagnostics.
type NameInterner struct {
	inner runtime.NameInterner
}

// NewNameTable returns a runtime name table with compile diagnostics.
func NewNameTable(maxNames int) (runtime.NameTable, error) {
	n, err := runtime.NewRuntimeNameTable(maxNames)
	return n, NameTableError(err)
}

// NewNameInterner returns a compile diagnostic adapter over table.
func NewNameInterner(table *runtime.NameTable) NameInterner {
	return NameInterner{inner: runtime.NewNameInterner(table)}
}

// IsZero reports whether n has no runtime table attached.
func (n NameInterner) IsZero() bool {
	return n == NameInterner{}
}

// InternNamespace interns uri into the runtime name table.
func (n NameInterner) InternNamespace(uri string) (runtime.NamespaceID, error) {
	id, err := n.inner.InternNamespace(uri)
	return id, NameTableError(err)
}

// InternLocal interns local into the runtime name table.
func (n NameInterner) InternLocal(local string) (runtime.LocalNameID, error) {
	id, err := n.inner.InternLocal(local)
	return id, NameTableError(err)
}

// InternQName interns ns and local into the runtime name table.
func (n NameInterner) InternQName(ns, local string) (runtime.QName, error) {
	q, err := n.inner.InternQName(ns, local)
	return q, NameTableError(err)
}

// NameTableError maps runtime name-table limit failures to schema diagnostics.
func NameTableError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, runtime.ErrNameLimit):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, runtime.ErrNameLimit.Error())
	case errors.Is(err, runtime.ErrNamespaceLimit):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, runtime.ErrNamespaceLimit.Error())
	case errors.Is(err, runtime.ErrLocalNameLimit):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, runtime.ErrLocalNameLimit.Error())
	default:
		return err
	}
}

// SortedQNames returns map keys ordered by expanded name text.
func SortedQNames[T any](m map[runtime.QName]T, names runtime.NameTable) []runtime.QName {
	return slices.SortedFunc(maps.Keys(m), func(a, b runtime.QName) int {
		aNS := names.Namespace(a.Namespace)
		bNS := names.Namespace(b.Namespace)
		if aNS != bNS {
			return cmp.Compare(aNS, bNS)
		}
		return cmp.Compare(names.Local(a.Local), names.Local(b.Local))
	})
}
