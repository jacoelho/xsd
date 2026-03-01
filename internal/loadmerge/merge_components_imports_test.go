package loadmerge

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestMergeImportContextsClonesImportsMap(t *testing.T) {
	source := parser.NewSchema()
	target := parser.NewSchema()
	source.ImportContexts["schema.xsd"] = parser.ImportContext{
		TargetNamespace: "urn:source",
		Imports: map[model.NamespaceURI]bool{
			"urn:a": true,
		},
	}
	ctx := mergeContext{source: source, target: target}

	ctx.mergeImportContexts()

	source.ImportContexts["schema.xsd"].Imports["urn:b"] = true
	merged := target.ImportContexts["schema.xsd"]
	if merged.Imports["urn:b"] {
		t.Fatal("target import contexts aliased source imports map")
	}
}
