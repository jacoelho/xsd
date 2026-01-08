package validator

import (
	"strings"

	"github.com/jacoelho/xsd/internal/xml"
)

type schemaLocationHint struct {
	namespace string
	location  string
	attribute string
}

func schemaLocationHintsFromAttrs(attrs []xml.Attr) []schemaLocationHint {
	if len(attrs) == 0 {
		return nil
	}
	var hints []schemaLocationHint
	for _, attr := range attrs {
		if attr.NamespaceURI() != xml.XSINamespace {
			continue
		}
		switch attr.LocalName() {
		case "schemaLocation":
			hints = append(hints, schemaLocationHintsFromSchemaLocation(attr.Value())...)
		case "noNamespaceSchemaLocation":
			if attr.Value() != "" {
				hints = append(hints, schemaLocationHint{
					namespace: "",
					location:  attr.Value(),
					attribute: "xsi:noNamespaceSchemaLocation",
				})
			}
		}
	}
	return hints
}

func schemaLocationHintsFromSchemaLocation(value string) []schemaLocationHint {
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return nil
	}
	hints := make([]schemaLocationHint, 0, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		hints = append(hints, schemaLocationHint{
			namespace: fields[i],
			location:  fields[i+1],
			attribute: "xsi:schemaLocation",
		})
	}
	return hints
}

func hasSchemaLocationHint(attrs []xml.Attr) bool {
	for _, attr := range attrs {
		if attr.NamespaceURI() != xml.XSINamespace {
			continue
		}
		switch attr.LocalName() {
		case "schemaLocation":
			if len(strings.Fields(attr.Value())) >= 2 {
				return true
			}
		case "noNamespaceSchemaLocation":
			if attr.Value() != "" {
				return true
			}
		}
	}
	return false
}
