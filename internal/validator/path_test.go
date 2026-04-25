package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

func TestPathStackStringIncludesNamespace(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	id1 := xmlstream.NameID(1)
	id2 := xmlstream.NameID(2)
	sess.internName(id1, []byte("urn:a"), []byte("root"))
	sess.internName(id2, []byte("urn:b"), []byte("root"))
	sess.elemStack = append(sess.elemStack, elemFrame{name: NameID(id1)}, elemFrame{name: NameID(id2)})

	if got := sess.pathString(); got != "/{urn:a}root/{urn:b}root" {
		t.Fatalf("path = %q, want %q", got, "/{urn:a}root/{urn:b}root")
	}
}

func TestInternNameSparseIDUsesMap(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	id := xmlstream.NameID(maxNameMapSize + 5)
	sess.internName(id, []byte("urn:big"), []byte("root"))
	if len(sess.Names.Dense) != 0 {
		t.Fatalf("dense name map len = %d, want 0", len(sess.Names.Dense))
	}
	if sess.Names.Sparse == nil {
		t.Fatalf("expected sparse name map to be initialized")
	}
	sess.elemStack = append(sess.elemStack, elemFrame{name: NameID(id)})
	if got := sess.pathString(); got != "/{urn:big}root" {
		t.Fatalf("path = %q, want %q", got, "/{urn:big}root")
	}
}
