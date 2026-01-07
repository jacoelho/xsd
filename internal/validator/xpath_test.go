package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/xml"
)

func TestEvaluateSelectorWithNSUnprefixedMatchesNoNamespace(t *testing.T) {
	doc, err := xml.Parse(strings.NewReader(`
<root>
  <child xmlns="urn:ns">namespaced</child>
  <child>local</child>
</root>
`))
	if err != nil {
		t.Fatalf("parse xml: %v", err)
	}

	root := doc.DocumentElement()
	if root == xml.InvalidNode {
		t.Fatal("missing root element")
	}

	v := New(nil)
	run := v.newRun()
	run.root = root
	run.doc = doc
	nsContext := map[string]string{"": "urn:ns"}
	results := run.evaluateSelectorWithNS(root, "child", nsContext)
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results))
	}
	if doc.NamespaceURI(results[0]) != "" {
		t.Fatalf("expected no-namespace match, got namespace %q", doc.NamespaceURI(results[0]))
	}
}
