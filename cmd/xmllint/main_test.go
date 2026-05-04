package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgsAcceptsXmllintFlags(t *testing.T) {
	cfg, err := parseArgs([]string{"--noout", "--huge", "--schema", "schema.xsd", "doc.xml"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if !cfg.noout || !cfg.huge || cfg.schema != "schema.xsd" || cfg.doc != "doc.xml" {
		t.Fatalf("parseArgs() = %+v", cfg)
	}
}

func TestRunResolvesLocalSchemaRefsThroughXSDPackage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "root.xsd"), `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:r="urn:root"
           xmlns:xml="http://www.w3.org/XML/1998/namespace"
           targetNamespace="urn:root"
           elementFormDefault="qualified">
  <xs:include schemaLocation="included.xsd"/>
  <xs:import namespace="http://www.w3.org/XML/1998/namespace" schemaLocation="xml.xsd"/>
  <xs:element name="root" type="r:Included"/>
</xs:schema>`)
	writeFile(t, filepath.Join(dir, "included.xsd"), `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:xml="http://www.w3.org/XML/1998/namespace"
           elementFormDefault="qualified">
  <xs:complexType name="Included">
    <xs:sequence>
      <xs:element name="v" type="xs:int"/>
    </xs:sequence>
    <xs:attribute ref="xml:lang"/>
  </xs:complexType>
</xs:schema>`)
	writeFile(t, filepath.Join(dir, "xml.xsd"), `<not-a-schema/>`)
	writeFile(t, filepath.Join(dir, "doc.xml"), `<root xmlns="urn:root" xml:lang="en"><v>7</v></root>`)

	var stderr bytes.Buffer
	if code := run([]string{"--noout", "--huge", "--schema", filepath.Join(dir, "root.xsd"), filepath.Join(dir, "doc.xml")}, &stderr); code != 0 {
		t.Fatalf("run() = %d, stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "doc.xml validates") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
