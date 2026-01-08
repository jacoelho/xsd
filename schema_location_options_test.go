package xsd_test

import (
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
	xsderrors "github.com/jacoelho/xsd/errors"
)

type nonSeekableReader struct {
	r io.Reader
}

func (n nonSeekableReader) Read(p []byte) (int, error) {
	return n.r.Read(p)
}

func TestValidateRootOnlySchemaLocationNonSeekable(t *testing.T) {
	baseSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:base"
           elementFormDefault="qualified">
  <xs:element name="baseRoot" type="xs:string"/>
</xs:schema>`

	hintSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:hint"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"base.xsd": {Data: []byte(baseSchema)},
		"hint.xsd": {Data: []byte(hintSchema)},
	}

	schema, err := xsd.Load(fs, "base.xsd")
	if err != nil {
		t.Fatalf("Load schema: %v", err)
	}

	doc := `<?xml version="1.0"?>
<root xmlns="urn:hint"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="urn:hint hint.xsd">value</root>`

	reader := nonSeekableReader{r: strings.NewReader(doc)}
	if err := schema.Validate(reader); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateDocumentSchemaLocationNonSeekableError(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"base.xsd": {Data: []byte(schemaXML)},
	}

	schema, err := xsd.Load(fs, "base.xsd")
	if err != nil {
		t.Fatalf("Load schema: %v", err)
	}

	doc := `<?xml version="1.0"?>
<root xmlns="urn:test"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="urn:test base.xsd">value</root>`

	reader := nonSeekableReader{r: strings.NewReader(doc)}
	err = schema.ValidateWithOptions(reader, xsd.ValidateOptions{
		SchemaLocationPolicy: xsd.SchemaLocationDocument,
	})
	if err == nil {
		t.Fatal("expected error for non-seekable reader with schemaLocation hints")
	}
	list, ok := xsderrors.AsValidations(err)
	if !ok {
		t.Fatalf("expected validation error, got %v", err)
	}
	for _, v := range list {
		if v.Code == string(xsderrors.ErrSchemaLocationHint) {
			return
		}
	}
	t.Fatalf("expected ErrSchemaLocationHint, got %v", err)
}
