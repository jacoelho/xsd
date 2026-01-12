package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
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
func (l *SchemaLoader) mergeSchema(target, source *parser.Schema, kind mergeKind, remap namespaceRemapMode) error {
	ctx := newMergeContext(target, source, kind, remap)
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
	ctx.mergeIDAttributes()
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
	for qname, decl := range c.source.ElementDecls {
		targetQName := c.remapQName(qname)
		origin := c.originFor(c.source.ElementOrigins, qname)
		if existing, exists := c.target.ElementDecls[targetQName]; exists {
			if c.target.ElementOrigins[targetQName] == origin {
				continue
			}
			candidate := c.elementDeclCandidate(decl)
			if elementDeclEquivalent(existing, candidate) {
				continue
			}
			return fmt.Errorf("duplicate element declaration %s", targetQName)
		}
		c.target.ElementDecls[targetQName] = c.elementDeclForInsert(decl)
		c.target.ElementOrigins[targetQName] = origin
	}
	return nil
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
	for qname, typ := range c.source.TypeDefs {
		targetQName := c.remapQName(qname)
		origin := c.originFor(c.source.TypeOrigins, qname)
		if _, exists := c.target.TypeDefs[targetQName]; exists {
			if c.target.TypeOrigins[targetQName] == origin {
				continue
			}
			return fmt.Errorf("duplicate type definition %s", targetQName)
		}
		if c.isImport {
			c.target.TypeDefs[targetQName] = c.copyTypeForImport(typ)
		} else {
			c.target.TypeDefs[targetQName] = c.copyTypeForInclude(typ)
		}
		c.target.TypeOrigins[targetQName] = origin
	}
	return nil
}

func (c *mergeContext) copyTypeForImport(typ types.Type) types.Type {
	if complexType, ok := typ.(*types.ComplexType); ok {
		typeCopy := *complexType
		typeCopy.QName = c.remapQName(complexType.QName)
		typeCopy.SourceNamespace = c.source.TargetNamespace
		normalizeAttributeForms(&typeCopy, c.source.AttributeFormDefault)
		return &typeCopy
	}
	if simpleType, ok := typ.(*types.SimpleType); ok {
		typeCopy := *simpleType
		typeCopy.QName = c.remapQName(simpleType.QName)
		typeCopy.SourceNamespace = c.source.TargetNamespace
		return &typeCopy
	}
	return typ
}

func (c *mergeContext) copyTypeForInclude(typ types.Type) types.Type {
	copiedType := types.CopyType(typ, c.opts)
	if complexType, ok := copiedType.(*types.ComplexType); ok {
		normalizeAttributeForms(complexType, c.source.AttributeFormDefault)
	}
	return copiedType
}

func (c *mergeContext) mergeAttributeDecls() error {
	for qname, decl := range c.source.AttributeDecls {
		targetQName := c.remapQName(qname)
		origin := c.originFor(c.source.AttributeOrigins, qname)
		if _, exists := c.target.AttributeDecls[targetQName]; exists {
			if c.target.AttributeOrigins[targetQName] == origin {
				continue
			}
			return fmt.Errorf("duplicate attribute declaration %s", targetQName)
		}
		c.target.AttributeDecls[targetQName] = c.copyAttributeDecl(decl)
		c.target.AttributeOrigins[targetQName] = origin
	}
	return nil
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
	for qname, group := range c.source.AttributeGroups {
		targetQName := c.remapQName(qname)
		origin := c.originFor(c.source.AttributeGroupOrigins, qname)
		if _, exists := c.target.AttributeGroups[targetQName]; exists {
			if c.target.AttributeGroupOrigins[targetQName] == origin {
				continue
			}
			return fmt.Errorf("duplicate attributeGroup %s", targetQName)
		}
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
		c.target.AttributeGroups[targetQName] = groupCopy
		c.target.AttributeGroupOrigins[targetQName] = origin
	}
	return nil
}

func (c *mergeContext) mergeGroups() error {
	for qname, group := range c.source.Groups {
		targetQName := c.remapQName(qname)
		origin := c.originFor(c.source.GroupOrigins, qname)
		if _, exists := c.target.Groups[targetQName]; exists {
			if c.target.GroupOrigins[targetQName] == origin {
				continue
			}
			return fmt.Errorf("duplicate group %s", targetQName)
		}
		c.target.Groups[targetQName] = group.Copy(c.opts)
		c.target.GroupOrigins[targetQName] = origin
	}
	return nil
}

func (c *mergeContext) mergeSubstitutionGroups() {
	for head, members := range c.source.SubstitutionGroups {
		targetHead := c.remapQName(head)
		remappedMembers := make([]types.QName, len(members))
		for i, member := range members {
			remappedMembers[i] = c.remapQName(member)
		}
		if existing, exists := c.target.SubstitutionGroups[targetHead]; exists {
			c.target.SubstitutionGroups[targetHead] = append(existing, remappedMembers...)
		} else {
			c.target.SubstitutionGroups[targetHead] = remappedMembers
		}
	}
}

func (c *mergeContext) mergeNotationDecls() error {
	for qname, notation := range c.source.NotationDecls {
		targetQName := c.remapQName(qname)
		origin := c.originFor(c.source.NotationOrigins, qname)
		if _, exists := c.target.NotationDecls[targetQName]; exists {
			if c.target.NotationOrigins[targetQName] == origin {
				continue
			}
			return fmt.Errorf("duplicate notation %s", targetQName)
		}
		c.target.NotationDecls[targetQName] = notation.Copy(c.opts)
		c.target.NotationOrigins[targetQName] = origin
	}
	return nil
}

func (c *mergeContext) mergeIDAttributes() {
	for id, component := range c.source.IDAttributes {
		if _, exists := c.target.IDAttributes[id]; !exists {
			c.target.IDAttributes[id] = component
		}
	}
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
	if a.HasFixed != b.HasFixed || a.Fixed != b.Fixed || a.Default != b.Default {
		return false
	}
	if a.Form != b.Form {
		return false
	}
	if !schemacheck.ElementTypesCompatible(a.Type, b.Type) {
		return false
	}
	if len(a.Constraints) != len(b.Constraints) {
		return false
	}
	for i := range a.Constraints {
		ac := a.Constraints[i]
		bc := b.Constraints[i]
		if ac.Name != bc.Name || ac.Type != bc.Type || ac.Selector.XPath != bc.Selector.XPath {
			return false
		}
		if ac.ReferQName != bc.ReferQName {
			return false
		}
		if len(ac.Fields) != len(bc.Fields) {
			return false
		}
		for j := range ac.Fields {
			if ac.Fields[j].XPath != bc.Fields[j].XPath {
				return false
			}
		}
	}
	return true
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
