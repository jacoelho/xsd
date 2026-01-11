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

type loadState struct {
	loaded         map[string]*parser.Schema
	loading        map[string]bool
	loadingSchemas map[string]*parser.Schema
}

func newLoadState() loadState {
	return loadState{
		loaded:         make(map[string]*parser.Schema),
		loading:        make(map[string]bool),
		loadingSchemas: make(map[string]*parser.Schema),
	}
}

type importTracker struct {
	context        map[string]string
	mergedIncludes map[string]map[string]bool
	mergedImports  map[string]map[string]bool
}

func newImportTracker() importTracker {
	return importTracker{
		context:        make(map[string]string),
		mergedIncludes: make(map[string]map[string]bool),
		mergedImports:  make(map[string]map[string]bool),
	}
}

func (t *importTracker) trackContext(resolvedLocation, originalLocation, namespace string) func() {
	t.context[resolvedLocation] = namespace
	if resolvedLocation != originalLocation {
		t.context[originalLocation] = namespace
	}

	return func() {
		delete(t.context, resolvedLocation)
		if resolvedLocation != originalLocation {
			delete(t.context, originalLocation)
		}
	}
}

func (t *importTracker) namespaceFor(location string) (string, bool) {
	if ns, ok := t.context[location]; ok {
		return ns, true
	}

	locationBase := path.Base(location)
	if ns, ok := t.context[locationBase]; ok {
		return ns, true
	}

	for loc, ns := range t.context {
		if strings.HasSuffix(loc, locationBase) || strings.HasSuffix(location, path.Base(loc)) {
			return ns, true
		}
	}

	return "", false
}

func (t *importTracker) alreadyMergedInclude(baseLoc, includeLoc string) bool {
	merged, ok := t.mergedIncludes[baseLoc]
	if !ok {
		return false
	}
	return merged[includeLoc]
}

func (t *importTracker) markMergedInclude(baseLoc, includeLoc string) {
	if t.mergedIncludes[baseLoc] == nil {
		t.mergedIncludes[baseLoc] = make(map[string]bool)
	}
	t.mergedIncludes[baseLoc][includeLoc] = true
}

func (t *importTracker) alreadyMergedImport(baseLoc, importLoc string) bool {
	merged, ok := t.mergedImports[baseLoc]
	if !ok {
		return false
	}
	return merged[importLoc]
}

func (t *importTracker) markMergedImport(baseLoc, importLoc string) {
	if t.mergedImports[baseLoc] == nil {
		t.mergedImports[baseLoc] = make(map[string]bool)
	}
	t.mergedImports[baseLoc][importLoc] = true
}

type validationMode int

const (
	validateSchema validationMode = iota
	skipSchemaValidation
)

// SchemaLoader loads XML schemas with import/include resolution
type SchemaLoader struct {
	state   loadState
	imports importTracker
	config  Config
}

// NewLoader creates a new schema loader with the given configuration
func NewLoader(cfg Config) *SchemaLoader {
	if cfg.NamespaceFS == nil {
		cfg.NamespaceFS = make(map[string]fs.FS)
	}

	return &SchemaLoader{
		config:  cfg,
		state:   newLoadState(),
		imports: newImportTracker(),
	}
}

// Load loads a schema from the given location and validates it.
func (l *SchemaLoader) Load(location string) (*parser.Schema, error) {
	return l.loadWithValidation(location, validateSchema)
}

// loadWithValidation loads a schema with the requested validation mode.
func (l *SchemaLoader) loadWithValidation(location string, mode validationMode) (*parser.Schema, error) {
	absLoc := l.resolveLocation(location)
	session := newLoadSession(l, absLoc)

	if schema, ok := l.state.loaded[absLoc]; ok {
		return schema, nil
	}

	loadedSchema, err := session.handleCircularLoad()
	if err != nil || loadedSchema != nil {
		return loadedSchema, err
	}

	// check import context BEFORE setting loading flag (for first-time loads)
	// this handles the case where we're loading a schema that's being imported
	// the import context will be checked later when we detect a cycle

	l.state.loading[absLoc] = true
	defer delete(l.state.loading, absLoc)

	result, err := session.parseSchema()
	if err != nil {
		return nil, err
	}

	schema := result.Schema
	initSchemaOrigins(schema, absLoc)
	l.state.loadingSchemas[absLoc] = schema
	defer delete(l.state.loadingSchemas, absLoc)
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
	if mode == validateSchema {
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

	l.state.loaded[absLoc] = schema

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
	if schema, ok := l.state.loaded[absLocForContext]; ok {
		return schema, nil
	}
	// also check the original location in case it's stored differently
	if schema, ok := l.state.loaded[location]; ok {
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
	return l.loadWithValidation(location, skipSchemaValidation)
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

	return compiled, nil
}

// GetLoaded returns a loaded schema by location, if it exists
func (l *SchemaLoader) GetLoaded(location string) (*parser.Schema, bool) {
	absLoc := l.resolveLocation(location)
	schema, ok := l.state.loaded[absLoc]
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
	return l.imports.trackContext(resolvedLocation, originalLocation, namespace)
}

func (l *SchemaLoader) alreadyMergedInclude(baseLoc, includeLoc string) bool {
	return l.imports.alreadyMergedInclude(baseLoc, includeLoc)
}

func (l *SchemaLoader) markMergedInclude(baseLoc, includeLoc string) {
	l.imports.markMergedInclude(baseLoc, includeLoc)
}

func (l *SchemaLoader) alreadyMergedImport(baseLoc, importLoc string) bool {
	return l.imports.alreadyMergedImport(baseLoc, importLoc)
}

func (l *SchemaLoader) markMergedImport(baseLoc, importLoc string) {
	l.imports.markMergedImport(baseLoc, importLoc)
}

func ensureNamespaceMap(m map[types.NamespaceURI]map[types.NamespaceURI]bool, key types.NamespaceURI) map[types.NamespaceURI]bool {
	if m[key] == nil {
		m[key] = make(map[types.NamespaceURI]bool)
	}
	return m[key]
}

func registerImports(sch *parser.Schema, imports []parser.ImportInfo) {
	if sch == nil {
		return
	}
	if sch.ImportedNamespaces == nil {
		sch.ImportedNamespaces = make(map[types.NamespaceURI]map[types.NamespaceURI]bool)
	}
	fromNS := sch.TargetNamespace
	imported := ensureNamespaceMap(sch.ImportedNamespaces, fromNS)
	for _, imp := range imports {
		if imp.Namespace == "" {
			continue
		}
		imported[types.NamespaceURI(imp.Namespace)] = true
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
