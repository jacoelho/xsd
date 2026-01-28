package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func TestPathStackStringIncludesNamespace(t *testing.T) {
	sess := NewSession(runtime.NewBuilder().Build())
	id1 := xmlstream.NameID(1)
	id2 := xmlstream.NameID(2)
	sess.internName(id1, []byte("urn:a"), []byte("root"))
	sess.internName(id2, []byte("urn:b"), []byte("root"))
	sess.elemStack = append(sess.elemStack, elemFrame{name: NameID(id1)}, elemFrame{name: NameID(id2)})

	if got := sess.pathString(); got != "/{urn:a}root/{urn:b}root" {
		t.Fatalf("path = %q, want %q", got, "/{urn:a}root/{urn:b}root")
	}
}
