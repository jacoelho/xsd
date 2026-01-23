package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type loadSession struct {
	loader *SchemaLoader
	absLoc string
	ctx    fsContext
	key    loadKey
}

func newLoadSession(loader *SchemaLoader, absLoc string, ctx fsContext, key loadKey) *loadSession {
	return &loadSession{
		loader: loader,
		absLoc: absLoc,
		ctx:    ctx,
		key:    key,
	}
}

func (s *loadSession) handleCircularLoad() (*parser.Schema, error) {
	if !s.loader.state.loading[s.key] {
		return nil, nil
	}

	if schema, ok := s.loader.state.loaded[s.key]; ok {
		return schema, nil
	}

	inProgress, ok := s.loader.state.loadingSchemas[s.key]
	if !ok || inProgress == nil {
		return nil, fmt.Errorf("circular dependency detected: %s", s.absLoc)
	}

	importingNS, ok := s.importingNamespaceFor(s.key)
	if !ok {
		return nil, fmt.Errorf("circular dependency detected: %s", s.absLoc)
	}
	if importingNS == string(inProgress.TargetNamespace) {
		return nil, fmt.Errorf("circular dependency detected: %s", s.absLoc)
	}

	return inProgress, nil
}

func (s *loadSession) parseSchema() (result *parser.ParseResult, err error) {
	f, err := s.loader.openFile(s.ctx.fs, s.absLoc)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", s.absLoc, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", s.absLoc, closeErr)
		}
	}()

	result, err = parser.ParseWithImports(f)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.absLoc, err)
	}

	return result, nil
}

func (s *loadSession) importingNamespaceFor(key loadKey) (string, bool) {
	return s.loader.imports.namespaceFor(key.location, key.fsKey)
}

func (s *loadSession) processIncludes(schema *parser.Schema, includes []parser.IncludeInfo) error {
	for _, include := range includes {
		includeLoc, err := s.loader.resolveIncludeLocation(s.absLoc, include.SchemaLocation)
		if err != nil {
			if isNotFound(err) {
				// per XSD 1.0, failure to resolve schemaLocation is not an error.
				continue
			}
			return err
		}
		absIncludeLoc, err := s.loader.resolveLocation(includeLoc)
		if err != nil {
			if isNotFound(err) {
				// per XSD 1.0, failure to resolve schemaLocation is not an error.
				continue
			}
			return err
		}
		includeKey := s.loader.loadKey(s.ctx, absIncludeLoc)
		if s.loader.alreadyMergedInclude(s.key, includeKey) {
			continue
		}
		if s.loader.state.loading[includeKey] {
			inProgress := s.loader.state.loadingSchemas[includeKey]
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
		includedSchema, err := s.loader.loadWithValidation(includeLoc, skipSchemaValidation, s.ctx)
		if err != nil {
			if isNotFound(err) {
				// per XSD 1.0, failure to resolve schemaLocation is not an error.
				continue
			}
			return fmt.Errorf("load included schema %s: %w", include.SchemaLocation, err)
		}
		if !s.loader.isIncludeNamespaceCompatible(schema.TargetNamespace, includedSchema.TargetNamespace) {
			return fmt.Errorf("included schema %s has different target namespace: %s != %s",
				include.SchemaLocation, includedSchema.TargetNamespace, schema.TargetNamespace)
		}
		needsNamespaceRemap := !schema.TargetNamespace.IsEmpty() && includedSchema.TargetNamespace.IsEmpty()
		remapMode := keepNamespace
		if needsNamespaceRemap {
			remapMode = remapNamespace
		}
		if err := s.loader.mergeSchema(schema, includedSchema, mergeInclude, remapMode); err != nil {
			return fmt.Errorf("merge included schema %s: %w", include.SchemaLocation, err)
		}
		s.loader.markMergedInclude(s.key, includeKey)
	}

	return nil
}

func (s *loadSession) processImports(schema *parser.Schema, imports []parser.ImportInfo) error {
	for _, imp := range imports {
		if imp.SchemaLocation == "" {
			continue
		}
		importLoc, err := s.loader.resolveIncludeLocation(s.absLoc, imp.SchemaLocation)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return err
		}
		absImportLoc, err := s.loader.resolveLocation(importLoc)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return err
		}
		importCtx := s.loader.importFSContext(types.NamespaceURI(imp.Namespace))
		importKey := s.loader.loadKey(importCtx, absImportLoc)
		if s.loader.alreadyMergedImport(s.key, importKey) {
			continue
		}
		if s.loader.state.loading[importKey] {
			inProgress := s.loader.state.loadingSchemas[importKey]
			if inProgress == nil {
				return fmt.Errorf("circular dependency detected in import: %s", absImportLoc)
			}
			if imp.Namespace != "" && inProgress.TargetNamespace != types.NamespaceURI(imp.Namespace) {
				return fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s",
					imp.SchemaLocation, imp.Namespace, inProgress.TargetNamespace)
			}
			if imp.Namespace == "" && !inProgress.TargetNamespace.IsEmpty() {
				return fmt.Errorf("imported schema %s must have no target namespace when import namespace is omitted",
					imp.SchemaLocation)
			}
			if !schema.TargetNamespace.IsEmpty() && inProgress.TargetNamespace == schema.TargetNamespace {
				return fmt.Errorf("circular dependency detected in import: %s", absImportLoc)
			}
			s.loader.deferImport(importKey, s.key, imp.SchemaLocation, imp.Namespace)
			continue
		}
		importedSchema, err := s.loader.loadImport(importLoc, schema.TargetNamespace, importCtx)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return fmt.Errorf("load imported schema %s: %w", imp.SchemaLocation, err)
		}
		if imp.Namespace == "" {
			if !importedSchema.TargetNamespace.IsEmpty() {
				return fmt.Errorf("imported schema %s namespace mismatch: expected no namespace, got %s",
					imp.SchemaLocation, importedSchema.TargetNamespace)
			}
		} else if importedSchema.TargetNamespace != types.NamespaceURI(imp.Namespace) {
			return fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s",
				imp.SchemaLocation, imp.Namespace, importedSchema.TargetNamespace)
		}
		if imp.Namespace == "" && !importedSchema.TargetNamespace.IsEmpty() {
			return fmt.Errorf("imported schema %s must have no target namespace when import namespace is omitted",
				imp.SchemaLocation)
		}
		if err := s.loader.mergeSchema(schema, importedSchema, mergeImport, keepNamespace); err != nil {
			return fmt.Errorf("merge imported schema %s: %w", imp.SchemaLocation, err)
		}
		s.loader.markMergedImport(s.key, importKey)
	}

	return nil
}
