package validator

import (
	"errors"
	"io"
	"strings"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestValidateMaxDepth(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" type="xs:string"/>
</xs:schema>`
	doc := `<a><b><c><d><e><f/></e></d></c></b></a>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	sess := NewSession(rt, xmltext.MaxDepth(4))
	err := sess.Validate(strings.NewReader(doc))
	if err == nil {
		t.Fatalf("expected MaxDepth error")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrXMLParse) {
		t.Fatalf("expected ErrXMLParse, got %+v", list)
	}
}

func TestValidateMaxAttrs(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	doc := `<root a="1" b="2" c="3" d="4"/>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	sess := NewSession(rt, xmltext.MaxAttrs(2))
	err := sess.Validate(strings.NewReader(doc))
	if err == nil {
		t.Fatalf("expected MaxAttrs error")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrXMLParse) {
		t.Fatalf("expected ErrXMLParse, got %+v", list)
	}
}

func TestValidateMaxTokenSize(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	doc := `<root>abcdefghijklmnopqrstuvwxyz</root>`

	rt := mustBuildRuntimeSchema(t, schemaXML)
	sess := NewSession(rt, xmltext.MaxTokenSize(8))
	err := sess.Validate(strings.NewReader(doc))
	if err == nil {
		t.Fatalf("expected MaxTokenSize error")
	}
	list := mustValidationList(t, err)
	if !hasValidationCode(list, xsderrors.ErrXMLParse) {
		t.Fatalf("expected ErrXMLParse, got %+v", list)
	}
}

func TestValidatePassesQNameInternLimit(t *testing.T) {
	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		t.Fatalf("build runtime schema: %v", err)
	}
	sess := NewSession(schema, xmltext.MaxQNameInternEntries(3))

	orig := sess.readerFactory
	sess.readerFactory = func(_ io.Reader, opts ...xmlstream.Option) (*xmlstream.Reader, error) {
		merged := xmlstream.JoinOptions(opts...)
		limit, ok := merged.QNameInternEntries()
		if !ok || limit != 3 {
			t.Fatalf("QNameInternEntries = %d, ok=%v, want 3, true", limit, ok)
		}
		return nil, errors.New("stop")
	}
	t.Cleanup(func() {
		sess.readerFactory = orig
	})

	_ = sess.Validate(strings.NewReader("<root/>"))
}
