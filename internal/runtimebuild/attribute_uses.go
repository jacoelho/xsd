package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type attrCollectionMode uint8

const (
	attrMerge attrCollectionMode = iota
	attrRestriction
)

func collectAttributeUses(schema *parser.Schema, ct *types.ComplexType) ([]*types.AttributeDecl, *types.AnyAttribute, error) {
	if schema == nil || ct == nil {
		return nil, nil, nil
	}
	attrMap := make(map[types.QName]*types.AttributeDecl)
	chain := collectComplexTypeChain(schema, ct)
	var wildcard *types.AnyAttribute
	for i := len(chain) - 1; i >= 0; i-- {
		current := chain[i]
		if err := mergeAttributesFromComplexType(schema, current, attrMap); err != nil {
			return nil, nil, err
		}
		localWildcard, err := localAttributeWildcard(schema, current)
		if err != nil {
			return nil, nil, err
		}
		if i == len(chain)-1 {
			wildcard = localWildcard
		} else {
			wildcard, err = applyDerivedWildcard(wildcard, localWildcard, current)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	out := make([]*types.AttributeDecl, 0, len(attrMap))
	for _, decl := range attrMap {
		out = append(out, decl)
	}
	return out, wildcard, nil
}

func collectComplexTypeChain(schema *parser.Schema, ct *types.ComplexType) []*types.ComplexType {
	var chain []*types.ComplexType
	visited := make(map[*types.ComplexType]bool)
	for current := ct; current != nil; {
		if visited[current] {
			break
		}
		visited[current] = true
		chain = append(chain, current)
		var next *types.ComplexType
		if baseCT, ok := current.ResolvedBase.(*types.ComplexType); ok {
			next = baseCT
		} else if current.ResolvedBase != nil {
			if isAnyTypeQName(current.ResolvedBase.Name()) {
				next = types.NewAnyTypeComplexType()
			}
		} else {
			baseQName := types.QName{}
			if content := current.Content(); content != nil {
				baseQName = content.BaseTypeQName()
			}
			if !baseQName.IsZero() {
				if isAnyTypeQName(baseQName) {
					next = types.NewAnyTypeComplexType()
				} else if base, ok := lookupComplexType(schema, baseQName); ok {
					next = base
				}
			} else if !isAnyTypeQName(current.QName) {
				next = types.NewAnyTypeComplexType()
			}
		}
		if next == nil {
			break
		}
		current = next
	}
	return chain
}

func isAnyTypeQName(qname types.QName) bool {
	return qname.Namespace == types.XSDNamespace && qname.Local == string(types.TypeNameAnyType)
}

func lookupComplexType(schema *parser.Schema, name types.QName) (*types.ComplexType, bool) {
	if schema == nil || name.IsZero() {
		return nil, false
	}
	typ, ok := schema.TypeDefs[name]
	if !ok {
		return nil, false
	}
	ct, ok := types.AsComplexType(typ)
	return ct, ok
}

func mergeAttributesFromComplexType(schema *parser.Schema, ct *types.ComplexType, attrMap map[types.QName]*types.AttributeDecl) error {
	if ct == nil {
		return nil
	}
	if err := mergeAttributes(schema, ct.Attributes(), ct.AttrGroups, attrMap, attrMerge); err != nil {
		return err
	}
	content := ct.Content()
	if content == nil {
		return nil
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := mergeAttributes(schema, ext.Attributes, ext.AttrGroups, attrMap, attrMerge); err != nil {
			return err
		}
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := mergeAttributes(schema, restr.Attributes, restr.AttrGroups, attrMap, attrRestriction); err != nil {
			return err
		}
	}
	return nil
}

func mergeAttributes(schema *parser.Schema, attrs []*types.AttributeDecl, groups []types.QName, attrMap map[types.QName]*types.AttributeDecl, mode attrCollectionMode) error {
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		key := effectiveAttributeQName(schema, attr)
		if !shouldIncludeAttribute(attr) {
			if mode == attrRestriction {
				delete(attrMap, key)
			}
			continue
		}
		attrMap[key] = attr
	}
	if len(groups) == 0 {
		return nil
	}
	visited := make(map[*types.AttributeGroup]bool)
	return mergeAttributesFromGroups(schema, groups, attrMap, mode, visited)
}

func mergeAttributesFromGroups(schema *parser.Schema, groups []types.QName, attrMap map[types.QName]*types.AttributeDecl, mode attrCollectionMode, visited map[*types.AttributeGroup]bool) error {
	for _, ref := range groups {
		group, ok := schema.AttributeGroups[ref]
		if !ok {
			return fmt.Errorf("attributeGroup %s not found", ref)
		}
		if visited[group] {
			continue
		}
		visited[group] = true
		groupMode := mode
		if mode == attrRestriction {
			groupMode = attrMerge
		}
		if err := mergeAttributes(schema, group.Attributes, group.AttrGroups, attrMap, groupMode); err != nil {
			return err
		}
	}
	return nil
}

func localAttributeWildcard(schema *parser.Schema, ct *types.ComplexType) (*types.AnyAttribute, error) {
	if schema == nil || ct == nil {
		return nil, nil
	}
	var groups []types.QName
	var explicit []*types.AnyAttribute

	groups = append(groups, ct.AttrGroups...)
	if anyAttr := ct.AnyAttribute(); anyAttr != nil {
		explicit = append(explicit, anyAttr)
	}

	if content := ct.Content(); content != nil {
		if ext := content.ExtensionDef(); ext != nil {
			groups = append(groups, ext.AttrGroups...)
			if ext.AnyAttribute != nil {
				explicit = append(explicit, ext.AnyAttribute)
			}
		}
		if restr := content.RestrictionDef(); restr != nil {
			groups = append(groups, restr.AttrGroups...)
			if restr.AnyAttribute != nil {
				explicit = append(explicit, restr.AnyAttribute)
			}
		}
	}

	groupWildcard, err := collectAttributeGroupWildcard(schema, groups)
	if err != nil {
		return nil, err
	}
	wildcard := groupWildcard
	for _, anyAttr := range explicit {
		var err error
		wildcard, err = intersectLocalAnyAttribute(anyAttr, wildcard)
		if err != nil {
			return nil, err
		}
	}
	return wildcard, nil
}

func collectAttributeGroupWildcard(schema *parser.Schema, groups []types.QName) (*types.AnyAttribute, error) {
	if schema == nil || len(groups) == 0 {
		return nil, nil
	}
	visited := make(map[*types.AttributeGroup]bool)
	var wildcard *types.AnyAttribute
	for _, ref := range groups {
		group, ok := schema.AttributeGroups[ref]
		if !ok {
			return nil, fmt.Errorf("attributeGroup %s not found", ref)
		}
		groupWildcard, err := attributeGroupWildcard(schema, group, visited)
		if err != nil {
			return nil, err
		}
		wildcard, err = intersectLocalAnyAttribute(groupWildcard, wildcard)
		if err != nil {
			return nil, err
		}
	}
	return wildcard, nil
}

func attributeGroupWildcard(schema *parser.Schema, group *types.AttributeGroup, visited map[*types.AttributeGroup]bool) (*types.AnyAttribute, error) {
	if schema == nil || group == nil {
		return nil, nil
	}
	if visited[group] {
		return nil, nil
	}
	visited[group] = true
	wildcard := group.AnyAttribute
	for _, ref := range group.AttrGroups {
		nested, ok := schema.AttributeGroups[ref]
		if !ok {
			return nil, fmt.Errorf("attributeGroup %s not found", ref)
		}
		nestedWildcard, err := attributeGroupWildcard(schema, nested, visited)
		if err != nil {
			return nil, err
		}
		wildcard, err = intersectLocalAnyAttribute(nestedWildcard, wildcard)
		if err != nil {
			return nil, err
		}
	}
	return wildcard, nil
}

func intersectLocalAnyAttribute(a, b *types.AnyAttribute) (*types.AnyAttribute, error) {
	if a == nil {
		return b, nil
	}
	if b == nil {
		return a, nil
	}
	intersected, expressible, empty := types.IntersectAnyAttributeDetailed(a, b)
	if !expressible {
		return nil, fmt.Errorf("attribute wildcard intersection not expressible")
	}
	if empty {
		return nil, fmt.Errorf("attribute wildcard intersection empty")
	}
	return intersected, nil
}

func applyDerivedWildcard(base, local *types.AnyAttribute, ct *types.ComplexType) (*types.AnyAttribute, error) {
	method := types.DerivationRestriction
	if ct != nil && ct.DerivationMethod != 0 {
		method = ct.DerivationMethod
	}
	switch method {
	case types.DerivationExtension:
		return unionAnyAttribute(local, base)
	case types.DerivationRestriction:
		return restrictAnyAttribute(base, local)
	default:
		if local != nil {
			return local, nil
		}
		return base, nil
	}
}

func unionAnyAttribute(derived, base *types.AnyAttribute) (*types.AnyAttribute, error) {
	if derived == nil {
		return base, nil
	}
	if base == nil {
		return derived, nil
	}
	merged := types.UnionAnyAttribute(derived, base)
	if merged == nil {
		return nil, fmt.Errorf("attribute wildcard union not expressible")
	}
	return merged, nil
}

func restrictAnyAttribute(base, derived *types.AnyAttribute) (*types.AnyAttribute, error) {
	if derived == nil {
		return nil, nil
	}
	if base == nil {
		return nil, fmt.Errorf("attribute wildcard restriction adds wildcard")
	}
	intersected, expressible, empty := types.IntersectAnyAttributeDetailed(derived, base)
	if !expressible {
		return nil, fmt.Errorf("attribute wildcard restriction not expressible")
	}
	if empty {
		return nil, fmt.Errorf("attribute wildcard restriction empty")
	}
	if intersected != nil {
		intersected.ProcessContents = derived.ProcessContents
	}
	return intersected, nil
}

func shouldIncludeAttribute(attr *types.AttributeDecl) bool {
	return attr.Use != types.Prohibited || attr.HasFixed
}

func effectiveAttributeQName(schema *parser.Schema, attr *types.AttributeDecl) types.QName {
	if attr == nil {
		return types.QName{}
	}
	if attr.IsReference {
		return attr.Name
	}
	form := attr.Form
	if form == types.FormDefault {
		if schema != nil && schema.AttributeFormDefault == parser.Qualified {
			form = types.FormQualified
		} else {
			form = types.FormUnqualified
		}
	}
	if form == types.FormQualified {
		ns := types.NamespaceEmpty
		if schema != nil {
			ns = schema.TargetNamespace
		}
		if !attr.SourceNamespace.IsEmpty() {
			ns = attr.SourceNamespace
		}
		return types.QName{Namespace: ns, Local: attr.Name.Local}
	}
	return types.QName{Namespace: types.NamespaceEmpty, Local: attr.Name.Local}
}
