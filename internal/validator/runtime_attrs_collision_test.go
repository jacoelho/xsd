//go:build forcedcollide

package validator

import (
	"strconv"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func collisionSchemaSession(tb testing.TB) (*Session, runtime.TypeID) {
	tb.Helper()
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:anyAttribute namespace="##any" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	rt := mustBuildRuntimeSchema(tb, schema)
	sess := NewSession(rt)
	sym := rt.Symbols.Lookup(rt.PredefNS.Empty, []byte("root"))
	elemID, ok := sess.globalElementBySymbol(sym)
	if !ok {
		tb.Fatalf("root element symbol not found")
	}
	elem := rt.Elements[elemID]
	return sess, elem.Type
}

func TestAttrDuplicateDetectionCollisionSafeDifferentNames(t *testing.T) {
	sess, typeID := collisionSchemaSession(t)
	attrs := []StartAttr{
		{Local: []byte("a"), NSBytes: []byte("urn:one"), Value: []byte("1")},
		{Local: []byte("b"), NSBytes: []byte("urn:two"), Value: []byte("2")},
	}
	if _, err := sess.ValidateAttributes(typeID, attrs, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAttrDuplicateDetectionCollisionSafeDuplicate(t *testing.T) {
	sess, typeID := collisionSchemaSession(t)
	attrs := []StartAttr{
		{Local: []byte("dup"), NSBytes: []byte("urn:one"), Value: []byte("1")},
		{Local: []byte("dup"), NSBytes: []byte("urn:one"), Value: []byte("2")},
	}
	_, err := sess.ValidateAttributes(typeID, attrs, nil)
	if err == nil {
		t.Fatalf("expected duplicate attribute error")
	}
	code, ok := validationErrorInfo(err)
	if !ok || code != xsderrors.ErrXMLParse {
		t.Fatalf("expected %s, got %v", xsderrors.ErrXMLParse, err)
	}
}

func Benchmark_AttrDuplicateDetection_CollisionSafe(b *testing.B) {
	const attrCount = 64
	attrs := make([]StartAttr, attrCount)
	for i := 0; i < attrCount; i++ {
		local := make([]byte, 0, 8)
		local = append(local, 'a')
		local = strconv.AppendInt(local, int64(i), 10)
		attrs[i] = StartAttr{
			Local:   local,
			NSBytes: []byte("urn:bench"),
			Value:   []byte("v"),
		}
	}
	sess, _ := collisionSchemaSession(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		classified, err := sess.classifyAttrs(attrs, true)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		if classified.duplicateErr != nil {
			b.Fatalf("unexpected duplicate error: %v", classified.duplicateErr)
		}
	}
}
