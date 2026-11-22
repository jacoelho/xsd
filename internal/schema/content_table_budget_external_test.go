package schema_test

import (
	"fmt"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
)

// The schemas of each pair declare the same components and particles and
// differ only in the compiled table of one content model, so the difference
// in their least MaxSchemaDependencySteps is the difference in table units:
// rows, edge references and index entries.
func TestCompiledContentTablesChargeEveryRowEdgeReferenceAndIndexEntry(t *testing.T) {
	t.Parallel()
	elements := func(count int, attrs string) string {
		var b strings.Builder
		for i := range count {
			fmt.Fprintf(&b, `<xs:element name="e%d"%s/>`, i, attrs)
		}
		return b.String()
	}
	for _, test := range []struct {
		name           string
		larger, base   string
		wantDifference int
	}{
		{
			// Ten optional elements give row i the edges of elements i to 9:
			// 55 edge references against 10, and the rows of 10, 9 and 8 edges
			// are indexed, 27 entries.
			name:           "optional against required sequence",
			larger:         `<xs:sequence>` + elements(10, ` minOccurs="0"`) + `</xs:sequence>`,
			base:           `<xs:sequence>` + elements(10, "") + `</xs:sequence>`,
			wantDifference: 55 - 10 + 27,
		},
		{
			// Every element of a repeated choice of eight elements has the
			// same follow set, so its automaton has two rows, the start and
			// the state after any element, each taking all eight edges and
			// indexed. The unrepeated choice has nine rows and eight edge
			// references, all on its indexed start row.
			name:           "repeated against single choice",
			larger:         `<xs:choice maxOccurs="unbounded">` + elements(8, "") + `</xs:choice>`,
			base:           `<xs:choice>` + elements(8, "") + `</xs:choice>`,
			wantDifference: (2 + 16 + 16) - (9 + 8 + 8),
		},
		{
			// An all group of eight elements holds eight terms and indexes
			// them; the sequence holds nine rows and eight edge references.
			name:           "all group against sequence",
			larger:         `<xs:all>` + elements(8, "") + `</xs:all>`,
			base:           `<xs:sequence>` + elements(8, "") + `</xs:sequence>`,
			wantDifference: 8 + 8 - 9 - 8,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			limit := func(steps int) xsdSchema.Options {
				return xsdSchema.Options{MaxSchemaDependencySteps: steps}
			}
			schema := func(content string) string {
				return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:complexType name="T">` + content + `</xs:complexType></xs:schema>`
			}
			larger := minimumCompileLimit(t, schema(test.larger), limit)
			base := minimumCompileLimit(t, schema(test.base), limit)
			if got := larger - base; got != test.wantDifference {
				t.Fatalf("least dependency limits %d and %d differ by %d, want %d", larger, base, got, test.wantDifference)
			}
		})
	}
}
