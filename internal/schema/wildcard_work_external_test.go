package schema_test

import (
	"fmt"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestWildcardNamespaceComparisonsChargeEveryNamespaceCompared(t *testing.T) {
	t.Parallel()
	const namespaces = 512
	all := wildcardNamespaceList(0, 1, 2*namespaces)
	even := wildcardNamespaceList(0, 2, namespaces)
	odd := wildcardNamespaceList(1, 2, namespaces)
	contentLimit := func(steps int) xsdSchema.Options {
		return xsdSchema.Options{MaxContentModelAnalysisSteps: steps}
	}
	dependencyLimit := func(steps int) xsdSchema.Options {
		return xsdSchema.Options{MaxSchemaDependencySteps: steps}
	}
	for _, test := range []struct {
		name               string
		limit              func(int) xsdSchema.Options
		compared, separate string
	}{
		{
			name:     "element wildcard overlap",
			limit:    contentLimit,
			compared: `<xs:complexType name="T"><xs:choice>` + anyElement(even) + anyElement(odd) + `</xs:choice></xs:complexType>`,
			separate: `<xs:complexType name="T"><xs:sequence>` + anyElement(even) + `</xs:sequence></xs:complexType>` +
				`<xs:complexType name="U"><xs:sequence>` + anyElement(odd) + `</xs:sequence></xs:complexType>`,
		},
		{
			name:  "element wildcard restriction subset",
			limit: contentLimit,
			compared: `<xs:complexType name="B"><xs:sequence>` + anyElement(all) + `</xs:sequence></xs:complexType>` +
				`<xs:complexType name="D"><xs:complexContent><xs:restriction base="B"><xs:sequence>` + anyElement(odd) + `</xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`,
			separate: `<xs:complexType name="B"><xs:sequence>` + anyElement(all) + `</xs:sequence></xs:complexType>` +
				`<xs:complexType name="D"><xs:complexContent><xs:restriction base="B"><xs:sequence>` + anyElement(odd[:1]) + `</xs:sequence></xs:restriction></xs:complexContent></xs:complexType>` +
				`<xs:complexType name="U"><xs:sequence>` + anyElement(odd) + `</xs:sequence></xs:complexType>`,
		},
		{
			name:  "attribute wildcard intersection",
			limit: dependencyLimit,
			compared: anyAttributeGroups(even, odd) +
				`<xs:complexType name="T"><xs:attributeGroup ref="GA"/><xs:attributeGroup ref="GB"/></xs:complexType>`,
			separate: anyAttributeGroups(even, odd) +
				`<xs:complexType name="T"><xs:attributeGroup ref="GA"/></xs:complexType>` +
				`<xs:complexType name="U"><xs:attributeGroup ref="GB"/></xs:complexType>`,
		},
		{
			name:  "attribute wildcard restriction subset",
			limit: dependencyLimit,
			compared: `<xs:complexType name="B">` + anyAttribute(all) + `</xs:complexType>` +
				`<xs:complexType name="D"><xs:complexContent><xs:restriction base="B">` + anyAttribute(odd) + `</xs:restriction></xs:complexContent></xs:complexType>`,
			separate: `<xs:complexType name="B">` + anyAttribute(all) + `</xs:complexType>` +
				`<xs:complexType name="D"><xs:complexContent><xs:restriction base="B">` + anyAttribute(odd[:1]) + `</xs:restriction></xs:complexContent></xs:complexType>` +
				`<xs:complexType name="U">` + anyAttribute(odd) + `</xs:complexType>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			seed := `<xs:complexType name="Seed">` + anyAttribute(all) + `</xs:complexType>`
			compared := minimumCompileLimit(t, wildcardWorkSchema(seed+test.compared), test.limit)
			separate := minimumCompileLimit(t, wildcardWorkSchema(seed+test.separate), test.limit)
			if compared-separate < namespaces/2 {
				t.Fatalf("comparing two interleaved lists of %d namespaces needs %d steps, holding them apart %d; want at least %d more", namespaces, compared, separate, namespaces/2)
			}
		})
	}
}

func wildcardNamespaceList(first, stride, count int) []string {
	out := make([]string, count)
	for i := range out {
		out[i] = fmt.Sprintf("urn:ns:%d", first+i*stride)
	}
	return out
}

func anyElement(namespaces []string) string {
	return `<xs:any namespace="` + strings.Join(namespaces, " ") + `" processContents="skip"/>`
}

func anyAttribute(namespaces []string) string {
	return `<xs:anyAttribute namespace="` + strings.Join(namespaces, " ") + `" processContents="skip"/>`
}

func anyAttributeGroups(a, b []string) string {
	return `<xs:attributeGroup name="GA">` + anyAttribute(a) + `</xs:attributeGroup>` +
		`<xs:attributeGroup name="GB">` + anyAttribute(b) + `</xs:attributeGroup>`
}

func wildcardWorkSchema(body string) string {
	return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + body + `</xs:schema>`
}

func minimumCompileLimit(t *testing.T, document string, limit func(int) xsdSchema.Options) int {
	t.Helper()
	compiles := func(steps int) bool {
		_, err := xsdSchema.Compile(limit(steps), []source.Source{source.Bytes("schema.xsd", []byte(document))})
		if err == nil {
			return true
		}
		expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
		return false
	}
	low, high := 1, 1<<24
	if !compiles(high) {
		t.Fatalf("schema does not compile within %d steps", high)
	}
	for low < high {
		mid := low + (high-low)/2
		if compiles(mid) {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return low
}
