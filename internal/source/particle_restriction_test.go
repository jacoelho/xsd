package source

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestParticleRestriction_BlockSupersetInModelGroup(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com" xmlns:x="http://example.com">
		<xs:complexType name="Base">
			<xs:sequence>
				<xs:choice>
					<xs:element name="c1" block="#all"/>
					<xs:element name="c2"/>
				</xs:choice>
			</xs:sequence>
		</xs:complexType>
		<xs:complexType name="Derived">
			<xs:complexContent>
				<xs:restriction base="x:Base">
					<xs:sequence>
						<xs:element name="c1" block="extension"/>
					</xs:sequence>
				</xs:restriction>
			</xs:complexContent>
		</xs:complexType>
	</xs:schema>`

	fsys := fstest.MapFS{
		"test.xsd": &fstest.MapFile{Data: []byte(schema)},
	}
	loader := NewLoader(Config{FS: fsys})
	_, err := loadAndPrepare(t, loader, "test.xsd")
	if err == nil {
		t.Fatal("Expected schema validation error but got none")
	}
	if !strings.Contains(err.Error(), "block constraint") {
		t.Fatalf("Expected block constraint error, got: %v", err)
	}
}

func TestParticleRestriction_TypeMismatchInModelGroup(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com" xmlns:x="http://example.com">
		<xs:complexType name="MixedType" mixed="true">
			<xs:sequence>
				<xs:element name="child"/>
			</xs:sequence>
		</xs:complexType>
		<xs:complexType name="ElementOnlyType" mixed="false">
			<xs:sequence>
				<xs:element name="child"/>
			</xs:sequence>
		</xs:complexType>
		<xs:complexType name="Base">
			<xs:sequence>
				<xs:choice>
					<xs:element name="c1" type="x:ElementOnlyType"/>
					<xs:element name="c2"/>
				</xs:choice>
			</xs:sequence>
		</xs:complexType>
		<xs:complexType name="Derived">
			<xs:complexContent>
				<xs:restriction base="x:Base">
					<xs:sequence>
						<xs:element name="c1" type="x:MixedType"/>
					</xs:sequence>
				</xs:restriction>
			</xs:complexContent>
		</xs:complexType>
	</xs:schema>`

	fsys := fstest.MapFS{
		"test.xsd": &fstest.MapFile{Data: []byte(schema)},
	}
	loader := NewLoader(Config{FS: fsys})
	_, err := loadAndPrepare(t, loader, "test.xsd")
	if err == nil {
		t.Fatal("Expected schema validation error but got none")
	}
	if !strings.Contains(err.Error(), "type") {
		t.Fatalf("Expected type restriction error, got: %v", err)
	}
}

func TestParticleRestriction_ChoiceOccurrenceRange(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com" xmlns:x="http://example.com">
		<xs:complexType name="Base">
			<xs:sequence>
				<xs:choice>
					<xs:element name="a"/>
					<xs:element name="b"/>
				</xs:choice>
			</xs:sequence>
		</xs:complexType>
		<xs:complexType name="Derived">
			<xs:complexContent>
				<xs:restriction base="x:Base">
					<xs:sequence>
						<xs:choice minOccurs="0">
							<xs:element name="a"/>
						</xs:choice>
					</xs:sequence>
				</xs:restriction>
			</xs:complexContent>
		</xs:complexType>
	</xs:schema>`

	fsys := fstest.MapFS{
		"test.xsd": &fstest.MapFile{Data: []byte(schema)},
	}
	loader := NewLoader(Config{FS: fsys})
	_, err := loadAndPrepare(t, loader, "test.xsd")
	if err == nil {
		t.Fatal("Expected schema validation error but got none")
	}
	if !strings.Contains(err.Error(), "minOccurs") {
		t.Fatalf("Expected occurrence constraint error, got: %v", err)
	}
}

func TestParticleRestriction_ChoiceWildcardAllowsMultiple(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com" xmlns:x="http://example.com" elementFormDefault="qualified">
		<xs:complexType name="Base">
			<xs:sequence>
				<xs:choice>
					<xs:any namespace="##any" processContents="lax"/>
				</xs:choice>
			</xs:sequence>
		</xs:complexType>
		<xs:complexType name="Derived">
			<xs:complexContent>
				<xs:restriction base="x:Base">
					<xs:sequence>
						<xs:choice>
							<xs:element name="a"/>
							<xs:element name="b"/>
						</xs:choice>
					</xs:sequence>
				</xs:restriction>
			</xs:complexContent>
		</xs:complexType>
	</xs:schema>`

	fsys := fstest.MapFS{
		"test.xsd": &fstest.MapFile{Data: []byte(schema)},
	}
	loader := NewLoader(Config{FS: fsys})
	if _, err := loadAndPrepare(t, loader, "test.xsd"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParticleRestriction_PointlessChoiceToSequence(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		<xs:complexType name="Base">
			<xs:sequence>
				<xs:choice>
					<xs:sequence>
						<xs:element name="a" minOccurs="0"/>
						<xs:element name="b" minOccurs="0"/>
					</xs:sequence>
					<xs:sequence>
						<xs:element name="c" minOccurs="0"/>
					</xs:sequence>
				</xs:choice>
			</xs:sequence>
		</xs:complexType>
		<xs:complexType name="Derived">
			<xs:complexContent>
				<xs:restriction base="Base">
					<xs:sequence>
						<xs:choice>
							<xs:sequence>
								<xs:element name="a" minOccurs="0"/>
								<xs:element name="b" minOccurs="0"/>
							</xs:sequence>
						</xs:choice>
					</xs:sequence>
				</xs:restriction>
			</xs:complexContent>
		</xs:complexType>
	</xs:schema>`

	fsys := fstest.MapFS{
		"test.xsd": &fstest.MapFile{Data: []byte(schema)},
	}
	loader := NewLoader(Config{FS: fsys})
	_, err := loadAndPrepare(t, loader, "test.xsd")
	if err == nil {
		t.Fatal("Expected schema validation error but got none")
	}
	if !strings.Contains(err.Error(), "maxOccurs") {
		t.Fatalf("Expected occurrence constraint error, got: %v", err)
	}
}

func TestParticleRestriction_PointlessChoiceOptionalSequence(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		<xs:complexType name="Base">
			<xs:sequence>
				<xs:choice>
					<xs:sequence>
						<xs:element name="a"/>
					</xs:sequence>
					<xs:sequence>
						<xs:element name="b"/>
					</xs:sequence>
				</xs:choice>
			</xs:sequence>
		</xs:complexType>
		<xs:complexType name="Derived">
			<xs:complexContent>
				<xs:restriction base="Base">
					<xs:sequence>
						<xs:choice>
							<xs:sequence minOccurs="0">
								<xs:element name="a"/>
							</xs:sequence>
						</xs:choice>
					</xs:sequence>
				</xs:restriction>
			</xs:complexContent>
		</xs:complexType>
	</xs:schema>`

	fsys := fstest.MapFS{
		"test.xsd": &fstest.MapFile{Data: []byte(schema)},
	}
	loader := NewLoader(Config{FS: fsys})
	_, err := loadAndPrepare(t, loader, "test.xsd")
	if err == nil {
		t.Fatal("Expected schema validation error but got none")
	}
	if !strings.Contains(err.Error(), "minOccurs") {
		t.Fatalf("Expected occurrence constraint error, got: %v", err)
	}
}
