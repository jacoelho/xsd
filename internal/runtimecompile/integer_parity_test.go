package runtimecompile

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
)

func TestIntegerKindCompileRuntimeParity(t *testing.T) {
	for _, tc := range integerParityCases() {
		rt := mustBuildRuntimeSchema(t, schemaForType(tc.typeName))
		for _, vc := range tc.values {
			tc := tc

			t.Run(tc.typeName+"/"+vc.label, func(t *testing.T) {
				compileErr := compileDefaultValue(t, tc.typeName, vc.lexical)
				compileOK := compileErr == nil
				runtimeErr := validateValue(t, rt, vc.lexical)
				runtimeOK := runtimeErr == nil
				if compileOK != vc.valid {
					t.Fatalf("compile default xs:%s value %q valid=%v, err=%v", tc.typeName, vc.lexical, compileOK, compileErr)
				}
				if runtimeOK != vc.valid {
					t.Fatalf("runtime validate xs:%s value %q valid=%v, err=%v", tc.typeName, vc.lexical, runtimeOK, runtimeErr)
				}
			})
		}
	}
}

type integerParityCase struct {
	typeName string
	values   []valueCase
}

type valueCase struct {
	label   string
	lexical string
	valid   bool
}

func integerParityCases() []integerParityCase {
	one := num.Int{Sign: 1, Digits: []byte("1")}
	negOne := num.Int{Sign: -1, Digits: []byte("1")}

	render := func(v num.Int) string {
		return string(v.RenderCanonical(nil))
	}
	inc := func(v num.Int) num.Int {
		return num.Add(v, one)
	}
	dec := func(v num.Int) num.Int {
		return num.Add(v, negOne)
	}
	rangeCases := func(minValue, maxValue num.Int) []valueCase {
		return []valueCase{
			{label: "min", lexical: render(minValue), valid: true},
			{label: "max", lexical: render(maxValue), valid: true},
			{label: "min-minus-one", lexical: render(dec(minValue)), valid: false},
			{label: "max-plus-one", lexical: render(inc(maxValue)), valid: false},
		}
	}
	return []integerParityCase{
		{
			typeName: "integer",
			values: []valueCase{
				{label: "neg-one", lexical: "-1", valid: true},
				{label: "zero", lexical: "0", valid: true},
				{label: "one", lexical: "1", valid: true},
			},
		},
		{typeName: "long", values: rangeCases(num.MinInt64, num.MaxInt64)},
		{typeName: "int", values: rangeCases(num.MinInt32, num.MaxInt32)},
		{typeName: "short", values: rangeCases(num.MinInt16, num.MaxInt16)},
		{typeName: "byte", values: rangeCases(num.MinInt8, num.MaxInt8)},
		{
			typeName: "nonNegativeInteger",
			values: []valueCase{
				{label: "neg-one", lexical: "-1", valid: false},
				{label: "zero", lexical: "0", valid: true},
				{label: "one", lexical: "1", valid: true},
			},
		},
		{
			typeName: "positiveInteger",
			values: []valueCase{
				{label: "neg-one", lexical: "-1", valid: false},
				{label: "zero", lexical: "0", valid: false},
				{label: "one", lexical: "1", valid: true},
			},
		},
		{
			typeName: "nonPositiveInteger",
			values: []valueCase{
				{label: "neg-one", lexical: "-1", valid: true},
				{label: "zero", lexical: "0", valid: true},
				{label: "one", lexical: "1", valid: false},
			},
		},
		{
			typeName: "negativeInteger",
			values: []valueCase{
				{label: "neg-one", lexical: "-1", valid: true},
				{label: "zero", lexical: "0", valid: false},
				{label: "one", lexical: "1", valid: false},
			},
		},
		{typeName: "unsignedLong", values: rangeCases(num.IntZero, num.MaxUint64)},
		{typeName: "unsignedInt", values: rangeCases(num.IntZero, num.MaxUint32)},
		{typeName: "unsignedShort", values: rangeCases(num.IntZero, num.MaxUint16)},
		{typeName: "unsignedByte", values: rangeCases(num.IntZero, num.MaxUint8)},
	}
}

func schemaForType(typeName string) string {
	return fmt.Sprintf(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:%s"/>
</xs:schema>`, typeName)
}

func schemaWithDefault(typeName, value string) string {
	return fmt.Sprintf(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:%s" default="%s"/>
</xs:schema>`, typeName, value)
}

func compileDefaultValue(t *testing.T, typeName, value string) error {
	t.Helper()
	schemaXML := schemaWithDefault(typeName, value)
	parsed, err := resolveSchema(schemaXML)
	if err != nil {
		return err
	}
	_, err = buildSchemaForTest(parsed, BuildConfig{})
	return err
}

func validateValue(t *testing.T, rt *runtime.Schema, value string) error {
	t.Helper()
	sess := validator.NewSession(rt)
	return sess.Validate(strings.NewReader(fmt.Sprintf("<root>%s</root>", value)))
}
