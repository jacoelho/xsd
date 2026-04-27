package runtimetest

import "github.com/jacoelho/xsd/internal/runtime"

type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

func NewAssembler(tb TB) *runtime.Assembler {
	tb.Helper()
	return runtime.NewSchemaAssembler()
}

func Seal(tb TB, a *runtime.Assembler) *runtime.Schema {
	tb.Helper()
	schema, err := a.Seal()
	if err != nil {
		tb.Fatalf("seal runtime schema: %v", err)
	}
	return schema
}

func EmptySchema(tb TB) *runtime.Schema {
	tb.Helper()
	return Seal(tb, NewAssembler(tb))
}

func Mutate(tb TB, schema *runtime.Schema, apply func(*runtime.Assembler) error) {
	tb.Helper()
	if schema == nil {
		tb.Fatalf("mutate runtime schema: nil schema")
	}
	if err := apply(runtime.NewAssembler(schema)); err != nil {
		tb.Fatalf("mutate runtime schema: %v", err)
	}
}

func SetElement(tb TB, schema *runtime.Schema, id runtime.ElemID, mutate func(*runtime.Element)) {
	tb.Helper()
	elem, ok := schema.Element(id)
	if !ok {
		tb.Fatalf("runtime element %d not found", id)
	}
	mutate(&elem)
	Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetElement(id, elem)
	})
}

func SetType(tb TB, schema *runtime.Schema, id runtime.TypeID, mutate func(*runtime.Type)) {
	tb.Helper()
	typ, ok := schema.Type(id)
	if !ok {
		tb.Fatalf("runtime type %d not found", id)
	}
	mutate(&typ)
	Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetType(id, typ)
	})
}

func SetComplexType(tb TB, schema *runtime.Schema, id uint32, mutate func(*runtime.ComplexType)) {
	tb.Helper()
	ct, ok := schema.ComplexType(id)
	if !ok {
		tb.Fatalf("runtime complex type %d not found", id)
	}
	mutate(&ct)
	Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetComplexType(id, ct)
	})
}
