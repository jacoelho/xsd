package schema

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
)

const unheldUnionType = `<xs:simpleType><xs:union memberTypes="t:missing xs:int xs:date"><xs:simpleType><xs:list itemType="xs:int"/></xs:simpleType></xs:union></xs:simpleType>`

// unheldInlineTypeOrderSchema holds unheld, a source item that maps to no
// component, in complex type unheldOwner, and a live element whose inline list
// names the missing type t:missing in complex type live. compileGlobalTypes
// compiles complex types in name order, so when unheldOwner sorts first an
// inline type of unheld that names t:missing creates the missing-type
// placeholder, and otherwise it finds the placeholder. Both orders hold the
// same components.
func unheldInlineTypeOrderSchema(unheld, unheldOwner, live string) string {
	return fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t">
  <xs:complexType name="%s">%s</xs:complexType>
  <xs:complexType name="%s"><xs:sequence>
    <xs:element name="b"><xs:simpleType><xs:list itemType="t:missing"/></xs:simpleType></xs:element>
  </xs:sequence></xs:complexType>
</xs:schema>`, unheldOwner, unheld, live)
}

// minimalPassingLimit returns the least value of a limit at which text
// compiles, searching up to 1<<20.
func minimalPassingLimit(t *testing.T, text string, set func(*Options, int)) int {
	t.Helper()
	compiles := func(limit int) bool {
		var opts Options
		set(&opts, limit)
		_, err := compileSources(opts, []source.Source{source.Bytes("schema.xsd", []byte(text))})
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

func TestUnheldInlineTypeDropIsIndependentOfCompileOrder(t *testing.T) {
	t.Parallel()

	for _, unheld := range []struct {
		name        string
		text        string
		withoutType string
	}{
		{name: "zero-count element", text: `<xs:sequence><xs:element name="z" minOccurs="0" maxOccurs="0">` + unheldUnionType + `</xs:element></xs:sequence>`, withoutType: `<xs:sequence><xs:element name="z" minOccurs="0" maxOccurs="0"/></xs:sequence>`},
		{name: "prohibited attribute", text: `<xs:attribute name="p" use="prohibited">` + unheldUnionType + `</xs:attribute>`, withoutType: `<xs:attribute name="p" use="prohibited"/>`},
	} {
		t.Run(unheld.name, func(t *testing.T) {
			t.Parallel()
			first, second := unheldInlineTypeOrderSchema(unheld.text, "A", "B"), unheldInlineTypeOrderSchema(unheld.text, "B", "A")
			want := len(compiledTestBuild(t, unheldInlineTypeOrderSchema(unheld.withoutType, "A", "B")).SimpleTypes)
			for order, text := range map[string]string{"unheld first": first, "unheld second": second} {
				if got := len(compiledTestBuild(t, text).SimpleTypes); got != want {
					t.Errorf("%s: simple types = %d, want %d as without the inline type", order, got, want)
				}
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
				if f, s := minimalPassingLimit(t, first, limit.set), minimalPassingLimit(t, second, limit.set); f != s {
					t.Errorf("least passing %s: unheld first %d, unheld second %d, want equal", limit.name, f, s)
				}
			}
		})
	}
}
