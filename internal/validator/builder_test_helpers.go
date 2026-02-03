package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func mustInternNamespace(tb testing.TB, b *runtime.Builder, uri []byte) runtime.NamespaceID {
	tb.Helper()
	id, err := b.InternNamespace(uri)
	if err != nil {
		tb.Fatalf("InternNamespace: %v", err)
	}
	return id
}

func mustInternSymbol(tb testing.TB, b *runtime.Builder, nsID runtime.NamespaceID, local []byte) runtime.SymbolID {
	tb.Helper()
	id, err := b.InternSymbol(nsID, local)
	if err != nil {
		tb.Fatalf("InternSymbol: %v", err)
	}
	return id
}
