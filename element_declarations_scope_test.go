package xsd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

// anonymousChild declares a local element whose anonymous type is distinct
// from every other anonymous type, so two declarations of one name in one
// content model are inconsistent.
func anonymousChild(name, attribute string) string {
	return `<xs:element name="` + name + `"><xs:complexType><xs:attribute name="` + attribute + `"/></xs:complexType></xs:element>`
}

func TestElementDeclarationsConsistentComparesOneContentModel(t *testing.T) {
	t.Parallel()

	element := func(name, model string) string {
		return `<xs:element name="` + name + `"><xs:complexType>` + model + `</xs:complexType></xs:element>`
	}
	var wide strings.Builder
	for i := range 300 {
		wide.WriteString(anonymousChild(fmt.Sprintf("x%d", i), "wide"))
	}
	tests := []struct {
		name     string
		body     string
		conflict bool
	}{
		{
			name: "same name in sibling content models",
			body: element("a", `<xs:sequence>`+anonymousChild("x", "p")+`</xs:sequence>`) +
				element("b", `<xs:sequence>`+anonymousChild("x", "q")+`</xs:sequence>`),
		},
		{
			name: "same name in a nested group of one model and in the next model",
			body: element("a", `<xs:sequence>`+anonymousChild("y", "p")+`<xs:choice>`+anonymousChild("x", "p")+`</xs:choice></xs:sequence>`) +
				element("b", `<xs:sequence>`+anonymousChild("x", "q")+`<xs:choice>`+anonymousChild("y", "q")+`</xs:choice></xs:sequence>`),
		},
		{
			name: "same name after a content model with many names",
			body: element("a", `<xs:sequence>`+wide.String()+`</xs:sequence>`) +
				element("b", `<xs:sequence>`+anonymousChild("x0", "q")+anonymousChild("x299", "q")+`</xs:sequence>`),
		},
		{
			name: "same name in a named group used by two content models",
			body: `<xs:group name="g"><xs:sequence>` + anonymousChild("x", "p") + `</xs:sequence></xs:group>` +
				element("a", `<xs:sequence><xs:group ref="g"/></xs:sequence>`) +
				element("b", `<xs:sequence><xs:group ref="g"/>`+anonymousChild("y", "q")+`</xs:sequence>`),
		},
		{
			name:     "same name twice in one content model",
			body:     element("a", `<xs:sequence>`+anonymousChild("x", "p")+anonymousChild("x", "q")+`</xs:sequence>`),
			conflict: true,
		},
		{
			name: "same name in a content model and in its nested group",
			body: element("a", `<xs:sequence>`+anonymousChild("y", "p")+`</xs:sequence>`) +
				element("b", `<xs:sequence>`+anonymousChild("x", "p")+`<xs:choice>`+anonymousChild("y", "p")+anonymousChild("x", "q")+`</xs:choice></xs:sequence>`),
			conflict: true,
		},
		{
			name: "same name in a content model and in the named group it uses",
			body: `<xs:group name="g"><xs:sequence>` + anonymousChild("x", "p") + `</xs:sequence></xs:group>` +
				element("a", `<xs:sequence><xs:group ref="g"/></xs:sequence>`) +
				element("b", `<xs:sequence><xs:group ref="g"/>`+anonymousChild("x", "q")+`</xs:sequence>`),
			conflict: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + tt.body + `</xs:schema>`
			_, err := xsd.Compile(xsd.Bytes("edc.xsd", []byte(schema)))
			if !tt.conflict {
				if err != nil {
					t.Fatalf("Compile() = %v, want a schema whose content models each declare a name once", err)
				}
				return
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != xsderrors.CodeSchemaContentModel || !strings.Contains(diagnostic.Message(), "element declarations with the same name") {
				t.Fatalf("Compile() = %v, want schema.content_model for element declarations with the same name", err)
			}
		})
	}
}
