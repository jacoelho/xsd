package schema

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
)

// zeroCountInlineTypeOrderSchema holds a zero-count element whose inline union
// names the missing type t:missing in complex type zeroCount, and a live
// element whose inline list names it too in complex type live.
// compileGlobalTypes compiles complex types in name order: when zeroCount
// sorts first, the inline union creates the missing-type placeholder and is
// kept; otherwise it finds the placeholder and is dropped. Both orders hold the
// same components.
func zeroCountInlineTypeOrderSchema(zeroCount, live string) string {
	return fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t">
  <xs:complexType name="%s"><xs:sequence>
    <xs:element name="z" minOccurs="0" maxOccurs="0"><xs:simpleType><xs:union memberTypes="t:missing xs:int xs:date"><xs:simpleType><xs:list itemType="xs:int"/></xs:simpleType></xs:union></xs:simpleType></xs:element>
  </xs:sequence></xs:complexType>
  <xs:complexType name="%s"><xs:sequence>
    <xs:element name="b"><xs:simpleType><xs:list itemType="t:missing"/></xs:simpleType></xs:element>
  </xs:sequence></xs:complexType>
</xs:schema>`, zeroCount, live)
}

// minimalPassingLimit returns the least value of a limit at which text
// compiles, searching up to 1<<20.
func minimalPassingLimit(t *testing.T, text string, set func(*Options, int)) int {
	t.Helper()
	compiles := func(limit int) bool {
		var opts Options
		set(&opts, limit)
		_, err := Compile(opts, []source.Source{source.Bytes("schema.xsd", []byte(text))})
		return err == nil
	}
	low, high := 1, 1<<20
	if !compiles(high) {
		t.Fatalf("schema does not compile at limit %d", high)
	}
	for low < high {
		mid := low + (high-low)/2
		if compiles(mid) {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return low
}

func TestDroppingAZeroCountInlineTypeChangesNoLimitBoundary(t *testing.T) {
	t.Parallel()

	kept, dropped := zeroCountInlineTypeOrderSchema("A", "B"), zeroCountInlineTypeOrderSchema("B", "A")
	if k, d := len(compiledTestBuild(t, kept).SimpleTypes), len(compiledTestBuild(t, dropped).SimpleTypes); k != d+2 {
		t.Fatalf("simple types kept = %d, dropped = %d, want the dropped order to lack the inline union and its list member", k, d)
	}
	for _, limit := range []struct {
		name string
		set  func(*Options, int)
	}{
		{name: "MaxSchemaDependencySteps", set: func(o *Options, v int) { o.MaxSchemaDependencySteps = v }},
		{name: "MaxContentModelAnalysisSteps", set: func(o *Options, v int) { o.MaxContentModelAnalysisSteps = v }},
		{name: "MaxSchemaInstantiatedNodes", set: func(o *Options, v int) { o.MaxSchemaInstantiatedNodes = v }},
		{name: "MaxSchemaNames", set: func(o *Options, v int) { o.MaxSchemaNames = v }},
		{name: "MaxSimpleUnionMemberEntries", set: func(o *Options, v int) { o.MaxSimpleUnionMemberEntries = v }},
	} {
		t.Run(limit.name, func(t *testing.T) {
			t.Parallel()
			if k, d := minimalPassingLimit(t, kept, limit.set), minimalPassingLimit(t, dropped, limit.set); k != d {
				t.Fatalf("least passing %s: kept inline type %d, dropped %d, want equal", limit.name, k, d)
			}
		})
	}
}
