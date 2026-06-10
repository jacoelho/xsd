package xsd

import (
	"slices"
	"strings"
)

func (c *compiler) compileWildcardParticle(n *rawNode, ctx *schemaContext) (particle, error) {
	if err := validateAnyParticleSyntax(n); err != nil {
		return particle{}, err
	}
	id, err := c.compileWildcard(n, ctx)
	if err != nil {
		return particle{}, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return particle{}, err
	}
	return particle{Kind: particleWildcard, Wildcard: id, Occurs: occurs}, nil
}

func validateAnyParticleSyntax(n *rawNode) error {
	if len(n.xsContentChildren()) != 0 {
		return schemaCompileAt(n, ErrSchemaContentModel, "any can contain only annotation")
	}
	return nil
}

func validateAnyAttributeSyntax(n *rawNode) error {
	if len(n.xsContentChildren()) != 0 {
		return schemaCompileAt(n, ErrSchemaContentModel, "anyAttribute can contain only annotation")
	}
	return nil
}

func (c *compiler) compileAttributeWildcard(n *rawNode, ctx *schemaContext) (wildcardID, error) {
	if err := validateAnyAttributeSyntax(n); err != nil {
		return noWildcard, err
	}
	return c.compileWildcard(n, ctx)
}

func isAnyParticleAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrNamespace, xsdAttrProcessContents, xsdAttrMinOccurs, xsdAttrMaxOccurs:
		return true
	default:
		return false
	}
}

func isAnyAttributeAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrNamespace, xsdAttrProcessContents:
		return true
	default:
		return false
	}
}

func (c *compiler) compileWildcard(n *rawNode, ctx *schemaContext) (wildcardID, error) {
	var mode wildcardMode
	var namespaces []namespaceID
	other := namespaceID(0)
	nsSpec := n.attrDefault(xsdAttrNamespace, "##any")
	switch nsSpec {
	case "##any":
		mode = wildAny
	case "##other":
		mode = wildOther
		ns, err := c.rt.Names.InternNamespace(ctx.targetNS)
		if err != nil {
			return noWildcard, err
		}
		other = ns
	case "##local":
		mode = wildLocal
	case "##targetNamespace":
		mode = wildTargetNamespace
		ns, err := c.rt.Names.InternNamespace(ctx.targetNS)
		if err != nil {
			return noWildcard, err
		}
		namespaces = append(namespaces, ns)
	default:
		mode = wildList
		for part := range xmlFieldsSeq(nsSpec) {
			var uri string
			switch part {
			case "##local":
				uri = ""
			case "##targetNamespace":
				uri = ctx.targetNS
			default:
				if strings.HasPrefix(part, "##") {
					return noWildcard, schemaCompileAt(n, ErrSchemaInvalidAttribute, "invalid wildcard namespace "+part)
				}
				uri = part
			}
			ns, err := c.rt.Names.InternNamespace(uri)
			if err != nil {
				return noWildcard, err
			}
			namespaces = append(namespaces, ns)
		}
		namespaces = normalizeNamespaceList(namespaces)
	}
	var process processContents
	switch n.attrDefault(xsdAttrProcessContents, "strict") {
	case "skip":
		process = processSkip
	case "lax":
		process = processLax
	case "strict":
		process = processStrict
	default:
		return noWildcard, schemaCompileAt(n, ErrSchemaInvalidAttribute, "invalid processContents")
	}
	id, err := nextWildcardID(len(c.rt.Wildcards))
	if err != nil {
		return noWildcard, err
	}
	c.rt.Wildcards = append(c.rt.Wildcards, wildcard{Mode: mode, Namespaces: namespaces, OtherThan: other, Process: process})
	return id, nil
}

func (c *compiler) unionWildcards(a, b wildcardID, process processContents) (wildcardID, error) {
	wa := c.rt.Wildcards[a]
	wb := c.rt.Wildcards[b]
	if wildcardNamespaceEqual(wa, wb) {
		wa.Process = process
		return c.addWildcard(wa)
	}
	if wa.Mode == wildAny || wb.Mode == wildAny {
		return c.addWildcard(wildcard{Mode: wildAny, Process: process})
	}
	emptyNS := emptyNamespaceID
	if wa.Mode == wildOther && wb.Mode == wildOther {
		return c.addWildcard(wildcard{Mode: wildOther, OtherThan: emptyNS, Process: process})
	}
	if wa.Mode == wildOther {
		return c.unionOtherWithFinite(wa, wb, process)
	}
	if wb.Mode == wildOther {
		return c.unionOtherWithFinite(wb, wa, process)
	}
	namespaces := append(wildcardFiniteNamespaces(wa), wildcardFiniteNamespaces(wb)...)
	namespaces = normalizeNamespaceList(namespaces)
	return c.addWildcard(wildcard{Mode: wildList, Namespaces: namespaces, Process: process})
}

func (c *compiler) unionOtherWithFinite(other, finite wildcard, process processContents) (wildcardID, error) {
	namespaces := wildcardFiniteNamespaces(finite)
	emptyNS := emptyNamespaceID
	hasAbsent := slices.Contains(namespaces, emptyNS)
	hasNegated := slices.Contains(namespaces, other.OtherThan)
	if other.OtherThan == emptyNS {
		if hasAbsent {
			return c.addWildcard(wildcard{Mode: wildAny, Process: process})
		}
		return c.addWildcard(wildcard{Mode: wildOther, OtherThan: other.OtherThan, Process: process})
	}
	switch {
	case hasAbsent && hasNegated:
		return c.addWildcard(wildcard{Mode: wildAny, Process: process})
	case hasNegated:
		return c.addWildcard(wildcard{Mode: wildOther, OtherThan: emptyNS, Process: process})
	case hasAbsent:
		return noWildcard, schemaCompile(ErrSchemaContentModel, "attribute wildcard union is not expressible")
	default:
		return c.addWildcard(wildcard{Mode: wildOther, OtherThan: other.OtherThan, Process: process})
	}
}

func (c *compiler) intersectWildcards(a, b wildcardID, process processContents) (wildcardID, error) {
	wa := c.rt.Wildcards[a]
	wb := c.rt.Wildcards[b]
	if wildcardNamespaceEqual(wa, wb) {
		wa.Process = process
		return c.addWildcard(wa)
	}
	if wa.Mode == wildAny {
		wb.Process = process
		return c.addWildcard(wb)
	}
	if wb.Mode == wildAny {
		wa.Process = process
		return c.addWildcard(wa)
	}
	emptyNS := emptyNamespaceID
	if wa.Mode == wildOther && wb.Mode == wildOther {
		if wa.OtherThan == emptyNS {
			wb.Process = process
			return c.addWildcard(wb)
		}
		if wb.OtherThan == emptyNS {
			wa.Process = process
			return c.addWildcard(wa)
		}
		return noWildcard, schemaCompile(ErrSchemaContentModel, "attribute wildcard intersection is not expressible")
	}
	candidates := append(wildcardFiniteNamespaces(wa), wildcardFiniteNamespaces(wb)...)
	candidates = normalizeNamespaceList(candidates)
	var namespaces []namespaceID
	for _, ns := range candidates {
		if wildcardAllowsNamespace(wa, ns) && wildcardAllowsNamespace(wb, ns) {
			namespaces = append(namespaces, ns)
		}
	}
	return c.addWildcard(wildcard{Mode: wildList, Namespaces: namespaces, Process: process})
}

func normalizeNamespaceList(namespaces []namespaceID) []namespaceID {
	slices.Sort(namespaces)
	return slices.Compact(namespaces)
}

func wildcardFiniteNamespaces(w wildcard) []namespaceID {
	switch w.Mode {
	case wildLocal:
		return []namespaceID{emptyNamespaceID}
	case wildTargetNamespace, wildList:
		return slices.Clone(w.Namespaces)
	default:
		return nil
	}
}

func wildcardAllowsNamespace(w wildcard, ns namespaceID) bool {
	switch w.Mode {
	case wildAny:
		return true
	case wildOther:
		return ns != emptyNamespaceID && ns != w.OtherThan
	case wildLocal:
		return ns == emptyNamespaceID
	case wildTargetNamespace:
		return len(w.Namespaces) != 0 && w.Namespaces[0] == ns
	case wildList:
		return slices.Contains(w.Namespaces, ns)
	default:
		return false
	}
}

func (c *compiler) wildcardAllowsQName(id wildcardID, q qName) bool {
	return wildcardAllowsNamespace(c.rt.Wildcards[id], q.Namespace)
}

func (c *compiler) wildcardSubset(derivedID, baseID wildcardID) bool {
	derived := c.rt.Wildcards[derivedID]
	base := c.rt.Wildcards[baseID]
	if derived.Process > base.Process {
		return false
	}
	switch derived.Mode {
	case wildAny:
		return base.Mode == wildAny
	case wildOther:
		return base.Mode == wildAny || (base.Mode == wildOther && base.OtherThan == derived.OtherThan)
	case wildLocal:
		return wildcardAllowsNamespace(base, emptyNamespaceID)
	case wildTargetNamespace:
		return len(derived.Namespaces) != 0 && wildcardAllowsNamespace(base, derived.Namespaces[0])
	case wildList:
		for _, ns := range derived.Namespaces {
			if !wildcardAllowsNamespace(base, ns) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func wildcardsOverlap(a, b wildcard) bool {
	if a.Mode == wildAny || b.Mode == wildAny {
		return true
	}
	if a.Mode == wildOther && b.Mode == wildOther {
		return true
	}
	if a.Mode == wildOther {
		return wildcardHasNamespaceOtherThan(b, a.OtherThan)
	}
	if b.Mode == wildOther {
		return wildcardHasNamespaceOtherThan(a, b.OtherThan)
	}
	emptyNS := emptyNamespaceID
	if a.Mode == wildLocal && b.Mode == wildLocal {
		return true
	}
	if a.Mode == wildLocal {
		return wildcardAllowsNamespace(b, emptyNS)
	}
	if b.Mode == wildLocal {
		return wildcardAllowsNamespace(a, emptyNS)
	}
	for _, ns := range wildcardNamespaces(a) {
		if wildcardAllowsNamespace(b, ns) {
			return true
		}
	}
	return false
}

func wildcardHasNamespaceOtherThan(w wildcard, excluded namespaceID) bool {
	switch w.Mode {
	case wildAny, wildOther:
		return true
	case wildLocal:
		return false
	case wildTargetNamespace, wildList:
		for _, ns := range wildcardNamespaces(w) {
			if ns != excluded {
				return true
			}
		}
	}
	return false
}

func wildcardNamespaceEqual(a, b wildcard) bool {
	if a.Mode != b.Mode || a.OtherThan != b.OtherThan {
		return false
	}
	return slices.Equal(a.Namespaces, b.Namespaces)
}

func wildcardNamespaces(w wildcard) []namespaceID {
	if w.Mode == wildTargetNamespace || w.Mode == wildList {
		return w.Namespaces
	}
	return nil
}

func (c *compiler) addWildcard(w wildcard) (wildcardID, error) {
	id, err := nextWildcardID(len(c.rt.Wildcards))
	if err != nil {
		return noWildcard, err
	}
	c.rt.Wildcards = append(c.rt.Wildcards, w)
	return id, nil
}
