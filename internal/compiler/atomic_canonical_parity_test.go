package compiler

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
)

func TestAtomicCanonicalCompileRuntimeParity(t *testing.T) {
	cases := []struct {
		typeName string
		valid    string
		invalid  string
	}{
		{typeName: "boolean", valid: "1", invalid: "maybe"},
		{typeName: "decimal", valid: "+001.2300", invalid: "1e2"},
		{typeName: "integer", valid: "+042", invalid: "1.0"},
		{typeName: "float", valid: "01.50", invalid: "abc"},
		{typeName: "double", valid: "01.50", invalid: "abc"},
		{typeName: "duration", valid: "P1Y2M3DT4H5M6S", invalid: "P"},
		{typeName: "anyURI", valid: "https://example.com/a%20b", invalid: "%zz"},
		{typeName: "hexBinary", valid: "0a0B", invalid: "0"},
		{typeName: "base64Binary", valid: "YWI=", invalid: "%%%="},
		{typeName: "NCName", valid: "localName", invalid: "bad:name"},
	}

	for _, tc := range cases {
		t.Run(tc.typeName, func(t *testing.T) {
			rt := mustBuildRuntimeSchema(t, schemaForType(tc.typeName))
			typeID := mustRootTypeID(t, rt)

			compileRT := mustBuildRuntimeSchema(t, schemaWithDefault(tc.typeName, tc.valid))
			compileCanon := mustRootDefaultValue(t, compileRT)

			sess := validator.NewSession(rt)
			runtimeCanon, _, err := sess.ValidateTextValue(typeID, []byte(tc.valid), nil, validator.TextValueOptions{
				RequireCanonical: true,
			})
			if err != nil {
				t.Fatalf("ValidateTextValue(xs:%s, %q) error = %v", tc.typeName, tc.valid, err)
			}
			if !bytes.Equal(compileCanon, runtimeCanon) {
				t.Fatalf("compile/runtime canonical mismatch for xs:%s: compile=%q runtime=%q", tc.typeName, compileCanon, runtimeCanon)
			}

			compileErr := compileDefaultValue(t, tc.typeName, tc.invalid)
			if compileErr == nil {
				t.Fatalf("compile default xs:%s value %q unexpectedly succeeded", tc.typeName, tc.invalid)
			}

			_, _, runtimeErr := sess.ValidateTextValue(typeID, []byte(tc.invalid), nil, validator.TextValueOptions{
				RequireCanonical: true,
			})
			if runtimeErr == nil {
				t.Fatalf("ValidateTextValue(xs:%s, %q) unexpectedly succeeded", tc.typeName, tc.invalid)
			}
		})
	}
}

func mustRootTypeID(t *testing.T, rt *runtime.Schema) runtime.TypeID {
	t.Helper()
	if rt == nil || len(rt.ElementTable()) <= 1 {
		t.Fatal("runtime schema missing root element")
	}
	return rt.ElementTable()[1].Type
}

func mustRootDefaultValue(t *testing.T, rt *runtime.Schema) []byte {
	t.Helper()
	if rt == nil || len(rt.ElementTable()) <= 1 {
		t.Fatal("runtime schema missing root element")
	}
	value := valueRefBytes(rt, rt.ElementTable()[1].Default)
	if value == nil {
		t.Fatal("runtime schema missing root default value")
	}
	return value
}
