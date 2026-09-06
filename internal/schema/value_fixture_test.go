package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func builtinValueForTest(tb testing.TB, typeName, lexical string) value.Value {
	tb.Helper()
	id, ok := value.BuiltinTypeID(typeName)
	if !ok {
		tb.Fatalf("unknown builtin %q", typeName)
	}
	got, err := value.NewBuilder(value.BuilderOptions{}).Validate(id, lexical, value.Resolver{}, value.NeedCanonical|value.NeedIdentity, nil)
	if err != nil {
		tb.Fatalf("validate %s value %q: %v", typeName, lexical, err)
	}
	return got
}
