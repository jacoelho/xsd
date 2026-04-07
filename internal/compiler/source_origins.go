package compiler

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func InitOrigins(sch *parser.Schema, location string) {
	if sch == nil {
		return
	}

	sch.Location = parser.ImportContextKey("", location)
	sch.ElementOrigins = assignMissingOrigins(sch.ElementOrigins, sch.ElementDecls, sch.Location)
	sch.TypeOrigins = assignMissingOrigins(sch.TypeOrigins, sch.TypeDefs, sch.Location)
	sch.AttributeOrigins = assignMissingOrigins(sch.AttributeOrigins, sch.AttributeDecls, sch.Location)
	sch.AttributeGroupOrigins = assignMissingOrigins(sch.AttributeGroupOrigins, sch.AttributeGroups, sch.Location)
	sch.GroupOrigins = assignMissingOrigins(sch.GroupOrigins, sch.Groups, sch.Location)
	sch.NotationOrigins = assignMissingOrigins(sch.NotationOrigins, sch.NotationDecls, sch.Location)
}

func assignMissingOrigins[V any](origins map[model.QName]string, decls map[model.QName]V, location string) map[model.QName]string {
	if len(decls) == 0 {
		return origins
	}
	if origins == nil {
		origins = make(map[model.QName]string, len(decls))
	}
	for _, name := range model.SortedMapKeys(decls) {
		if origins[name] == "" {
			origins[name] = location
		}
	}
	return origins
}
