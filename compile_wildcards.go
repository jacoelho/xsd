package xsd

import (
	"slices"
	"strings"
)

func (c *compiler) compileWildcardParticle(n *rawNode, ctx *schemaContext) (particle, error) {
	if err := validateAnyParticleSyntax(n); err != nil {
		return particle{}, err
	}
	id, err := c.compileWildcard(n, ctx, false)
	if err != nil {
		return particle{}, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return particle{}, err
	}
	return particle{Kind: particleWildcard, wildcard: id, occurs: occurs}, nil
}

func validateAnyParticleSyntax(n *rawNode) error {
	if err := validateKnownAttributes(n, "any", map[string]bool{
		"id": true, "namespace": true, "processContents": true, "minOccurs": true, "maxOccurs": true,
	}); err != nil {
		return err
	}
	if len(n.xsContentChildren()) != 0 {
		return schemaCompile(ErrSchemaContentModel, "any can contain only annotation")
	}
	return nil
}

func (c *compiler) compileWildcard(n *rawNode, ctx *schemaContext, attr bool) (wildcardID, error) {
	if attr {
		if err := validateKnownAttributes(n, "anyAttribute", map[string]bool{"id": true, "namespace": true, "processContents": true}); err != nil {
			return noWildcard, err
		}
		if len(n.xsContentChildren()) != 0 {
			return noWildcard, schemaCompile(ErrSchemaContentModel, "anyAttribute can contain only annotation")
		}
	}
	var mode wildcardMode
	var namespaces []namespaceID
	other := namespaceID(0)
	nsSpec := n.attrDefault("namespace", "##any")
	switch nsSpec {
	case "##any":
		mode = wildAny
	case "##other":
		mode = wildOther
		other = c.rt.Names.InternNamespace(ctx.targetNS)
	case "##local":
		mode = wildLocal
	case "##targetNamespace":
		mode = wildTargetNamespace
		namespaces = append(namespaces, c.rt.Names.InternNamespace(ctx.targetNS))
	default:
		mode = wildList
		parts := strings.Fields(nsSpec)
		for _, part := range parts {
			switch part {
			case "##local":
				namespaces = append(namespaces, c.rt.Names.InternNamespace(""))
			case "##targetNamespace":
				namespaces = append(namespaces, c.rt.Names.InternNamespace(ctx.targetNS))
			default:
				if strings.HasPrefix(part, "##") {
					return noWildcard, schemaCompile(ErrSchemaInvalidAttribute, "invalid wildcard namespace "+part)
				}
				namespaces = append(namespaces, c.rt.Names.InternNamespace(part))
			}
		}
	}
	var process processContents
	switch n.attrDefault("processContents", "strict") {
	case "skip":
		process = processSkip
	case "lax":
		process = processLax
	case "strict":
		process = processStrict
	default:
		return noWildcard, schemaCompile(ErrSchemaInvalidAttribute, "invalid processContents")
	}
	id := wildcardID(len(c.rt.Wildcards))
	c.rt.Wildcards = append(c.rt.Wildcards, wildcard{Mode: mode, Namespaces: namespaces, OtherThan: other, Process: process})
	return id, nil
}

func (c *compiler) unionWildcards(a, b wildcardID, process processContents) (wildcardID, error) {
	wa := c.rt.Wildcards[a]
	wb := c.rt.Wildcards[b]
	if c.sameWildcardNamespaceConstraint(wa, wb) {
		wa.Process = process
		return c.addWildcard(wa), nil
	}
	if wa.Mode == wildAny || wb.Mode == wildAny {
		return c.addWildcard(wildcard{Mode: wildAny, Process: process}), nil
	}
	if wa.Mode == wildOther && wb.Mode == wildOther {
		return c.addWildcard(wildcard{Mode: wildOther, OtherThan: c.rt.Names.InternNamespace(""), Process: process}), nil
	}
	if wa.Mode == wildOther {
		return c.unionOtherWithFinite(wa, wb, process)
	}
	if wb.Mode == wildOther {
		return c.unionOtherWithFinite(wb, wa, process)
	}
	seen := make(map[namespaceID]bool)
	var namespaces []namespaceID
	add := func(w wildcard) {
		switch w.Mode {
		case wildLocal:
			ns := c.rt.Names.InternNamespace("")
			if !seen[ns] {
				seen[ns] = true
				namespaces = append(namespaces, ns)
			}
		case wildTargetNamespace:
			if len(w.Namespaces) != 0 && !seen[w.Namespaces[0]] {
				seen[w.Namespaces[0]] = true
				namespaces = append(namespaces, w.Namespaces[0])
			}
		case wildList:
			for _, ns := range w.Namespaces {
				if !seen[ns] {
					seen[ns] = true
					namespaces = append(namespaces, ns)
				}
			}
		}
	}
	add(wa)
	add(wb)
	return c.addWildcard(wildcard{Mode: wildList, Namespaces: namespaces, Process: process}), nil
}

func (c *compiler) unionOtherWithFinite(other, finite wildcard, process processContents) (wildcardID, error) {
	namespaces := c.wildcardFiniteNamespaces(finite)
	hasAbsent := slices.Contains(namespaces, c.rt.Names.InternNamespace(""))
	hasNegated := slices.Contains(namespaces, other.OtherThan)
	if other.OtherThan == c.rt.Names.InternNamespace("") {
		if hasAbsent {
			return c.addWildcard(wildcard{Mode: wildAny, Process: process}), nil
		}
		return c.addWildcard(wildcard{Mode: wildOther, OtherThan: other.OtherThan, Process: process}), nil
	}
	switch {
	case hasAbsent && hasNegated:
		return c.addWildcard(wildcard{Mode: wildAny, Process: process}), nil
	case hasNegated:
		return c.addWildcard(wildcard{Mode: wildOther, OtherThan: c.rt.Names.InternNamespace(""), Process: process}), nil
	case hasAbsent:
		return noWildcard, schemaCompile(ErrSchemaContentModel, "attribute wildcard union is not expressible")
	default:
		return c.addWildcard(wildcard{Mode: wildOther, OtherThan: other.OtherThan, Process: process}), nil
	}
}

func (c *compiler) intersectWildcards(a, b wildcardID, process processContents) (wildcardID, error) {
	wa := c.rt.Wildcards[a]
	wb := c.rt.Wildcards[b]
	if c.sameWildcardNamespaceConstraint(wa, wb) {
		wa.Process = process
		return c.addWildcard(wa), nil
	}
	if wa.Mode == wildAny {
		wb.Process = process
		return c.addWildcard(wb), nil
	}
	if wb.Mode == wildAny {
		wa.Process = process
		return c.addWildcard(wa), nil
	}
	if wa.Mode == wildOther && wb.Mode == wildOther {
		if wa.OtherThan == c.rt.Names.InternNamespace("") {
			wb.Process = process
			return c.addWildcard(wb), nil
		}
		if wb.OtherThan == c.rt.Names.InternNamespace("") {
			wa.Process = process
			return c.addWildcard(wa), nil
		}
		return noWildcard, schemaCompile(ErrSchemaContentModel, "attribute wildcard intersection is not expressible")
	}
	candidates := c.wildcardFiniteNamespaces(wa)
	other := c.wildcardFiniteNamespaces(wb)
	for _, ns := range other {
		if !slices.Contains(candidates, ns) {
			candidates = append(candidates, ns)
		}
	}
	var namespaces []namespaceID
	for _, ns := range candidates {
		if c.wildcardCoversNamespace(wa, ns) && c.wildcardCoversNamespace(wb, ns) && !slices.Contains(namespaces, ns) {
			namespaces = append(namespaces, ns)
		}
	}
	return c.addWildcard(wildcard{Mode: wildList, Namespaces: namespaces, Process: process}), nil
}

func (c *compiler) sameWildcardNamespaceConstraint(a, b wildcard) bool {
	if a.Mode != b.Mode {
		return false
	}
	switch a.Mode {
	case wildAny:
		return true
	case wildOther:
		return a.OtherThan == b.OtherThan
	case wildLocal:
		return true
	case wildTargetNamespace:
		return len(a.Namespaces) == len(b.Namespaces) && (len(a.Namespaces) == 0 || a.Namespaces[0] == b.Namespaces[0])
	case wildList:
		if len(a.Namespaces) != len(b.Namespaces) {
			return false
		}
		for _, ns := range a.Namespaces {
			if !slices.Contains(b.Namespaces, ns) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (c *compiler) wildcardFiniteNamespaces(w wildcard) []namespaceID {
	switch w.Mode {
	case wildLocal:
		return []namespaceID{c.rt.Names.InternNamespace("")}
	case wildTargetNamespace:
		return append([]namespaceID(nil), w.Namespaces...)
	case wildList:
		return append([]namespaceID(nil), w.Namespaces...)
	default:
		return nil
	}
}

func (c *compiler) wildcardCoversNamespace(w wildcard, ns namespaceID) bool {
	switch w.Mode {
	case wildAny:
		return true
	case wildOther:
		return ns != c.rt.Names.InternNamespace("") && ns != w.OtherThan
	case wildLocal:
		return c.rt.Names.Namespace(ns) == ""
	case wildTargetNamespace:
		return len(w.Namespaces) != 0 && w.Namespaces[0] == ns
	case wildList:
		return slices.Contains(w.Namespaces, ns)
	default:
		return false
	}
}

func (c *compiler) wildcardAllowsQName(id wildcardID, q qName) bool {
	return c.wildcardCoversNamespace(c.rt.Wildcards[id], q.Namespace)
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
		return c.wildcardCoversNamespace(base, c.rt.Names.InternNamespace(""))
	case wildTargetNamespace:
		return len(derived.Namespaces) != 0 && c.wildcardCoversNamespace(base, derived.Namespaces[0])
	case wildList:
		for _, ns := range derived.Namespaces {
			if !c.wildcardCoversNamespace(base, ns) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (c *compiler) addWildcard(w wildcard) wildcardID {
	id := wildcardID(len(c.rt.Wildcards))
	c.rt.Wildcards = append(c.rt.Wildcards, w)
	return id
}
