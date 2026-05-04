package xsd

import "testing"

func TestQNameValueUsesInstanceNamespacesWithoutInterning(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="q" type="xs:QName"/>
</xs:schema>`)
	localsBefore := len(engine.rt.Names.locals)
	mustValidate(t, engine, `<q xmlns:p="urn:dynamic">p:notInSchema</q>`)
	if got := len(engine.rt.Names.locals); got != localsBefore {
		t.Fatalf("QName validation interned instance local names: before=%d after=%d", localsBefore, got)
	}
	mustNotValidate(t, engine, `<q>p:notBound</q>`, ErrValidationFacet)
	mustNotValidate(t, engine, `<q>bad:name:shape</q>`, ErrValidationFacet)
}
