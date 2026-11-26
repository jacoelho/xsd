package schema_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
)

func TestUPAVerdictIgnoresLanguagePreservingNesting(t *testing.T) {
	var members []string
	for _, name := range []string{"x", "y"} {
		for _, occurs := range []string{``, ` minOccurs="0"`, ` maxOccurs="unbounded"`, ` minOccurs="0" maxOccurs="unbounded"`} {
			members = append(members, fmt.Sprintf(`<xs:element name="%s" type="xs:string"%s/>`, name, occurs))
		}
	}
	wrappers := map[string]func(run string) (group, particle string){
		"sequence": func(run string) (string, string) { return "", "<xs:sequence>" + run + "</xs:sequence>" },
		"group-ref": func(run string) (string, string) {
			return `<xs:group name="g"><xs:sequence>` + run + `</xs:sequence></xs:group>`, `<xs:group ref="g"/>`
		},
		"choice": func(run string) (string, string) {
			return "", "<xs:choice><xs:sequence>" + run + "</xs:sequence></xs:choice>"
		},
	}
	for _, flat := range memberSequences(members, 3) {
		want := upaNestingCompileError(t, "", strings.Join(flat, "")) == nil
		for start := range flat {
			for end := start + 1; end <= len(flat); end++ {
				run := strings.Join(flat[start:end], "")
				for wrapperName, wrap := range wrappers {
					group, particle := wrap(run)
					content := strings.Join(flat[:start], "") + particle + strings.Join(flat[end:], "")
					err := upaNestingCompileError(t, group, content)
					if got := err == nil; got != want {
						t.Errorf("flat sequence %s valid=%v, %s wrapping members [%d,%d) valid=%v (err %v)", strings.Join(flat, ""), want, wrapperName, start, end, got, err)
					}
				}
			}
		}
	}
}

func memberSequences(members []string, maxLen int) [][]string {
	var out [][]string
	var extend func(prefix []string)
	extend = func(prefix []string) {
		if len(prefix) > 0 {
			out = append(out, prefix)
		}
		if len(prefix) == maxLen {
			return
		}
		for _, m := range members {
			extend(append(slices.Clip(prefix), m))
		}
	}
	extend(nil)
	return out
}

func upaNestingCompileError(t *testing.T, group, content string) error {
	t.Helper()
	text := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + group +
		`<xs:complexType name="T"><xs:sequence>` + content + `</xs:sequence></xs:complexType>` +
		`<xs:element name="root" type="T"/></xs:schema>`
	_, err := compileSources(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(text))})
	if err != nil && !strings.Contains(err.Error(), "UPA violation") {
		t.Fatalf("compile %s: %v", text, err)
	}
	return err
}
