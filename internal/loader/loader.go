package loader

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
	"github.com/jacoelho/xsd/internal/types"
)

// Config holds configuration for the schema loader
type Config struct {
	FS fs.FS

	NamespaceFS map[string]fs.FS

	BasePath string
}

// SchemaLoader loads XML schemas with import/include resolution
type SchemaLoader struct {
	config  Config
	loaded  map[string]*parser.Schema
	loading map[string]bool
	// Track in-progress schemas to validate include cycles.
	loadingSchemas map[string]*parser.Schema
	// Track import context: map[location]importingNamespace for mutual import detection
	// When schema A imports schema B, we store importContext["B"] = A's namespace
	// This allows us to detect mutual imports: if B then imports A, and A's namespace
	// differs from B's namespace, the cycle is allowed.
	importContext map[string]string
	// Track include merges per including schema to avoid duplicate merges
	// when the same schemaLocation is included multiple times.
	mergedIncludes map[string]map[string]bool
	// Track import merges per importing schema to avoid duplicate merges
	// when the same schemaLocation is imported multiple times.
	mergedImports map[string]map[string]bool
}

// NewLoader creates a new schema loader with the given configuration
func NewLoader(cfg Config) *SchemaLoader {
	if cfg.NamespaceFS == nil {
		cfg.NamespaceFS = make(map[string]fs.FS)
	}

	return &SchemaLoader{
		config:         cfg,
		loaded:         make(map[string]*parser.Schema),
		loading:        make(map[string]bool),
		loadingSchemas: make(map[string]*parser.Schema),
		importContext:  make(map[string]string),
		mergedIncludes: make(map[string]map[string]bool),
		mergedImports:  make(map[string]map[string]bool),
	}
}

// Load loads a schema from the given location
// If skipValidation is true, skips schema validation (used for included schemas that will be validated after merging)
func (l *SchemaLoader) Load(location string) (*parser.Schema, error) {
	return l.loadWithValidation(location, true)
}

// loadWithValidation loads a schema, optionally skipping validation
func (l *SchemaLoader) loadWithValidation(location string, validate bool) (*parser.Schema, error) {
	absLoc := l.resolveLocation(location)
	session := newLoadSession(l, absLoc)

	if schema, ok := l.loaded[absLoc]; ok {
		return schema, nil
	}

	loadedSchema, err := session.handleCircularLoad()
	if err != nil || loadedSchema != nil {
		return loadedSchema, err
	}

	// check import context BEFORE setting loading flag (for first-time loads)
	// this handles the case where we're loading a schema that's being imported
	// the import context will be checked later when we detect a cycle

	l.loading[absLoc] = true
	defer delete(l.loading, absLoc)

	result, err := session.parseSchema()
	if err != nil {
		return nil, err
	}

	schema := result.Schema
	initSchemaOrigins(schema, absLoc)
	l.loadingSchemas[absLoc] = schema
	defer delete(l.loadingSchemas, absLoc)
	registerImports(schema, result.Imports)

	if err := validateImportConstraints(schema, result.Imports); err != nil {
		return nil, err
	}

	if err := session.processIncludes(schema, result.Includes); err != nil {
		return nil, err
	}

	if err := session.processImports(schema, result.Imports); err != nil {
		return nil, err
	}

	// resolve group and type references only when validating the full schema.
	// included/imported schemas may reference components defined in other files
	// that are only available after merging.
	if validate {
		if err := l.resolveGroupReferences(schema); err != nil {
			return nil, fmt.Errorf("resolve group references: %w", err)
		}

		// phase 2: Resolve all type references (two-phase resolution)
		if err := resolver.ResolveTypeReferences(schema); err != nil {
			return nil, fmt.Errorf("resolve type references: %w", err)
		}

		if err := validateSchemaConstraints(schema); err != nil {
			return nil, err
		}
	}

	l.loaded[absLoc] = schema

	return schema, nil
}

// loadImport loads a schema for import, allowing mutual imports between different namespaces.
func (l *SchemaLoader) loadImport(location string, importNamespace string, currentNamespace types.NamespaceURI) (*parser.Schema, error) {
	// the location passed to loadImport is already resolved via resolveIncludeLocation
	// load will call resolveLocation on it, which might produce a different path
	// to ensure the import context key matches what Load will use, we need to resolve it the same way
	// resolve it the way Load will to get the exact key that Load will use
	absLocForContext := l.resolveLocation(location)

	// if already loaded, reuse it
	if schema, ok := l.loaded[absLocForContext]; ok {
		return schema, nil
	}
	// also check the original location in case it's stored differently
	if schema, ok := l.loaded[location]; ok {
		return schema, nil
	}

	// store the IMPORTING schema's namespace (currentNamespace), not the imported schema's namespace.
	// this allows mutual import detection: when we detect a cycle, we can check if the
	// importing schema has a different namespace than the schema being imported.
	currentNS := string(currentNamespace)
	if currentNS != "" {
		clearImportContext := l.trackImportContext(absLocForContext, location, currentNS)
		defer clearImportContext()
	}

	// normal loading - skip validation for imported schemas.
	// they will be validated after merging into the main schema.
	return l.loadWithValidation(location, false)
}

// LoadCompiled loads and compiles a schema from the given location.
// Returns a CompiledSchema ready for schemacheck.
// This is the new multi-phase architecture: Parse → Resolve → Compile.
func (l *SchemaLoader) LoadCompiled(location string) (*grammar.CompiledSchema, error) {
	// phase 1: Parse (and load includes/imports/redefines)
	schema, err := l.Load(location)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", location, err)
	}

	// phase 2: Resolve all QName references
	resolver := resolver.NewResolver(schema)
	if err := resolver.Resolve(); err != nil {
		return nil, fmt.Errorf("resolve %s: %w", location, err)
	}

	// phase 3: Compile to grammar
	compiler := compiler.NewCompiler(schema)
	compiled, err := compiler.Compile()
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", location, err)
	}

	compiled.SourceFS = l.config.FS
	compiled.BasePath = l.config.BasePath

	return compiled, nil
}

// GetLoaded returns a loaded schema by location, if it exists
func (l *SchemaLoader) GetLoaded(location string) (*parser.Schema, bool) {
	absLoc := l.resolveLocation(location)
	schema, ok := l.loaded[absLoc]
	return schema, ok
}

func (l *SchemaLoader) resolveLocation(location string) string {
	if l.config.BasePath == "" {
		return location
	}
	if path.IsAbs(location) {
		return location
	}
	cleanBase := path.Clean(l.config.BasePath)
	cleanLoc := path.Clean(location)
	if cleanLoc == cleanBase || strings.HasPrefix(cleanLoc, cleanBase+"/") {
		return cleanLoc
	}
	return path.Join(cleanBase, cleanLoc)
}

// resolveIncludeLocation resolves an include/import location relative to a base location
func (l *SchemaLoader) resolveIncludeLocation(baseLoc, includeLoc string) string {
	// if include location is absolute, use it as-is
	if path.IsAbs(includeLoc) {
		return includeLoc
	}
	// otherwise, resolve relative to the base location's directory
	baseDir := path.Dir(baseLoc)
	return path.Join(baseDir, includeLoc)
}

func (l *SchemaLoader) trackImportContext(resolvedLocation, originalLocation, namespace string) func() {
	l.importContext[resolvedLocation] = namespace
	if resolvedLocation != originalLocation {
		l.importContext[originalLocation] = namespace
	}

	return func() {
		delete(l.importContext, resolvedLocation)
		if resolvedLocation != originalLocation {
			delete(l.importContext, originalLocation)
		}
	}
}

func (l *SchemaLoader) alreadyMergedInclude(baseLoc, includeLoc string) bool {
	merged, ok := l.mergedIncludes[baseLoc]
	if !ok {
		return false
	}
	return merged[includeLoc]
}

func (l *SchemaLoader) markMergedInclude(baseLoc, includeLoc string) {
	if l.mergedIncludes[baseLoc] == nil {
		l.mergedIncludes[baseLoc] = make(map[string]bool)
	}
	l.mergedIncludes[baseLoc][includeLoc] = true
}

func (l *SchemaLoader) alreadyMergedImport(baseLoc, importLoc string) bool {
	merged, ok := l.mergedImports[baseLoc]
	if !ok {
		return false
	}
	return merged[importLoc]
}

func (l *SchemaLoader) markMergedImport(baseLoc, importLoc string) {
	if l.mergedImports[baseLoc] == nil {
		l.mergedImports[baseLoc] = make(map[string]bool)
	}
	l.mergedImports[baseLoc][importLoc] = true
}

func registerImports(sch *parser.Schema, imports []parser.ImportInfo) {
	if sch == nil {
		return
	}
	if sch.ImportedNamespaces == nil {
		sch.ImportedNamespaces = make(map[types.NamespaceURI]map[types.NamespaceURI]bool)
	}
	fromNS := sch.TargetNamespace
	if _, ok := sch.ImportedNamespaces[fromNS]; !ok {
		sch.ImportedNamespaces[fromNS] = make(map[types.NamespaceURI]bool)
	}
	for _, imp := range imports {
		if imp.Namespace == "" {
			continue
		}
		sch.ImportedNamespaces[fromNS][types.NamespaceURI(imp.Namespace)] = true
	}

	if sch.ImportContexts == nil {
		sch.ImportContexts = make(map[string]parser.ImportContext)
	}
	if sch.Location != "" {
		ctx := sch.ImportContexts[sch.Location]
		if ctx.Imports == nil {
			ctx.Imports = make(map[types.NamespaceURI]bool)
		}
		ctx.TargetNamespace = sch.TargetNamespace
		for _, imp := range imports {
			if imp.Namespace == "" {
				continue
			}
			ctx.Imports[types.NamespaceURI(imp.Namespace)] = true
		}
		sch.ImportContexts[sch.Location] = ctx
	}
}

func validateImportConstraints(schema *parser.Schema, imports []parser.ImportInfo) error {
	if schema.TargetNamespace.IsEmpty() {
		for _, imp := range imports {
			if imp.Namespace == "" {
				return fmt.Errorf("schema without targetNamespace cannot use import without namespace attribute (namespace attribute is required)")
			}
		}
	}
	for _, imp := range imports {
		if imp.Namespace == "" {
			continue
		}
		if !schema.TargetNamespace.IsEmpty() && types.NamespaceURI(imp.Namespace) == schema.TargetNamespace {
			return fmt.Errorf("import namespace %s must be different from target namespace", imp.Namespace)
		}
	}
	return nil
}

func validateSchemaConstraints(schema *parser.Schema) error {
	validationErrors := ValidateSchema(schema)
	if len(validationErrors) == 0 {
		return nil
	}
	var errMsg strings.Builder
	errMsg.WriteString("schema validation failed:")
	for _, err := range validationErrors {
		errMsg.WriteString("\n  - ")
		errMsg.WriteString(err.Error())
	}
	return errors.New(errMsg.String())
}

func initSchemaOrigins(schema *parser.Schema, location string) {
	if schema == nil {
		return
	}
	schema.Location = location
	for qname := range schema.ElementDecls {
		if schema.ElementOrigins[qname] == "" {
			schema.ElementOrigins[qname] = location
		}
	}
	for qname := range schema.TypeDefs {
		if schema.TypeOrigins[qname] == "" {
			schema.TypeOrigins[qname] = location
		}
	}
	for qname := range schema.AttributeDecls {
		if schema.AttributeOrigins[qname] == "" {
			schema.AttributeOrigins[qname] = location
		}
	}
	for qname := range schema.AttributeGroups {
		if schema.AttributeGroupOrigins[qname] == "" {
			schema.AttributeGroupOrigins[qname] = location
		}
	}
	for qname := range schema.Groups {
		if schema.GroupOrigins[qname] == "" {
			schema.GroupOrigins[qname] = location
		}
	}
	for qname := range schema.NotationDecls {
		if schema.NotationOrigins[qname] == "" {
			schema.NotationOrigins[qname] = location
		}
	}
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err)
}

// isIncludeNamespaceCompatible checks if target namespaces are compatible for include
// Rules according to XSD 1.0 spec:
// - If including schema has a target namespace, included schema must have the same namespace OR no namespace
// - If including schema has no target namespace, included schema must also have no target namespace
func (l *SchemaLoader) isIncludeNamespaceCompatible(includingNS, includedNS types.NamespaceURI) bool {
	// same namespace - always compatible
	if includingNS == includedNS {
		return true
	}
	// including schema has namespace, included schema has no namespace - compatible
	if !includingNS.IsEmpty() && includedNS.IsEmpty() {
		return true
	}
	// all other cases are incompatible
	return false
}

func (l *SchemaLoader) openFile(location string) (io.ReadCloser, error) {
	if l.config.FS == nil {
		return nil, fmt.Errorf("no filesystem configured")
	}

	f, err := l.config.FS.Open(location)
	if err != nil {
		return nil, err
	}

	return f, nil
}
