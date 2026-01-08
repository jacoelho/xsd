package xsd_test

import (
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
)

func TestSchemaValidateConcurrent(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:int" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<?xml version="1.0"?>
<root xmlns="urn:test">
  <item>1</item>
  <item>2</item>
  <item>3</item>
</root>`

	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}
	schema, err := xsd.Load(fsys, "schema.xsd")
	if err != nil {
		t.Fatalf("Load schema: %v", err)
	}

	const goroutines = 8
	const iterations = 25

	errCh := make(chan error, goroutines*iterations)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if err := schema.Validate(strings.NewReader(docXML)); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent Validate error: %v", err)
	}
}
