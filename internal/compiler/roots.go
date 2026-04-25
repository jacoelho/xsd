package compiler

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jacoelho/xsd/internal/schemaast"
)

// PrepareRoots loads one or more schema roots and normalizes them for runtime builds.
func PrepareRoots(cfg LoadConfig) (*Prepared, error) {
	docs, err := loadRoots(cfg)
	if err != nil {
		return nil, err
	}
	return Prepare(docs)
}

func loadRoots(cfg LoadConfig) (*schemaast.DocumentSet, error) {
	roots, err := normalizeRoots(cfg.Roots)
	if err != nil {
		return nil, err
	}
	if len(roots) == 1 {
		root := roots[0]
		loader := newLoader(root, cfg)
		docs, err := loader.LoadDocuments(root.Location)
		if err != nil {
			return nil, fmt.Errorf("load parsed schema: %w", err)
		}
		return docs, nil
	}
	return loadDocumentRoots(roots, cfg)
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

func loadDocumentRoots(roots []Root, cfg LoadConfig) (*schemaast.DocumentSet, error) {
	seen := make(map[documentRootKey][]schemaast.SchemaDocument)
	var out schemaast.DocumentSet

	for i, root := range roots {
		loader := newLoader(root, cfg)
		docs, err := loader.LoadDocuments(root.Location)
		if err != nil {
			return nil, fmt.Errorf("load parsed schema %s: %w", root.Location, err)
		}
		for _, doc := range docs.Documents {
			key := documentRootKey{
				location:  doc.Location,
				namespace: doc.TargetNamespace,
			}
			if key.location == "" {
				key.location = fmt.Sprintf("root:%d", i)
			}
			if documentSeen(seen[key], doc) {
				continue
			}
			seen[key] = append(seen[key], doc)
			out.Documents = append(out.Documents, doc)
		}
	}

	if len(out.Documents) == 0 {
		return nil, fmt.Errorf("no schema roots loaded")
	}
	return &out, nil
}

type documentRootKey struct {
	location  string
	namespace schemaast.NamespaceURI
}

func documentSeen(seen []schemaast.SchemaDocument, doc schemaast.SchemaDocument) bool {
	for _, existing := range seen {
		if reflect.DeepEqual(existing, doc) {
			return true
		}
	}
	return false
}

func newLoader(root Root, cfg LoadConfig) *Loader {
	return NewLoader(LoaderConfig{
		FS:                          root.FS,
		Resolver:                    root.Resolver,
		AllowMissingImportLocations: cfg.AllowMissingImportLocations,
		SchemaParseOptions:          cfg.SchemaParseOptions,
		DocumentPool:                schemaast.NewDocumentPool(),
	})
}
