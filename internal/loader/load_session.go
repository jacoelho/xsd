package loader

import (
	"fmt"
	"path"
	"strings"

	schema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type loadSession struct {
	loader *SchemaLoader
	absLoc string
}

func newLoadSession(loader *SchemaLoader, absLoc string) *loadSession {
	return &loadSession{
		loader: loader,
		absLoc: absLoc,
	}
}

func (s *loadSession) handleCircularLoad() (*schema.Schema, error) {
	if !s.loader.loading[s.absLoc] {
		return nil, nil
	}

	if schema, ok := s.loader.loaded[s.absLoc]; ok {
		return schema, nil
	}

	importingNS, ok := s.importingNamespaceFor(s.absLoc)
	if !ok {
		return nil, fmt.Errorf("circular dependency detected: %s", s.absLoc)
	}

	result, err := s.parseSchema()
	if err != nil {
		return nil, err
	}

	if importingNS == string(result.Schema.TargetNamespace) {
		return nil, fmt.Errorf("circular dependency detected: %s", s.absLoc)
	}

	initSchemaOrigins(result.Schema, s.absLoc)
	return result.Schema, nil
}

func (s *loadSession) parseSchema() (result *schema.ParseResult, err error) {
	f, err := s.loader.openFile(s.absLoc)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", s.absLoc, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", s.absLoc, closeErr)
		}
	}()

	result, err = schema.ParseWithImports(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.absLoc, err)
	}

	return result, nil
}

func (s *loadSession) importingNamespaceFor(location string) (string, bool) {
	if ns, ok := s.loader.importContext[location]; ok {
		return ns, true
	}

	locationBase := path.Base(location)
	if ns, ok := s.loader.importContext[locationBase]; ok {
		return ns, true
	}

	for loc, ns := range s.loader.importContext {
		if strings.HasSuffix(loc, locationBase) || strings.HasSuffix(location, path.Base(loc)) {
			return ns, true
		}
	}

	return "", false
}

func (s *loadSession) processIncludes(schema *schema.Schema, includes []schema.IncludeInfo) error {
	for _, include := range includes {
		includeLoc := s.loader.resolveIncludeLocation(s.absLoc, include.SchemaLocation)
		absIncludeLoc := s.loader.resolveLocation(includeLoc)
		if s.loader.alreadyMergedInclude(s.absLoc, absIncludeLoc) {
			continue
		}
		if s.loader.loading[absIncludeLoc] {
			inProgress := s.loader.loadingSchemas[absIncludeLoc]
			if inProgress == nil {
				// loadingSchemas should be set before includes are processed; nil means loader state is inconsistent.
				return fmt.Errorf("circular dependency detected in include: %s", absIncludeLoc)
			}
			if !s.loader.isIncludeNamespaceCompatible(schema.TargetNamespace, inProgress.TargetNamespace) {
				return fmt.Errorf("included schema %s has different target namespace: %s != %s",
					include.SchemaLocation, inProgress.TargetNamespace, schema.TargetNamespace)
			}
			continue
		}
		includedSchema, err := s.loader.loadWithValidation(includeLoc, false)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return fmt.Errorf("load included schema %s: %w", include.SchemaLocation, err)
		}
		if !s.loader.isIncludeNamespaceCompatible(schema.TargetNamespace, includedSchema.TargetNamespace) {
			return fmt.Errorf("included schema %s has different target namespace: %s != %s",
				include.SchemaLocation, includedSchema.TargetNamespace, schema.TargetNamespace)
		}
		needsNamespaceRemap := !schema.TargetNamespace.IsEmpty() && includedSchema.TargetNamespace.IsEmpty()
		if err := s.loader.mergeSchema(schema, includedSchema, false, needsNamespaceRemap); err != nil {
			return fmt.Errorf("merge included schema %s: %w", include.SchemaLocation, err)
		}
		s.loader.markMergedInclude(s.absLoc, absIncludeLoc)
	}

	return nil
}

func (s *loadSession) processImports(schema *schema.Schema, imports []schema.ImportInfo) error {
	for _, imp := range imports {
		if imp.SchemaLocation == "" {
			continue
		}
		importLoc := s.loader.resolveIncludeLocation(s.absLoc, imp.SchemaLocation)
		absImportLoc := s.loader.resolveLocation(importLoc)
		if s.loader.alreadyMergedImport(s.absLoc, absImportLoc) {
			continue
		}
		importedSchema, err := s.loader.loadImport(importLoc, imp.Namespace, schema.TargetNamespace)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return fmt.Errorf("load imported schema %s: %w", imp.SchemaLocation, err)
		}
		if imp.Namespace != "" && importedSchema.TargetNamespace != types.NamespaceURI(imp.Namespace) {
			return fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s",
				imp.SchemaLocation, imp.Namespace, importedSchema.TargetNamespace)
		}
		if err := s.loader.mergeSchema(schema, importedSchema, true, false); err != nil {
			return fmt.Errorf("merge imported schema %s: %w", imp.SchemaLocation, err)
		}
		s.loader.markMergedImport(s.absLoc, absImportLoc)
	}

	return nil
}
