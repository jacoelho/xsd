package preprocessor

import (
	"github.com/jacoelho/xsd/internal/objects"
	parser "github.com/jacoelho/xsd/internal/parser"
	qnameorder "github.com/jacoelho/xsd/internal/qname"
)

func initSchemaOrigins(sch *parser.Schema, location string) {
	if sch == nil {
		return
	}
	sch.Location = parser.ImportContextKey("", location)
	for _, qname := range sortedQNames(sch.ElementDecls) {
		if sch.ElementOrigins[qname] == "" {
			sch.ElementOrigins[qname] = sch.Location
		}
	}
	for _, qname := range sortedQNames(sch.TypeDefs) {
		if sch.TypeOrigins[qname] == "" {
			sch.TypeOrigins[qname] = sch.Location
		}
	}
	for _, qname := range sortedQNames(sch.AttributeDecls) {
		if sch.AttributeOrigins[qname] == "" {
			sch.AttributeOrigins[qname] = sch.Location
		}
	}
	for _, qname := range sortedQNames(sch.AttributeGroups) {
		if sch.AttributeGroupOrigins[qname] == "" {
			sch.AttributeGroupOrigins[qname] = sch.Location
		}
	}
	for _, qname := range sortedQNames(sch.Groups) {
		if sch.GroupOrigins[qname] == "" {
			sch.GroupOrigins[qname] = sch.Location
		}
	}
	for _, qname := range sortedQNames(sch.NotationDecls) {
		if sch.NotationOrigins[qname] == "" {
			sch.NotationOrigins[qname] = sch.Location
		}
	}
}

func sortedQNames[V any](m map[objects.QName]V) []objects.QName {
	return qnameorder.SortedMapKeys(m)
}
