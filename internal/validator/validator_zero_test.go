package validator

import (
	"errors"
	"strings"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func TestValidatorZeroRejected(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:string"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	ns := rt.PredefNS.Empty
	rootSym := rt.Symbols.Lookup(ns, []byte("root"))
	if rootSym == 0 || int(rootSym) >= len(rt.GlobalElements) {
		t.Fatalf("root symbol not found")
	}
	rootID := rt.GlobalElements[rootSym]
	if int(rootID) >= len(rt.Elements) {
		t.Fatalf("root element not found")
	}
	root := rt.Elements[rootID]
	if int(root.Type) >= len(rt.Types) {
		t.Fatalf("root type out of range")
	}
	rootType := rt.Types[root.Type]
	if rootType.Kind != runtime.TypeComplex {
		t.Fatalf("expected root to be complex type")
	}
	if rootType.Complex.ID == 0 || int(rootType.Complex.ID) >= len(rt.ComplexTypes) {
		t.Fatalf("root complex type not found")
	}
	attrSym := rt.Symbols.Lookup(ns, []byte("a"))
	if attrSym == 0 {
		t.Fatalf("attribute symbol not found")
	}
	attrs := rt.ComplexTypes[rootType.Complex.ID].Attrs
	uses := sliceAttrUses(rt.AttrIndex.Uses, attrs)
	if len(uses) == 0 {
		t.Fatalf("expected attribute uses")
	}
	_, idx, ok := lookupAttrUse(rt, attrs, attrSym)
	if !ok {
		t.Fatalf("attribute use not found")
	}
	uses[idx].Validator = 0

	sess := NewSession(rt, xmlstream.MaxDepth(64))
	err := sess.Validate(strings.NewReader(`<root a="x"/>`))
	if err == nil {
		t.Fatalf("expected validator missing error")
	}
	var list xsderrors.ValidationList
	if !errors.As(err, &list) {
		t.Fatalf("expected ValidationList, got %T", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected validation errors")
	}
	if list[0].Code != string(xsderrors.ErrDatatypeInvalid) {
		t.Fatalf("expected ErrDatatypeInvalid, got %v", list)
	}
	if !strings.Contains(list[0].Message, "validator missing") {
		t.Fatalf("expected validator missing message, got %q", list[0].Message)
	}
}
