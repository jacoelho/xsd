package types

import "testing"

func mustBuiltinSimpleType(t *testing.T, name TypeName) *SimpleType {
	t.Helper()
	st, err := NewBuiltinSimpleType(name)
	if err != nil {
		t.Fatalf("NewBuiltinSimpleType(%s) failed: %v", name, err)
	}
	return st
}
