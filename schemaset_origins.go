package xsd

import (
	"maps"
	"strconv"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func schemaSetRootKey(index int) string {
	return "root:" + strconv.Itoa(index)
}

func disambiguateSchemaOriginsForRoot(sch *parser.Schema, rootKey string) {
	if sch == nil || rootKey == "" {
		return
	}

	sch.Location = remapOriginKey(sch.Location, rootKey)
	remapOriginMap(sch.ElementOrigins, rootKey)
	remapOriginMap(sch.TypeOrigins, rootKey)
	remapOriginMap(sch.AttributeOrigins, rootKey)
	remapOriginMap(sch.AttributeGroupOrigins, rootKey)
	remapOriginMap(sch.GroupOrigins, rootKey)
	remapOriginMap(sch.NotationOrigins, rootKey)
	sch.ImportContexts = remapImportContexts(sch.ImportContexts, rootKey)
}

func remapOriginKey(key, rootKey string) string {
	if key == "" {
		return ""
	}
	location := parser.ImportContextLocation(key)
	return parser.ImportContextKey(rootKey, location)
}

func remapOriginMap[K comparable](origins map[K]string, rootKey string) {
	for k, key := range origins {
		origins[k] = remapOriginKey(key, rootKey)
	}
}

func remapImportContexts(
	contexts map[string]parser.ImportContext,
	rootKey string,
) map[string]parser.ImportContext {
	if len(contexts) == 0 {
		return contexts
	}

	remapped := make(map[string]parser.ImportContext, len(contexts))
	for key, ctx := range contexts {
		newKey := remapOriginKey(key, rootKey)
		if existing, ok := remapped[newKey]; ok {
			if existing.Imports == nil {
				existing.Imports = make(map[model.NamespaceURI]bool)
			}
			maps.Copy(existing.Imports, ctx.Imports)
			if existing.TargetNamespace == "" {
				existing.TargetNamespace = ctx.TargetNamespace
			}
			remapped[newKey] = existing
			continue
		}
		remapped[newKey] = ctx
	}
	return remapped
}
