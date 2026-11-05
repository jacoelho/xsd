package schema

import (
	"reflect"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

// Every leaf field of SimpleType and its value specification takes part in
// intrinsicSimpleTypeEqual. A field added without extending the comparator
// fails here instead of letting an altered intrinsic definition publish.
// Slices are perturbed by length only; element fields such as LiteralSpec in
// Enumeration need no walk because intrinsic definitions carry no facets and
// facetSpecEmpty rejects any non-empty slice.
func TestIntrinsicComparatorDetectsEveryFieldChange(t *testing.T) {
	baseline := MissingSimpleType(QName{Namespace: 1, Local: 2}, value.AnySimpleTypeID)
	if !intrinsicSimpleTypeEqual(baseline, baseline) {
		t.Fatal("intrinsicSimpleTypeEqual(x, x) = false")
	}
	// Reflection cannot set unexported fields; this package can.
	unexported := map[string]func(*SimpleType){
		"unionContainsList": func(st *SimpleType) { st.unionContainsList = !st.unionContainsList },
		"unionHasFacets":    func(st *SimpleType) { st.unionHasFacets = !st.unionHasFacets },
	}
	visitedUnexported := map[string]bool{}
	requireDetected := func(path string, changed SimpleType) {
		t.Helper()
		if intrinsicSimpleTypeEqual(baseline, changed) || intrinsicSimpleTypeEqual(changed, baseline) {
			t.Errorf("change to %s not detected; extend intrinsicSimpleTypeEqual", path)
		}
	}
	var visit func(path string, typ reflect.Type, index []int)
	visit = func(path string, typ reflect.Type, index []int) {
		for i := range typ.NumField() {
			field := typ.Field(i)
			fieldPath := path + "." + field.Name
			fieldIndex := append(slices.Clone(index), i)
			if !field.IsExported() {
				perturb, ok := unexported[field.Name]
				if !ok || len(index) != 0 {
					t.Fatalf("unexported field %s has no perturbation; extend this guard", fieldPath)
				}
				visitedUnexported[field.Name] = true
				changed := baseline
				perturb(&changed)
				requireDetected(fieldPath, changed)
				continue
			}
			if field.Type.Kind() == reflect.Struct {
				visit(fieldPath, field.Type, fieldIndex)
				continue
			}
			changed := baseline
			perturbIntrinsicLeaf(t, fieldPath, reflect.ValueOf(&changed).Elem().FieldByIndex(fieldIndex))
			requireDetected(fieldPath, changed)
		}
	}
	visit("SimpleType", reflect.TypeFor[SimpleType](), nil)
	for name := range unexported {
		if !visitedUnexported[name] {
			t.Errorf("SimpleType.%s no longer exists; remove its perturbation", name)
		}
	}
}

func perturbIntrinsicLeaf(t *testing.T, path string, v reflect.Value) {
	t.Helper()
	// Kinds without a case fail the guard; listing them would hide new ones.
	//exhaustive:ignore
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(!v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(v.Int() + 1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(v.Uint() + 1)
	case reflect.String:
		v.SetString(v.String() + "x")
	case reflect.Slice:
		v.Set(reflect.Append(v, reflect.Zero(v.Type().Elem())))
	case reflect.Func:
		v.Set(reflect.MakeFunc(v.Type(), func([]reflect.Value) []reflect.Value {
			results := make([]reflect.Value, v.Type().NumOut())
			for i := range results {
				results[i] = reflect.Zero(v.Type().Out(i))
			}
			return results
		}))
	default:
		t.Fatalf("field %s has unsupported kind %s; extend this guard", path, v.Kind())
	}
}
