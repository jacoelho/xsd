package xsd_test

import (
	"testing"

	"github.com/jacoelho/xsd"
)

func TestSubstitutionMemberWithUnavailableHeadDefersValueConstraint(t *testing.T) {
	t.Parallel()

	for _, constraint := range []struct {
		name string
		attr string
	}{
		{name: "default", attr: `default="not-a-value-for-the-unavailable-type"`},
		{name: "fixed", attr: `fixed="not-a-value-for-the-unavailable-type"`},
	} {
		t.Run(constraint.name, func(t *testing.T) {
			t.Parallel()
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:m="urn:missing">
  <xs:import namespace="urn:missing"/>
  <xs:element name="head" type="m:T"/>
  <xs:element name="member" substitutionGroup="head" ` + constraint.attr + `/>
</xs:schema>`
			if engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema))); err != nil || engine == nil {
				t.Fatalf("Compile() = (%v, %v), want an engine with deferred unavailable-type constraint", engine, err)
			}
		})
	}
}
