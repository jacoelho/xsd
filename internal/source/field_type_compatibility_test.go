package source

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/types"
)

// TestKeyRef_FieldCountMismatch tests that keyref with wrong number of fields fails validation
func TestKeyRef_FieldCountMismatch(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="http://example.com"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="purchaseReport">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parts">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="number" type="xs:integer"/>
                  <xs:attribute name="name" type="xs:string"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
        <xs:element name="orders">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="order" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:sequence>
                    <xs:element name="part">
                      <xs:complexType>
                        <xs:attribute name="number" type="xs:integer"/>
                      </xs:complexType>
                    </xs:element>
                  </xs:sequence>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="tns:parts/tns:part"/>
      <xs:field xpath="@number"/>
      <xs:field xpath="@name"/>
    </xs:key>
    <xs:keyref name="partRef" refer="partKey">
      <xs:selector xpath="tns:orders/tns:order/tns:part"/>
      <xs:field xpath="@number"/>
      <!-- Missing second field - should fail -->
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Fatal("Load() should return error for keyref with wrong number of fields")
	}

	if !strings.Contains(err.Error(), "has 1 fields but referenced constraint") {
		t.Errorf("error should mention field count mismatch, got: %v", err)
	}
	if !strings.Contains(err.Error(), "has 2 fields") {
		t.Errorf("error should mention expected field count, got: %v", err)
	}
}

// TestKeyRef_IncompatibleFieldTypes tests that keyref with incompatible field types fails validation
func TestKeyRef_IncompatibleFieldTypes(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="http://example.com"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="purchaseReport">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parts">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="number" type="xs:integer"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
        <xs:element name="orders">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="order" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:sequence>
                    <xs:element name="part">
                      <xs:complexType>
                        <xs:attribute name="number" type="xs:string"/>
                      </xs:complexType>
                    </xs:element>
                  </xs:sequence>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="tns:parts/tns:part"/>
      <xs:field xpath="@number"/>
    </xs:key>
    <xs:keyref name="partRef" refer="partKey">
      <xs:selector xpath="tns:orders/tns:order/tns:part"/>
      <xs:field xpath="@number"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Fatal("Load() should return error for keyref with incompatible field types")
	}

	if !strings.Contains(err.Error(), "is not compatible") {
		t.Errorf("error should mention type incompatibility, got: %v", err)
	}
}

// TestKeyRef_NamespaceContextMismatch tests that namespace context affects field type resolution.
func TestKeyRef_NamespaceContextMismatch(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:int" form="unqualified"/>
        <xs:element name="item" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:item"/>
      <xs:field xpath="."/>
    </xs:key>
    <xs:keyref name="itemRef" refer="tns:itemKey">
      <xs:selector xpath="item"/>
      <xs:field xpath="."/>
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Fatal("Load() should return error for keyref with namespace-mismatched field types")
	}
	if !strings.Contains(err.Error(), "not compatible") {
		t.Fatalf("error should mention type incompatibility, got: %v", err)
	}
}

// TestKeyRef_CompatibleFieldTypes tests that keyref with compatible field types passes validation
func TestKeyRef_CompatibleFieldTypes(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="http://example.com"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="purchaseReport">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parts">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="number" type="xs:integer"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
        <xs:element name="orders">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="order" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:sequence>
                    <xs:element name="part">
                      <xs:complexType>
                        <xs:attribute name="number" type="xs:long"/>
                      </xs:complexType>
                    </xs:element>
                  </xs:sequence>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="tns:parts/tns:part"/>
      <xs:field xpath="@number"/>
    </xs:key>
    <xs:keyref name="partRef" refer="partKey">
      <xs:selector xpath="tns:orders/tns:order/tns:part"/>
      <xs:field xpath="@number"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() should succeed for keyref with compatible field types, got error: %v", err)
	}

	if schema == nil {
		t.Fatal("Load() returned nil schema")
	}

	// verify the constraints are present
	purchaseReportQName := types.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport",
	}
	decl, ok := schema.ElementDecls[purchaseReportQName]
	if !ok {
		t.Fatal("element 'purchaseReport' not found")
	}

	var keyrefConstraint *types.IdentityConstraint
	for _, constraint := range decl.Constraints {
		if constraint.Name == "partRef" {
			keyrefConstraint = constraint
			break
		}
	}
	if keyrefConstraint == nil {
		t.Fatal("keyref constraint 'partRef' not found")
	}

	// verify field types are resolved
	if len(keyrefConstraint.Fields) == 0 {
		t.Fatal("keyref constraint should have fields")
	}
	if keyrefConstraint.Fields[0].ResolvedType == nil {
		t.Error("keyref field type should be resolved")
	}
}

// TestKeyRef_SameFieldTypes tests that keyref with identical field types passes validation
func TestKeyRef_SameFieldTypes(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="http://example.com"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="purchaseReport">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parts">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="number" type="xs:integer"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
        <xs:element name="orders">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="order" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:sequence>
                    <xs:element name="part">
                      <xs:complexType>
                        <xs:attribute name="number" type="xs:integer"/>
                      </xs:complexType>
                    </xs:element>
                  </xs:sequence>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="tns:parts/tns:part"/>
      <xs:field xpath="@number"/>
    </xs:key>
    <xs:keyref name="partRef" refer="partKey">
      <xs:selector xpath="tns:orders/tns:order/tns:part"/>
      <xs:field xpath="@number"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() should succeed for keyref with identical field types, got error: %v", err)
	}

	if schema == nil {
		t.Fatal("Load() returned nil schema")
	}
}

// TestKeyRef_UniqueConstraint tests that keyref can reference unique constraint with compatible types
func TestKeyRef_UniqueConstraint(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="http://example.com"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="purchaseReport">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parts">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="number" type="xs:integer"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
        <xs:element name="orders">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="order" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:sequence>
                    <xs:element name="part">
                      <xs:complexType>
                        <xs:attribute name="number" type="xs:integer"/>
                      </xs:complexType>
                    </xs:element>
                  </xs:sequence>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="partUnique">
      <xs:selector xpath="tns:parts/tns:part"/>
      <xs:field xpath="@number"/>
    </xs:unique>
    <xs:keyref name="partRef" refer="partUnique">
      <xs:selector xpath="tns:orders/tns:order/tns:part"/>
      <xs:field xpath="@number"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() should succeed for keyref referencing unique constraint, got error: %v", err)
	}

	if schema == nil {
		t.Fatal("Load() returned nil schema")
	}
}

// TestKeyRef_DescendantOrSelfPrefix tests that descendant-or-self selectors work correctly
func TestKeyRef_DescendantOrSelfPrefix(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="http://example.com"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="container">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="wrapper">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="target" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="id" type="xs:integer"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
        <xs:element name="references">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="ref" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="targetId" type="xs:integer"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="targetKey">
      <xs:selector xpath=".//tns:target"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="targetRef" refer="targetKey">
      <xs:selector xpath="tns:references/tns:ref"/>
      <xs:field xpath="@targetId"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() should succeed for keyref with descendant-or-self selector, got error: %v", err)
	}

	if schema == nil {
		t.Fatal("Load() returned nil schema")
	}

	// verify field types are resolved
	containerQName := types.QName{
		Namespace: "http://example.com",
		Local:     "container",
	}
	decl, ok := schema.ElementDecls[containerQName]
	if !ok {
		t.Fatal("element 'container' not found")
	}

	var keyConstraint *types.IdentityConstraint
	for _, constraint := range decl.Constraints {
		if constraint.Name == "targetKey" {
			keyConstraint = constraint
			break
		}
	}
	if keyConstraint == nil {
		t.Fatal("key constraint 'targetKey' not found")
	}

	// verify field type is resolved for descendant-or-self selector
	if len(keyConstraint.Fields) == 0 {
		t.Fatal("key constraint should have fields")
	}
	if keyConstraint.Fields[0].ResolvedType == nil {
		t.Error("key field type should be resolved for descendant selector")
	}
}

// TestKeyRef_MultipleFields tests that keyref with multiple fields validates all field types
func TestKeyRef_MultipleFields(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="http://example.com"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="items">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="item" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="category" type="xs:string"/>
                  <xs:attribute name="code" type="xs:integer"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
        <xs:element name="references">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="ref" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="category" type="xs:string"/>
                  <xs:attribute name="code" type="xs:long"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:items/tns:item"/>
      <xs:field xpath="@category"/>
      <xs:field xpath="@code"/>
    </xs:key>
    <xs:keyref name="itemRef" refer="itemKey">
      <xs:selector xpath="tns:references/tns:ref"/>
      <xs:field xpath="@category"/>
      <xs:field xpath="@code"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() should succeed for keyref with multiple compatible fields, got error: %v", err)
	}

	if schema == nil {
		t.Fatal("Load() returned nil schema")
	}
}

// TestKeyRef_MultipleFieldsIncompatible tests that keyref with incompatible second field fails
func TestKeyRef_MultipleFieldsIncompatible(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="http://example.com"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="items">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="item" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="category" type="xs:string"/>
                  <xs:attribute name="code" type="xs:integer"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
        <xs:element name="references">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="ref" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="category" type="xs:string"/>
                  <xs:attribute name="code" type="xs:string"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:items/tns:item"/>
      <xs:field xpath="@category"/>
      <xs:field xpath="@code"/>
    </xs:key>
    <xs:keyref name="itemRef" refer="itemKey">
      <xs:selector xpath="tns:references/tns:ref"/>
      <xs:field xpath="@category"/>
      <xs:field xpath="@code"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Fatal("Load() should return error for keyref with incompatible second field type")
	}

	if !strings.Contains(err.Error(), "is not compatible") {
		t.Errorf("error should mention type incompatibility, got: %v", err)
	}
}

// TestKeyRef_BuiltinTypeDerivation tests built-in type derivation chains (e.g., decimal -> integer -> long)
func TestKeyRef_BuiltinTypeDerivation(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="http://example.com"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="items">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="item" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="value" type="xs:decimal"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
        <xs:element name="references">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="ref" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="value" type="xs:long"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="tns:items/tns:item"/>
      <xs:field xpath="@value"/>
    </xs:key>
    <xs:keyref name="itemRef" refer="itemKey">
      <xs:selector xpath="tns:references/tns:ref"/>
      <xs:field xpath="@value"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() should succeed for keyref with compatible built-in types (decimal and long share primitive), got error: %v", err)
	}

	if schema == nil {
		t.Fatal("Load() returned nil schema")
	}
}
