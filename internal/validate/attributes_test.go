package validate

import (
	"encoding/xml"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeSeenTracksBitsetAndSliceSlots(t *testing.T) {
	t.Parallel()

	var scratch []bool
	bitset := newAttributeSeenWithScratch(2, &scratch)
	if scratch != nil {
		t.Fatalf("inline attribute presence allocated scratch: %v", scratch)
	}
	if !bitset.mark(1) {
		t.Fatal("first bitset mark failed")
	}
	if bitset.mark(1) {
		t.Fatal("duplicate bitset mark succeeded")
	}
	if !bitset.has(1) || bitset.has(0) {
		t.Fatalf("bitset has states = slot0:%v slot1:%v", bitset.has(0), bitset.has(1))
	}

	const slot = 65
	slice := newAttributeSeenWithScratch(70, &scratch)
	if !slice.mark(slot) {
		t.Fatal("first slice mark failed")
	}
	if slice.mark(slot) {
		t.Fatal("duplicate slice mark succeeded")
	}
	if !slice.has(slot) || slice.has(1) {
		t.Fatalf("slice has states = slot1:%v slot%d:%v", slice.has(1), slot, slice.has(slot))
	}
}

func TestAttributeSeenScratchIsClearedBeforeReuse(t *testing.T) {
	var scratch []bool
	seen := newAttributeSeenWithScratch(70, &scratch)
	if !seen.mark(69) {
		t.Fatal("first mark failed")
	}
	retained := &scratch[0]

	seen = newAttributeSeenWithScratch(70, &scratch)
	if &scratch[0] != retained {
		t.Fatal("attribute presence scratch was not reused")
	}
	if !seen.mark(69) {
		t.Fatal("reused attribute presence scratch retained a mark")
	}

	retained = &scratch[0]
	seen = newAttributeSeenWithScratch(maxRetainedSliceCap+1, &scratch)
	if &scratch[0] != retained {
		t.Fatal("oversized attribute presence replaced retained scratch")
	}
	if !seen.mark(maxRetainedSliceCap) || !seen.has(maxRetainedSliceCap) {
		t.Fatal("oversized attribute presence did not track final slot")
	}
}

func TestMatchAttributeWildcard(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:test" targetNamespace="urn:test">
  <xs:attribute name="known" type="xs:string"/>
  <xs:element name="strict"><xs:complexType><xs:anyAttribute processContents="strict"/></xs:complexType></xs:element>
  <xs:element name="lax"><xs:complexType><xs:anyAttribute processContents="lax"/></xs:complexType></xs:element>
  <xs:element name="skip"><xs:complexType><xs:anyAttribute processContents="skip"/></xs:complexType></xs:element>
  <xs:element name="local"><xs:complexType><xs:anyAttribute namespace="##local" processContents="strict"/></xs:complexType></xs:element>
</xs:schema>`)
	wildcard := func(local string) xsdSchema.WildcardID {
		q, ok := rt.LookupQName("urn:test", local)
		if !ok {
			t.Fatalf("missing element name %s", local)
		}
		_, info, ok := rt.RootElement(xsdSchema.RuntimeName{Known: true, Name: q, NS: "urn:test", Local: local})
		if !ok {
			t.Fatalf("missing element %s", local)
		}
		uses, has, valid := rt.AttributeUseSetForType(info.Type)
		if !has || !valid {
			t.Fatalf("attribute uses for %s = has %v, valid %v", local, has, valid)
		}
		return uses.Wildcard()
	}
	knownName, ok := rt.LookupQName("urn:test", "known")
	if !ok {
		t.Fatal("missing global attribute name")
	}
	knownID, found, valid := rt.GlobalAttribute(knownName)
	if !found || !valid {
		t.Fatalf("global attribute = found %v, valid %v", found, valid)
	}
	missingName, ok := rt.LookupQName("urn:test", "strict")
	if !ok {
		t.Fatal("missing known non-attribute name")
	}
	tests := []struct {
		name     string
		wildcard xsdSchema.WildcardID
		rn       xsdSchema.RuntimeName
		want     attributeWildcardMatch
		valid    bool
	}{
		{
			name:     "no wildcard",
			wildcard: xsdSchema.NoWildcard,
			valid:    true,
		},
		{
			name:     "invalid wildcard",
			wildcard: xsdSchema.WildcardID(999),
		},
		{
			name:     "namespace not allowed",
			wildcard: wildcard("local"),
			rn:       xsdSchema.RuntimeName{NS: "urn:not-local", Local: "x"},
			valid:    true,
		},
		{
			name:     "skip",
			wildcard: wildcard("skip"),
			rn:       xsdSchema.RuntimeName{NS: "urn:any", Local: "x"},
			want:     attributeWildcardMatch{disposition: attributeWildcardSkip},
			valid:    true,
		},
		{
			name:     "lax missing",
			wildcard: wildcard("lax"),
			rn:       xsdSchema.RuntimeName{NS: "urn:any", Local: "x"},
			want:     attributeWildcardMatch{disposition: attributeWildcardLaxMissing},
			valid:    true,
		},
		{
			name:     "lax known missing global",
			wildcard: wildcard("lax"),
			rn:       xsdSchema.RuntimeName{Known: true, Name: missingName, NS: "urn:test", Local: "strict"},
			want:     attributeWildcardMatch{disposition: attributeWildcardLaxMissing},
			valid:    true,
		},
		{
			name:     "strict missing",
			wildcard: wildcard("strict"),
			rn:       xsdSchema.RuntimeName{NS: "urn:any", Local: "x"},
			want:     attributeWildcardMatch{disposition: attributeWildcardStrictMissing},
			valid:    true,
		},
		{
			name:     "known global",
			wildcard: wildcard("strict"),
			rn:       xsdSchema.RuntimeName{Known: true, Name: knownName, NS: "urn:test", Local: "known"},
			want:     attributeWildcardMatch{attribute: knownID, disposition: attributeWildcardDeclared},
			valid:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, valid := matchAttributeWildcard(rt, tc.wildcard, tc.rn)
			if valid != tc.valid || got != tc.want {
				t.Fatalf("matchAttributeWildcard() = %+v/%v, want %+v/%v", got, valid, tc.want, tc.valid)
			}
		})
	}
}

func TestRelativeNamespaceAttributeIsNotReservedOrDropped(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:anyAttribute processContents="skip"/></xs:complexType>
    <xs:key name="k"><xs:selector xpath="."/><xs:field xpath="@*"/></xs:key>
  </xs:element>
</xs:schema>`
	rt := compileRuntimeForTest(t, schema)
	session, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := &session.session
	rootName, ok := rt.LookupQName("", "root")
	if !ok {
		t.Fatal("compiled runtime is missing root")
	}
	rootID, _, ok := rt.RootElement(xsdSchema.RuntimeName{Known: true, Name: rootName, Local: "root"})
	if !ok {
		t.Fatal("compiled runtime is missing root element metadata")
	}
	rootRuntimeName := xsdSchema.RuntimeName{Known: true, Name: rootName, Local: "root"}
	s.doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "root"}}, frame{})
	if startErr := s.doc.identity.startElement(identityElementStart{
		Context: s.startContext(1, 1),
		Name:    rootRuntimeName,
		Element: rootID,
		Mode:    elementAssessed,
	}); startErr != nil {
		t.Fatal(startErr)
	}

	declaration := xmlstream.Attr{Name: xml.Name{Space: vocab.XMLNSNamespaceURI, Local: "p"}, Value: "xmlns"}
	ordinary := xmlstream.Attr{Name: xml.Name{Space: vocab.XMLNSPrefix, Local: "value"}, Value: "v"}
	if handled, reservedErr := s.validateReservedAttribute(&declaration, 1, 1); reservedErr != nil || !handled {
		t.Fatalf("expanded namespace declaration = handled %v, error %v; want handled without error", handled, reservedErr)
	}
	if handled, reservedErr := s.validateReservedAttribute(&ordinary, 1, 1); reservedErr != nil || handled {
		t.Fatalf("ordinary relative-namespace attribute = handled %v, error %v; want ordinary without error", handled, reservedErr)
	}
	if identityErr := s.rejectUnassessedIdentityAttributes([]xmlstream.Attr{declaration, ordinary}, 1, 1, identityMissingSimpleValue); identityErr != nil {
		t.Fatalf("rejectUnassessedIdentityAttributes() error = %v", identityErr)
	}
	if got := s.doc.identity.fieldValues[0].state; got != identityFieldInvalid {
		t.Fatalf("ordinary relative-namespace identity field state = %v, want invalid", got)
	}

	// The same boundary must be visible through ordinary document validation.
	integrationSession, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := integrationSession.Validate(strings.NewReader(`<root xmlns:p="xmlns" p:value="v"/>`)); err == nil {
		t.Fatal("Validate() accepted an undeclared ordinary relative-namespace attribute")
	} else {
		expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
	}
}
