package validatorgen

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestPrimitiveNameIntegerDerivedUsesDecimal(t *testing.T) {
	t.Parallel()

	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="MyInt">
    <xs:restriction base="xs:int"/>
  </xs:simpleType>
</xs:schema>`

	sch := mustResolveSchema(t, schemaXML)
	typ := sch.TypeDefs[model.QName{Local: "MyInt"}]
	if typ == nil {
		t.Fatal("type MyInt not found")
	}

	name, err := newTypeResolver(sch).primitiveName(typ)
	if err != nil {
		t.Fatalf("primitiveName() error = %v", err)
	}
	if name != "decimal" {
		t.Fatalf("primitiveName() = %q, want %q", name, "decimal")
	}
}

func TestIntegerDerivedFlowsUseDecimalPrimitivePath(t *testing.T) {
	t.Parallel()

	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="MyInt">
    <xs:restriction base="xs:int"/>
  </xs:simpleType>
  <xs:simpleType name="MyPositive">
    <xs:restriction base="xs:positiveInteger"/>
  </xs:simpleType>
</xs:schema>`

	sch := mustResolveSchema(t, schemaXML)
	myInt := sch.TypeDefs[model.QName{Local: "MyInt"}]
	myPositive := sch.TypeDefs[model.QName{Local: "MyPositive"}]
	if myInt == nil || myPositive == nil {
		t.Fatalf("expected MyInt and MyPositive types")
	}

	comp := newCompiler(sch)

	canonical, err := comp.canonicalizeAtomic("01", myInt, nil)
	if err != nil {
		t.Fatalf("canonicalizeAtomic(MyInt) error = %v", err)
	}
	if string(canonical) != "1" {
		t.Fatalf("canonicalizeAtomic(MyInt) = %q, want %q", canonical, "1")
	}

	if _, err := comp.canonicalizeAtomic("0", myPositive, nil); err == nil {
		t.Fatal("canonicalizeAtomic(MyPositive) expected integer-kind error, got nil")
	}

	key, err := comp.keyBytesAtomic("01", myInt, nil)
	if err != nil {
		t.Fatalf("keyBytesAtomic(MyInt) error = %v", err)
	}
	if key.kind != runtime.VKDecimal {
		t.Fatalf("keyBytesAtomic(MyInt).kind = %v, want %v", key.kind, runtime.VKDecimal)
	}
	wantKey := num.EncodeDecKey(nil, num.FromInt64(1).AsDec())
	if !bytes.Equal(key.bytes, wantKey) {
		t.Fatalf("keyBytesAtomic(MyInt).bytes = %v, want %v", key.bytes, wantKey)
	}

	comparable, err := comp.comparableValue("01", myInt)
	if err != nil {
		t.Fatalf("comparableValue(MyInt) error = %v", err)
	}
	if _, ok := comparable.(model.ComparableInt); !ok {
		t.Fatalf("comparableValue(MyInt) type = %T, want model.ComparableInt", comparable)
	}
}
