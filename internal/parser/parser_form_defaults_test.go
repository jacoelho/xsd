package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func parseFixture(t *testing.T, name string) *Schema {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "parser", name)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("close fixture %s: %v", name, err)
		}
	}()

	schema, err := Parse(file)
	if err != nil {
		t.Fatalf("Parse(%s) error = %v", name, err)
	}
	return schema
}

func findLocalElement(t *testing.T, group *types.ModelGroup, local string) *types.ElementDecl {
	t.Helper()
	for _, particle := range group.Particles {
		elem, ok := particle.(*types.ElementDecl)
		if !ok {
			continue
		}
		if elem.Name.Local == local {
			return elem
		}
	}
	t.Fatalf("local element %q not found", local)
	return nil
}

func findAttribute(t *testing.T, attrs []*types.AttributeDecl, local string) *types.AttributeDecl {
	t.Helper()
	for _, attr := range attrs {
		if attr.Name.Local == local {
			return attr
		}
	}
	t.Fatalf("attribute %q not found", local)
	return nil
}

func TestParseFormDefaultsQualified(t *testing.T) {
	schema := parseFixture(t, "form_defaults_qualified.xsd")
	if schema.ElementFormDefault != Qualified {
		t.Fatalf("ElementFormDefault = %v, want Qualified", schema.ElementFormDefault)
	}
	if schema.AttributeFormDefault != Unqualified {
		t.Fatalf("AttributeFormDefault = %v, want Unqualified", schema.AttributeFormDefault)
	}

	root := schema.ElementDecls[types.QName{Namespace: "urn:qualified", Local: "root"}]
	if root == nil {
		t.Fatalf("element 'root' not found")
	}
	ct, ok := root.Type.(*types.ComplexType)
	if !ok {
		t.Fatalf("root type = %T, want *types.ComplexType", root.Type)
	}
	content, ok := ct.Content().(*types.ElementContent)
	if !ok {
		t.Fatalf("root content = %T, want *types.ElementContent", ct.Content())
	}
	group, ok := content.Particle.(*types.ModelGroup)
	if !ok {
		t.Fatalf("root particle = %T, want *types.ModelGroup", content.Particle)
	}

	child := findLocalElement(t, group, "child")
	if child.Form != types.FormQualified {
		t.Fatalf("child form = %v, want FormQualified", child.Form)
	}
	if child.Name.Namespace != types.NamespaceURI("urn:qualified") {
		t.Fatalf("child namespace = %q, want %q", child.Name.Namespace, "urn:qualified")
	}

	childUnq := findLocalElement(t, group, "childUnq")
	if childUnq.Form != types.FormUnqualified {
		t.Fatalf("childUnq form = %v, want FormUnqualified", childUnq.Form)
	}
	if childUnq.Name.Namespace != "" {
		t.Fatalf("childUnq namespace = %q, want empty", childUnq.Name.Namespace)
	}

	childAnon := findLocalElement(t, group, "childAnon")
	st, ok := childAnon.Type.(*types.SimpleType)
	if !ok {
		t.Fatalf("childAnon type = %T, want *types.SimpleType", childAnon.Type)
	}
	if !st.QName.IsZero() {
		t.Fatalf("childAnon type QName = %s, want zero", st.QName)
	}

	attr := findAttribute(t, ct.Attributes(), "attr")
	if attr.Form != types.FormUnqualified {
		t.Fatalf("attr form = %v, want FormUnqualified", attr.Form)
	}

	attrQ := findAttribute(t, ct.Attributes(), "attrQ")
	if attrQ.Form != types.FormQualified {
		t.Fatalf("attrQ form = %v, want FormQualified", attrQ.Form)
	}
}

func TestParseFormDefaultsUnqualifiedDefaultNamespace(t *testing.T) {
	schema := parseFixture(t, "form_defaults_unqualified.xsd")
	if schema.ElementFormDefault != Unqualified {
		t.Fatalf("ElementFormDefault = %v, want Unqualified", schema.ElementFormDefault)
	}
	if schema.AttributeFormDefault != Qualified {
		t.Fatalf("AttributeFormDefault = %v, want Qualified", schema.AttributeFormDefault)
	}

	root := schema.ElementDecls[types.QName{Namespace: "urn:unqualified", Local: "root"}]
	if root == nil {
		t.Fatalf("element 'root' not found")
	}
	ct, ok := root.Type.(*types.ComplexType)
	if !ok {
		t.Fatalf("root type = %T, want *types.ComplexType", root.Type)
	}
	content, ok := ct.Content().(*types.ElementContent)
	if !ok {
		t.Fatalf("root content = %T, want *types.ElementContent", ct.Content())
	}
	group, ok := content.Particle.(*types.ModelGroup)
	if !ok {
		t.Fatalf("root particle = %T, want *types.ModelGroup", content.Particle)
	}

	child := findLocalElement(t, group, "child")
	if child.Form != types.FormUnqualified {
		t.Fatalf("child form = %v, want FormUnqualified", child.Form)
	}
	if child.Name.Namespace != "" {
		t.Fatalf("child namespace = %q, want empty", child.Name.Namespace)
	}

	childQ := findLocalElement(t, group, "childQ")
	if childQ.Form != types.FormQualified {
		t.Fatalf("childQ form = %v, want FormQualified", childQ.Form)
	}
	if childQ.Name.Namespace != types.NamespaceURI("urn:unqualified") {
		t.Fatalf("childQ namespace = %q, want %q", childQ.Name.Namespace, "urn:unqualified")
	}

	attr := findAttribute(t, ct.Attributes(), "attr")
	if attr.Form != types.FormQualified {
		t.Fatalf("attr form = %v, want FormQualified", attr.Form)
	}

	attrU := findAttribute(t, ct.Attributes(), "attrU")
	if attrU.Form != types.FormUnqualified {
		t.Fatalf("attrU form = %v, want FormUnqualified", attrU.Form)
	}
}
