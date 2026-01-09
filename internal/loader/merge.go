package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/validation"
)

// mergeSchema merges a source schema into a target schema.
// For imports (isImport=true), preserves source namespace.
// For includes (isImport=false), uses chameleon namespace remapping if needed.
func (l *SchemaLoader) mergeSchema(target, source *parser.Schema, isImport, needsNamespaceRemap bool) error {
	remapQName := func(qname types.QName) types.QName {
		if needsNamespaceRemap && qname.Namespace.IsEmpty() {
			return types.QName{
				Namespace: target.TargetNamespace,
				Local:     qname.Local,
			}
		}
		return qname
	}

	computeSourceNamespace := func() types.NamespaceURI {
		if isImport {
			return source.TargetNamespace
		}
		if needsNamespaceRemap {
			return target.TargetNamespace
		}
		return source.TargetNamespace
	}

	opts := types.CopyOptions{
		SourceNamespace: computeSourceNamespace(),
		RemapQName:      remapQName,
	}

	if source.ImportedNamespaces != nil {
		if target.ImportedNamespaces == nil {
			target.ImportedNamespaces = make(map[types.NamespaceURI]map[types.NamespaceURI]bool)
		}
		for fromNS, imports := range source.ImportedNamespaces {
			mappedFrom := fromNS
			if needsNamespaceRemap && fromNS.IsEmpty() {
				mappedFrom = target.TargetNamespace
			}
			if _, ok := target.ImportedNamespaces[mappedFrom]; !ok {
				target.ImportedNamespaces[mappedFrom] = make(map[types.NamespaceURI]bool)
			}
			for ns := range imports {
				target.ImportedNamespaces[mappedFrom][ns] = true
			}
		}
	}

	if source.ImportContexts != nil {
		if target.ImportContexts == nil {
			target.ImportContexts = make(map[string]parser.ImportContext)
		}
		for location, ctx := range source.ImportContexts {
			merged := ctx
			if needsNamespaceRemap && merged.TargetNamespace.IsEmpty() {
				merged.TargetNamespace = target.TargetNamespace
			}
			if merged.Imports == nil {
				merged.Imports = make(map[types.NamespaceURI]bool)
			}
			if existing, ok := target.ImportContexts[location]; ok {
				if existing.Imports == nil {
					existing.Imports = make(map[types.NamespaceURI]bool)
				}
				for ns := range merged.Imports {
					existing.Imports[ns] = true
				}
				if existing.TargetNamespace.IsEmpty() {
					existing.TargetNamespace = merged.TargetNamespace
				}
				target.ImportContexts[location] = existing
				continue
			}
			target.ImportContexts[location] = merged
		}
	}

	for qname, decl := range source.ElementDecls {
		targetQName := remapQName(qname)
		origin := source.ElementOrigins[qname]
		if origin == "" {
			origin = source.Location
		}
		if _, exists := target.ElementDecls[targetQName]; exists {
			if target.ElementOrigins[targetQName] == origin {
				continue
			}
			existing := target.ElementDecls[targetQName]
			var candidate *types.ElementDecl
			if isImport {
				declCopy := *decl
				declCopy.Name = remapQName(decl.Name)
				declCopy.SourceNamespace = source.TargetNamespace
				candidate = &declCopy
			} else if needsNamespaceRemap {
				candidate = decl.Copy(opts)
			} else {
				candidate = decl
			}
			if elementDeclEquivalent(existing, candidate) {
				continue
			}
			return fmt.Errorf("duplicate element declaration %s", targetQName)
		}
		// for imports, use shallow copy to avoid unnecessary type remapping
		if isImport {
			declCopy := *decl
			declCopy.Name = remapQName(decl.Name)
			declCopy.SourceNamespace = source.TargetNamespace
			// types and constraints are not remapped for imports
			target.ElementDecls[targetQName] = &declCopy
		} else {
			target.ElementDecls[targetQName] = decl.Copy(opts)
		}
		target.ElementOrigins[targetQName] = origin
	}

	for qname, typ := range source.TypeDefs {
		targetQName := remapQName(qname)
		origin := source.TypeOrigins[qname]
		if origin == "" {
			origin = source.Location
		}
		if _, exists := target.TypeDefs[targetQName]; exists {
			if target.TypeOrigins[targetQName] == origin {
				continue
			}
			return fmt.Errorf("duplicate type definition %s", targetQName)
		}
		// for imports, use shallow copy to avoid unnecessary remapping
		if isImport {
			if complexType, ok := typ.(*types.ComplexType); ok {
				typeCopy := *complexType
				typeCopy.QName = remapQName(complexType.QName)
				typeCopy.SourceNamespace = source.TargetNamespace
				normalizeAttributeForms(&typeCopy, source.AttributeFormDefault)
				target.TypeDefs[targetQName] = &typeCopy
			} else if simpleType, ok := typ.(*types.SimpleType); ok {
				typeCopy := *simpleType
				typeCopy.QName = remapQName(simpleType.QName)
				typeCopy.SourceNamespace = source.TargetNamespace
				target.TypeDefs[targetQName] = &typeCopy
			} else {
				target.TypeDefs[targetQName] = typ
			}
		} else {
			copiedType := types.CopyType(typ, opts)
			if complexType, ok := copiedType.(*types.ComplexType); ok {
				normalizeAttributeForms(complexType, source.AttributeFormDefault)
			}
			target.TypeDefs[targetQName] = copiedType
		}
		target.TypeOrigins[targetQName] = origin
	}

	for qname, decl := range source.AttributeDecls {
		targetQName := remapQName(qname)
		origin := source.AttributeOrigins[qname]
		if origin == "" {
			origin = source.Location
		}
		if _, exists := target.AttributeDecls[targetQName]; exists {
			if target.AttributeOrigins[targetQName] == origin {
				continue
			}
			return fmt.Errorf("duplicate attribute declaration %s", targetQName)
		}
		// for imports, use shallow copy to avoid unnecessary type remapping
		if isImport {
			declCopy := *decl
			declCopy.Name = remapQName(decl.Name)
			declCopy.SourceNamespace = source.TargetNamespace
			// types are not remapped for imports
			target.AttributeDecls[targetQName] = &declCopy
		} else {
			target.AttributeDecls[targetQName] = decl.Copy(opts)
		}
		target.AttributeOrigins[targetQName] = origin
	}

	for qname, group := range source.AttributeGroups {
		targetQName := remapQName(qname)
		origin := source.AttributeGroupOrigins[qname]
		if origin == "" {
			origin = source.Location
		}
		if _, exists := target.AttributeGroups[targetQName]; exists {
			if target.AttributeGroupOrigins[targetQName] == origin {
				continue
			}
			return fmt.Errorf("duplicate attributeGroup %s", targetQName)
		}
		groupCopy := group.Copy(opts)
		for _, attr := range groupCopy.Attributes {
			if attr.Form == types.FormDefault {
				if source.AttributeFormDefault == parser.Qualified {
					attr.Form = types.FormQualified
				} else {
					attr.Form = types.FormUnqualified
				}
			}
		}
		target.AttributeGroups[targetQName] = groupCopy
		target.AttributeGroupOrigins[targetQName] = origin
	}

	for qname, group := range source.Groups {
		targetQName := remapQName(qname)
		origin := source.GroupOrigins[qname]
		if origin == "" {
			origin = source.Location
		}
		if _, exists := target.Groups[targetQName]; exists {
			if target.GroupOrigins[targetQName] == origin {
				continue
			}
			return fmt.Errorf("duplicate group %s", targetQName)
		}
		// for chameleon includes (needsNamespaceRemap), Copy handles remapping particles
		// for imports, just preserve SourceNamespace (handled by Copy)
		target.Groups[targetQName] = group.Copy(opts)
		target.GroupOrigins[targetQName] = origin
	}

	for head, members := range source.SubstitutionGroups {
		targetHead := remapQName(head)
		remappedMembers := make([]types.QName, len(members))
		for i, member := range members {
			remappedMembers[i] = remapQName(member)
		}
		if existing, exists := target.SubstitutionGroups[targetHead]; exists {
			target.SubstitutionGroups[targetHead] = append(existing, remappedMembers...)
		} else {
			target.SubstitutionGroups[targetHead] = remappedMembers
		}
	}

	for qname, notation := range source.NotationDecls {
		targetQName := remapQName(qname)
		origin := source.NotationOrigins[qname]
		if origin == "" {
			origin = source.Location
		}
		if _, exists := target.NotationDecls[targetQName]; exists {
			if target.NotationOrigins[targetQName] == origin {
				continue
			}
			return fmt.Errorf("duplicate notation %s", targetQName)
		}
		target.NotationDecls[targetQName] = notation.Copy(opts)
		target.NotationOrigins[targetQName] = origin
	}

	// merge id attributes (per XSD spec, id uniqueness is per schema document, not across merged schemas)
	for id, component := range source.IDAttributes {
		if _, exists := target.IDAttributes[id]; !exists {
			target.IDAttributes[id] = component
		}
	}

	return nil
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
	if !validation.ElementTypesCompatible(a.Type, b.Type) {
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
func normalizeAttributeForms(ct *types.ComplexType, sourceAttrFormDefault parser.Form) {
	normalizeAttr := func(attr *types.AttributeDecl) {
		if attr.Form == types.FormDefault {
			if sourceAttrFormDefault == parser.Qualified {
				attr.Form = types.FormQualified
			} else {
				attr.Form = types.FormUnqualified
			}
		}
	}

	for _, attr := range ct.Attributes() {
		normalizeAttr(attr)
	}

	content := ct.Content()
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
