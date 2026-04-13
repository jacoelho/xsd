package compiler

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// PrepareRoots loads one or more schema roots and normalizes them for runtime builds.
func PrepareRoots(cfg LoadConfig) (*Prepared, error) {
	parsed, err := loadRoots(cfg)
	if err != nil {
		return nil, err
	}
	return PrepareOwned(parsed)
}

func loadRoots(cfg LoadConfig) (*parser.Schema, error) {
	roots, err := normalizeRoots(cfg.Roots)
	if err != nil {
		return nil, err
	}
	if len(roots) == 1 {
		root := roots[0]
		loader := newLoader(root, cfg)
		parsed, err := loader.Load(root.Location)
		if err != nil {
			return nil, fmt.Errorf("load parsed schema: %w", err)
		}
		return parsed, nil
	}
	return loadAndMergeRoots(roots, cfg)
}

func normalizeRoots(input []Root) ([]Root, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("prepare schema: no roots")
	}

	roots := make([]Root, 0, len(input))
	for i, root := range input {
		if root.FS == nil {
			return nil, fmt.Errorf("prepare schema: roots[%d]: nil fs", i)
		}
		location := strings.TrimSpace(root.Location)
		if location == "" {
			return nil, fmt.Errorf("prepare schema: roots[%d]: empty location", i)
		}
		roots = append(roots, Root{
			FS:       root.FS,
			Resolver: root.Resolver,
			Location: location,
		})
	}
	return roots, nil
}

func loadAndMergeRoots(roots []Root, cfg LoadConfig) (*parser.Schema, error) {
	var merged *parser.Schema
	insertAt := 0

	for i, root := range roots {
		loader := newLoader(root, cfg)
		parsed, err := loader.Load(root.Location)
		if err != nil {
			return nil, fmt.Errorf("load parsed schema %s: %w", root.Location, err)
		}

		disambiguateSchemaOriginsForRoot(parsed, schemaRootKey(i))
		if i == 0 {
			merged = parsed
			insertAt = len(merged.GlobalDecls)
			continue
		}

		kind := Import
		if parsed.TargetNamespace == merged.TargetNamespace {
			kind = Include
		}
		if err := Apply(merged, parsed, kind, KeepNamespace, insertAt); err != nil {
			return nil, fmt.Errorf("merge schema %s: %w", root.Location, err)
		}
		insertAt = len(merged.GlobalDecls)
	}

	if merged == nil {
		return nil, fmt.Errorf("no schema roots loaded")
	}
	return merged, nil
}

func newLoader(root Root, cfg LoadConfig) *Loader {
	return NewLoader(LoaderConfig{
		FS:                          root.FS,
		Resolver:                    root.Resolver,
		AllowMissingImportLocations: cfg.AllowMissingImportLocations,
		SchemaParseOptions:          cfg.SchemaParseOptions,
		DocumentPool:                parser.NewDocumentPool(),
	})
}

func schemaRootKey(index int) string {
	return "root:" + strconv.Itoa(index)
}

func disambiguateSchemaOriginsForRoot(sch *parser.Schema, rootKey string) {
	if sch == nil || rootKey == "" {
		return
	}

	sch.Location = remapOriginKey(rootKey, sch.Location)
	remapOriginMap(sch.ElementOrigins, rootKey)
	remapOriginMap(sch.TypeOrigins, rootKey)
	remapOriginMap(sch.AttributeOrigins, rootKey)
	remapOriginMap(sch.AttributeGroupOrigins, rootKey)
	remapOriginMap(sch.GroupOrigins, rootKey)
	remapOriginMap(sch.NotationOrigins, rootKey)
	sch.ImportContexts = remapImportContexts(sch.ImportContexts, rootKey)
}

func remapOriginKey(rootKey, key string) string {
	if key == "" {
		return ""
	}
	location := parser.ImportContextLocation(key)
	return parser.ImportContextKey(rootKey, location)
}

func remapOriginMap[K comparable](origins map[K]string, rootKey string) {
	for key, value := range origins {
		origins[key] = remapOriginKey(rootKey, value)
	}
}

func remapImportContexts(contexts map[string]parser.ImportContext, rootKey string) map[string]parser.ImportContext {
	if len(contexts) == 0 {
		return contexts
	}

	remapped := make(map[string]parser.ImportContext, len(contexts))
	for key, ctx := range contexts {
		newKey := remapOriginKey(rootKey, key)
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
