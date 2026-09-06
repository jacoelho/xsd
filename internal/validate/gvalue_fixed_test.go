package validate

import (
	"strings"
	"testing"
)

func TestFixedElementGMonthDayUsesValueSpaceEquality(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:gMonthDay" fixed="--01-02+14:00"/>
</xs:schema>`)

	for _, test := range []struct {
		name string
		doc  string
		want bool
	}{
		{name: "equivalent timezone spelling", doc: `<root>--01-01-10:00</root>`, want: true},
		{name: "different instant", doc: `<root>--01-01-09:00</root>`, want: false},
		{name: "timezone presence differs", doc: `<root>--01-01</root>`, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, err := newSessionForTest(rt, Options{})
			if err != nil {
				t.Fatal(err)
			}
			err = s.Validate(strings.NewReader(test.doc))
			if (err == nil) != test.want {
				t.Fatalf("Validate(%s) error = %v, want valid=%v", test.doc, err, test.want)
			}
		})
	}
}
