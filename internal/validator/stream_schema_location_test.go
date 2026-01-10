package validator

import (
	"io"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/loader"
)

type nonSeekableReader struct {
	r io.Reader
}

func (n nonSeekableReader) Read(p []byte) (int, error) {
	return n.r.Read(p)
}

func TestStreamSchemaLocationSeekableMerge(t *testing.T) {
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

	l := loader.NewLoader(loader.Config{FS: fs})
	compiled, err := l.LoadCompiled("base.xsd")
	if err != nil {
		t.Fatalf("load compiled schema: %v", err)
	}

	v := New(compiled, WithSchemaLocationLoader(l))

	doc := `<?xml version="1.0"?>
<root xmlns="urn:hint"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="urn:hint hint.xsd">value</root>`

	violations, err := v.ValidateStream(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}
}

func TestStreamSchemaLocationNonSeekableRootOnly(t *testing.T) {
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

	l := loader.NewLoader(loader.Config{FS: fs})
	compiled, err := l.LoadCompiled("base.xsd")
	if err != nil {
		t.Fatalf("load compiled schema: %v", err)
	}

	v := New(compiled, WithSchemaLocationLoader(l))

	doc := `<?xml version="1.0"?>
<root xmlns="urn:hint"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="urn:hint hint.xsd">value</root>`

	reader := nonSeekableReader{r: strings.NewReader(doc)}
	violations, err := v.ValidateStream(reader)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}
}

func TestStreamSchemaLocationNonSeekableDocumentError(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"base.xsd": {Data: []byte(schemaXML)},
	}

	l := loader.NewLoader(loader.Config{FS: fs})
	compiled, err := l.LoadCompiled("base.xsd")
	if err != nil {
		t.Fatalf("load compiled schema: %v", err)
	}

	v := New(compiled, WithSchemaLocationLoader(l))

	doc := `<?xml version="1.0"?>
<root xmlns="urn:test"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="urn:test base.xsd">value</root>`

	reader := nonSeekableReader{r: strings.NewReader(doc)}
	violations, err := v.ValidateStreamWithOptions(reader, StreamOptions{
		SchemaLocationPolicy: SchemaLocationDocument,
	})
	if err == nil {
		t.Fatal("expected error for non-seekable reader with schemaLocation hints")
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}
	list, ok := errors.AsValidations(err)
	if !ok {
		t.Fatalf("expected validation error, got %v", err)
	}
	if !hasValidationCode(list, errors.ErrSchemaLocationHint) {
		t.Fatalf("expected ErrSchemaLocationHint, got %v", err)
	}
}

func TestStreamSchemaLocationIgnoreNonSeekable(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"base.xsd": {Data: []byte(schemaXML)},
	}

	l := loader.NewLoader(loader.Config{FS: fs})
	compiled, err := l.LoadCompiled("base.xsd")
	if err != nil {
		t.Fatalf("load compiled schema: %v", err)
	}

	v := New(compiled, WithSchemaLocationLoader(l))

	doc := `<?xml version="1.0"?>
<root xmlns="urn:test"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="urn:test base.xsd">value</root>`

	reader := nonSeekableReader{r: strings.NewReader(doc)}
	violations, err := v.ValidateStreamWithOptions(reader, StreamOptions{
		SchemaLocationPolicy: SchemaLocationIgnore,
	})
	if err != nil {
		t.Fatalf("ValidateStreamWithOptions() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}
}
