package loader

import (
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	schemadet "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

type mergeKind int

const (
	mergeInclude mergeKind = iota
	mergeImport
)

type namespaceRemapMode int

const (
	remapNamespace namespaceRemapMode = iota
	keepNamespace
)

type globalDeclKey struct {
	name types.QName
	kind parser.GlobalDeclKind
}

type mergeContext struct {
	target              *parser.Schema
	source              *parser.Schema
	remapQName          func(types.QName) types.QName
	opts                types.CopyOptions
	isImport            bool
	needsNamespaceRemap bool
}

func newMergeContext(target, source *parser.Schema, kind mergeKind, remap namespaceRemapMode) mergeContext {
	isImport := kind == mergeImport
	needsNamespaceRemap := remap == remapNamespace
	remapQName := func(qname types.QName) types.QName {
		if needsNamespaceRemap && qname.Namespace.IsEmpty() {
			return types.QName{
				Namespace: target.TargetNamespace,
				Local:     qname.Local,
			}
		}
		return qname
	}

	sourceNamespace := source.TargetNamespace
	if !isImport && needsNamespaceRemap {
		sourceNamespace = target.TargetNamespace
	}

	opts := types.CopyOptions{
		SourceNamespace: sourceNamespace,
		RemapQName:      remapQName,
	}

	return mergeContext{
		target:              target,
		source:              source,
		isImport:            isImport,
		needsNamespaceRemap: needsNamespaceRemap,
		remapQName:          remapQName,
		opts:                opts,
	}
}

// mergeSchema merges a source schema into a target schema.
// For imports, preserves source namespace.
// For includes, uses chameleon namespace remapping if needed.
func (l *SchemaLoader) mergeSchema(target, source *parser.Schema, kind mergeKind, remap namespaceRemapMode, insertAt int) error {
	staging := cloneSchemaForMerge(target)
	ctx := newMergeContext(staging, source, kind, remap)
	existingDecls := existingGlobalDecls(staging)
	ctx.mergeImportedNamespaces()
	ctx.mergeImportContexts()
	if err := ctx.mergeElementDecls(); err != nil {
		return err
	}
	if err := ctx.mergeTypeDefs(); err != nil {
		return err
	}
	if err := ctx.mergeAttributeDecls(); err != nil {
		return err
	}
	if err := ctx.mergeAttributeGroups(); err != nil {
		return err
	}
	if err := ctx.mergeGroups(); err != nil {
		return err
	}
	ctx.mergeSubstitutionGroups()
	if err := ctx.mergeNotationDecls(); err != nil {
		return err
	}
	if err := ctx.mergeIDAttributes(); err != nil {
		return err
	}
	ctx.mergeGlobalDecls(existingDecls, insertAt)
	staging.HasPlaceholders = staging.HasPlaceholders || source.HasPlaceholders
	*target = *staging
	return nil
}

func cloneSchemaForMerge(schema *parser.Schema) *parser.Schema {
	clone := *schema
	clone.ImportContexts = copyImportContexts(schema.ImportContexts)
	clone.ImportedNamespaces = copyImportedNamespaces(schema.ImportedNamespaces)
	clone.ElementDecls = cloneMap(schema.ElementDecls)
	clone.ElementOrigins = cloneMap(schema.ElementOrigins)
	clone.TypeDefs = cloneMap(schema.TypeDefs)
	clone.TypeOrigins = cloneMap(schema.TypeOrigins)
	clone.AttributeDecls = cloneMap(schema.AttributeDecls)
	clone.AttributeOrigins = cloneMap(schema.AttributeOrigins)
	clone.AttributeGroups = cloneMap(schema.AttributeGroups)
	clone.AttributeGroupOrigins = cloneMap(schema.AttributeGroupOrigins)
	clone.Groups = cloneMap(schema.Groups)
	clone.GroupOrigins = cloneMap(schema.GroupOrigins)
	clone.SubstitutionGroups = copyQNameSliceMap(schema.SubstitutionGroups)
	clone.NotationDecls = cloneMap(schema.NotationDecls)
	clone.NotationOrigins = cloneMap(schema.NotationOrigins)
	clone.IDAttributes = cloneMap(schema.IDAttributes)
	clone.GlobalDecls = append([]parser.GlobalDecl(nil), schema.GlobalDecls...)
	return &clone
}

func cloneMap[K comparable, V any](src map[K]V) map[K]V {
	if src == nil {
		return nil
	}
	dst := make(map[K]V, len(src))
	maps.Copy(dst, src)
	return dst
}

func copyImportContexts(src map[string]parser.ImportContext) map[string]parser.ImportContext {
	if src == nil {
		return nil
	}
	dst := make(map[string]parser.ImportContext, len(src))
	for key, ctx := range src {
		copied := ctx
		if ctx.Imports != nil {
			imports := make(map[types.NamespaceURI]bool, len(ctx.Imports))
			for ns := range ctx.Imports {
				imports[ns] = true
			}
			copied.Imports = imports
		} else {
			copied.Imports = nil
		}
		dst[key] = copied
	}
	return dst
}

func copyImportedNamespaces(src map[types.NamespaceURI]map[types.NamespaceURI]bool) map[types.NamespaceURI]map[types.NamespaceURI]bool {
	if src == nil {
		return nil
	}
	dst := make(map[types.NamespaceURI]map[types.NamespaceURI]bool, len(src))
	for ns, imports := range src {
		if imports == nil {
			dst[ns] = nil
			continue
		}
		copied := make(map[types.NamespaceURI]bool, len(imports))
		for imported := range imports {
			copied[imported] = true
		}
		dst[ns] = copied
	}
	return dst
}

func copyQNameSliceMap(src map[types.QName][]types.QName) map[types.QName][]types.QName {
	if src == nil {
		return nil
	}
	dst := make(map[types.QName][]types.QName, len(src))
	for key, value := range src {
		if value == nil {
			dst[key] = nil
			continue
		}
		copied := make([]types.QName, len(value))
		copy(copied, value)
		dst[key] = copied
	}
	return dst
}

func existingGlobalDecls(schema *parser.Schema) map[globalDeclKey]struct{} {
	decls := make(map[globalDeclKey]struct{}, len(schema.GlobalDecls))
	for _, decl := range schema.GlobalDecls {
		decls[globalDeclKey{kind: decl.Kind, name: decl.Name}] = struct{}{}
	}
	return decls
}

func (c *mergeContext) mergeGlobalDecls(existing map[globalDeclKey]struct{}, insertAt int) {
	if c.source.GlobalDecls == nil {
		return
	}
	newDecls := make([]parser.GlobalDecl, 0, len(c.source.GlobalDecls))
	for _, decl := range c.source.GlobalDecls {
		mappedName := c.remapQName(decl.Name)
		key := globalDeclKey{kind: decl.Kind, name: mappedName}
		if _, seen := existing[key]; seen {
			continue
		}
		if !c.globalDeclExists(decl.Kind, mappedName) {
			continue
		}
		newDecls = append(newDecls, parser.GlobalDecl{
			Kind: decl.Kind,
			Name: mappedName,
		})
		existing[key] = struct{}{}
	}
	if len(newDecls) == 0 {
		return
	}
	if insertAt < 0 || insertAt > len(c.target.GlobalDecls) {
		insertAt = len(c.target.GlobalDecls)
	}
	c.target.GlobalDecls = insertGlobalDecls(c.target.GlobalDecls, insertAt, newDecls)
}

func insertGlobalDecls(dst []parser.GlobalDecl, insertAt int, insert []parser.GlobalDecl) []parser.GlobalDecl {
	if len(insert) == 0 {
		return dst
	}
	if insertAt < 0 || insertAt > len(dst) {
		insertAt = len(dst)
	}
	merged := make([]parser.GlobalDecl, 0, len(dst)+len(insert))
	merged = append(merged, dst[:insertAt]...)
	merged = append(merged, insert...)
	merged = append(merged, dst[insertAt:]...)
	return merged
}

func (c *mergeContext) globalDeclExists(kind parser.GlobalDeclKind, name types.QName) bool {
	switch kind {
	case parser.GlobalDeclElement:
		_, ok := c.target.ElementDecls[name]
		return ok
	case parser.GlobalDeclType:
		_, ok := c.target.TypeDefs[name]
		return ok
	case parser.GlobalDeclAttribute:
		_, ok := c.target.AttributeDecls[name]
		return ok
	case parser.GlobalDeclAttributeGroup:
		_, ok := c.target.AttributeGroups[name]
		return ok
	case parser.GlobalDeclGroup:
		_, ok := c.target.Groups[name]
		return ok
	case parser.GlobalDeclNotation:
		_, ok := c.target.NotationDecls[name]
		return ok
	default:
		return false
	}
}

func mergeNamed[V any](
	source map[types.QName]V,
	target map[types.QName]V,
	targetOrigins map[types.QName]string,
	remap func(types.QName) types.QName,
	originFor func(types.QName) string,
	insert func(V) V,
	candidate func(V) V,
	equivalent func(existing V, candidate V) bool,
	kindName string,
) error {
	if insert == nil {
		insert = func(value V) V { return value }
	}
	for _, qname := range schemadet.SortedQNames(source) {
		value := source[qname]
		targetQName := remap(qname)
		origin := originFor(qname)
		if existing, exists := target[targetQName]; exists {
			if targetOrigins[targetQName] == origin {
				continue
			}
			if equivalent != nil {
				cand := value
				if candidate != nil {
					cand = candidate(value)
				}
				if equivalent(existing, cand) {
					continue
				}
			}
			return fmt.Errorf("duplicate %s %s", kindName, targetQName)
		}
		target[targetQName] = insert(value)
		targetOrigins[targetQName] = origin
	}
	return nil
}

func (c *mergeContext) mergeImportedNamespaces() {
	if c.source.ImportedNamespaces == nil {
		return
	}
	if c.target.ImportedNamespaces == nil {
		c.target.ImportedNamespaces = make(map[types.NamespaceURI]map[types.NamespaceURI]bool)
	}
	for fromNS, imports := range c.source.ImportedNamespaces {
		mappedFrom := fromNS
		if c.needsNamespaceRemap && fromNS.IsEmpty() {
			mappedFrom = c.target.TargetNamespace
		}
		if _, ok := c.target.ImportedNamespaces[mappedFrom]; !ok {
			c.target.ImportedNamespaces[mappedFrom] = make(map[types.NamespaceURI]bool)
		}
		for ns := range imports {
			c.target.ImportedNamespaces[mappedFrom][ns] = true
		}
	}
}

func (c *mergeContext) mergeImportContexts() {
	if c.source.ImportContexts == nil {
		return
	}
	if c.target.ImportContexts == nil {
		c.target.ImportContexts = make(map[string]parser.ImportContext)
	}
	for location, ctx := range c.source.ImportContexts {
		merged := ctx
		if c.needsNamespaceRemap && merged.TargetNamespace.IsEmpty() {
			merged.TargetNamespace = c.target.TargetNamespace
		}
		if merged.Imports == nil {
			merged.Imports = make(map[types.NamespaceURI]bool)
		}
		if existing, ok := c.target.ImportContexts[location]; ok {
			if existing.Imports == nil {
				existing.Imports = make(map[types.NamespaceURI]bool)
			}
			for ns := range merged.Imports {
				existing.Imports[ns] = true
			}
			if existing.TargetNamespace.IsEmpty() {
				existing.TargetNamespace = merged.TargetNamespace
			}
			c.target.ImportContexts[location] = existing
			continue
		}
		c.target.ImportContexts[location] = merged
	}
}

func (c *mergeContext) mergeElementDecls() error {
	return mergeNamed(
		c.source.ElementDecls,
		c.target.ElementDecls,
		c.target.ElementOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.ElementOrigins, qname) },
		c.elementDeclForInsert,
		c.elementDeclCandidate,
		elementDeclEquivalent,
		"element declaration",
	)
}

func (c *mergeContext) elementDeclCandidate(decl *types.ElementDecl) *types.ElementDecl {
	if c.isImport {
		declCopy := *decl
		declCopy.Name = c.remapQName(decl.Name)
		declCopy.SourceNamespace = c.source.TargetNamespace
		return &declCopy
	}
	if c.needsNamespaceRemap {
		return decl.Copy(c.opts)
	}
	return decl
}

func (c *mergeContext) elementDeclForInsert(decl *types.ElementDecl) *types.ElementDecl {
	if c.isImport {
		declCopy := *decl
		declCopy.Name = c.remapQName(decl.Name)
		declCopy.SourceNamespace = c.source.TargetNamespace
		return &declCopy
	}
	return decl.Copy(c.opts)
}

func (c *mergeContext) mergeTypeDefs() error {
	insert := func(typ types.Type) types.Type {
		if c.isImport {
			return c.copyTypeForImport(typ)
		}
		return c.copyTypeForInclude(typ)
	}
	return mergeNamed(
		c.source.TypeDefs,
		c.target.TypeDefs,
		c.target.TypeOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.TypeOrigins, qname) },
		insert,
		nil,
		nil,
		"type definition",
	)
}

func (c *mergeContext) copyTypeForImport(typ types.Type) types.Type {
	copied := types.CopyType(typ, c.opts)
	if complexType, ok := copied.(*types.ComplexType); ok {
		normalizeAttributeForms(complexType, c.source.AttributeFormDefault)
	}
	return copied
}

func (c *mergeContext) copyTypeForInclude(typ types.Type) types.Type {
	copiedType := types.CopyType(typ, c.opts)
	if complexType, ok := copiedType.(*types.ComplexType); ok {
		normalizeAttributeForms(complexType, c.source.AttributeFormDefault)
	}
	return copiedType
}

func (c *mergeContext) mergeAttributeDecls() error {
	return mergeNamed(
		c.source.AttributeDecls,
		c.target.AttributeDecls,
		c.target.AttributeOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.AttributeOrigins, qname) },
		c.copyAttributeDecl,
		nil,
		nil,
		"attribute declaration",
	)
}

func (c *mergeContext) copyAttributeDecl(decl *types.AttributeDecl) *types.AttributeDecl {
	if c.isImport {
		declCopy := *decl
		declCopy.Name = c.remapQName(decl.Name)
		declCopy.SourceNamespace = c.source.TargetNamespace
		return &declCopy
	}
	return decl.Copy(c.opts)
}

func (c *mergeContext) mergeAttributeGroups() error {
	insert := func(group *types.AttributeGroup) *types.AttributeGroup {
		groupCopy := group.Copy(c.opts)
		for _, attr := range groupCopy.Attributes {
			if attr.Form == types.FormDefault {
				if c.source.AttributeFormDefault == parser.Qualified {
					attr.Form = types.FormQualified
				} else {
					attr.Form = types.FormUnqualified
				}
			}
		}
		return groupCopy
	}
	return mergeNamed(
		c.source.AttributeGroups,
		c.target.AttributeGroups,
		c.target.AttributeGroupOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.AttributeGroupOrigins, qname) },
		insert,
		nil,
		nil,
		"attributeGroup",
	)
}

func (c *mergeContext) mergeGroups() error {
	return mergeNamed(
		c.source.Groups,
		c.target.Groups,
		c.target.GroupOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.GroupOrigins, qname) },
		func(group *types.ModelGroup) *types.ModelGroup { return group.Copy(c.opts) },
		nil,
		nil,
		"group",
	)
}

func (c *mergeContext) mergeSubstitutionGroups() {
	if c.target.SubstitutionGroups == nil {
		c.target.SubstitutionGroups = make(map[types.QName][]types.QName)
	}
	heads := make([]types.QName, 0, len(c.source.SubstitutionGroups))
	for head := range c.source.SubstitutionGroups {
		heads = append(heads, head)
	}
	sort.Slice(heads, func(i, j int) bool {
		return qnameLess(heads[i], heads[j])
	})
	for _, head := range heads {
		members := c.source.SubstitutionGroups[head]
		targetHead := c.remapQName(head)
		remappedMembers := make([]types.QName, 0, len(members))
		for _, member := range members {
			remapped := c.remapQName(member)
			remappedMembers = append(remappedMembers, remapped)
		}
		remappedMembers = sortAndDedupeQNames(remappedMembers)
		if existing, exists := c.target.SubstitutionGroups[targetHead]; exists {
			if len(remappedMembers) == 0 {
				c.target.SubstitutionGroups[targetHead] = sortAndDedupeQNames(existing)
				continue
			}
			existing = append(existing, remappedMembers...)
			c.target.SubstitutionGroups[targetHead] = sortAndDedupeQNames(existing)
			continue
		}
		if len(remappedMembers) > 0 {
			c.target.SubstitutionGroups[targetHead] = remappedMembers
		}
	}
}

func sortAndDedupeQNames(names []types.QName) []types.QName {
	if len(names) < 2 {
		return names
	}
	sort.Slice(names, func(i, j int) bool {
		return qnameLess(names[i], names[j])
	})
	out := names[:0]
	var last types.QName
	for i, name := range names {
		if i == 0 || !name.Equal(last) {
			out = append(out, name)
			last = name
		}
	}
	return out
}

func qnameLess(a, b types.QName) bool {
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	return a.Local < b.Local
}

func (c *mergeContext) mergeNotationDecls() error {
	return mergeNamed(
		c.source.NotationDecls,
		c.target.NotationDecls,
		c.target.NotationOrigins,
		c.remapQName,
		func(qname types.QName) string { return c.originFor(c.source.NotationOrigins, qname) },
		func(notation *types.NotationDecl) *types.NotationDecl { return notation.Copy(c.opts) },
		nil,
		nil,
		"notation",
	)
}

func (c *mergeContext) mergeIDAttributes() error {
	if c.target.IDAttributes == nil {
		c.target.IDAttributes = make(map[string]string)
	}
	for id, component := range c.source.IDAttributes {
		if _, exists := c.target.IDAttributes[id]; exists {
			continue
		}
		c.target.IDAttributes[id] = component
	}
	return nil
}

func (c *mergeContext) originFor(origins map[types.QName]string, qname types.QName) string {
	origin := origins[qname]
	if origin == "" {
		origin = c.source.Location
	}
	return origin
}

func elementDeclEquivalent(a, b *types.ElementDecl) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if a.Nillable != b.Nillable || a.Abstract != b.Abstract || a.SubstitutionGroup != b.SubstitutionGroup {
		return false
	}
	if a.Block != b.Block || a.Final != b.Final {
		return false
	}
	if a.HasFixed != b.HasFixed || a.HasDefault != b.HasDefault {
		return false
	}
	if a.HasFixed && a.Fixed != b.Fixed {
		return false
	}
	if a.HasDefault && a.Default != b.Default {
		return false
	}
	if a.Form != b.Form {
		return false
	}
	if !schemacheck.ElementTypesCompatible(a.Type, b.Type) {
		return false
	}
	return identityConstraintsEquivalent(a.Constraints, b.Constraints)
}

func identityConstraintsEquivalent(a, b []*types.IdentityConstraint) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	keysA := make([]string, 0, len(a))
	for _, constraint := range a {
		keysA = append(keysA, identityConstraintKey(constraint))
	}
	keysB := make([]string, 0, len(b))
	for _, constraint := range b {
		keysB = append(keysB, identityConstraintKey(constraint))
	}
	sort.Strings(keysA)
	sort.Strings(keysB)
	for i := range keysA {
		if keysA[i] != keysB[i] {
			return false
		}
	}
	return true
}

func identityConstraintKey(constraint *types.IdentityConstraint) string {
	if constraint == nil {
		return "<nil>"
	}
	var builder strings.Builder
	builder.WriteString(constraint.Name)
	builder.WriteByte('|')
	builder.WriteString(strconv.Itoa(int(constraint.Type)))
	builder.WriteByte('|')
	builder.WriteString(constraint.Selector.XPath)
	builder.WriteByte('|')
	builder.WriteString(constraint.ReferQName.String())
	builder.WriteByte('|')
	builder.WriteString(string(constraint.TargetNamespace))
	builder.WriteByte('|')
	for _, field := range constraint.Fields {
		builder.WriteString(field.XPath)
		builder.WriteByte('\x1f')
	}
	builder.WriteByte('|')
	if len(constraint.NamespaceContext) == 0 {
		return builder.String()
	}
	keys := make([]string, 0, len(constraint.NamespaceContext))
	for key := range constraint.NamespaceContext {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(constraint.NamespaceContext[key])
		builder.WriteByte(';')
	}
	return builder.String()
}

// normalizeAttributeForms explicitly sets the Form on attributes that have FormDefault
// based on the source schema's attributeFormDefault. This ensures that when types from
// imported or chameleon-included schemas are merged into a main schema, the attributes
// retain their original form semantics regardless of the main schema's attributeFormDefault.
func normalizeAttributeForms(complexType *types.ComplexType, sourceAttrFormDefault parser.Form) {
	normalizeAttr := func(attr *types.AttributeDecl) {
		if attr.Form == types.FormDefault {
			if sourceAttrFormDefault == parser.Qualified {
				attr.Form = types.FormQualified
			} else {
				attr.Form = types.FormUnqualified
			}
		}
	}

	for _, attr := range complexType.Attributes() {
		normalizeAttr(attr)
	}

	content := complexType.Content()
	if ext := content.ExtensionDef(); ext != nil {
		for _, attr := range ext.Attributes {
			normalizeAttr(attr)
		}
	}
	if restr := content.RestrictionDef(); restr != nil {
		for _, attr := range restr.Attributes {
			normalizeAttr(attr)
		}
	}
}
