package xsd

import "strings"

func identityConstraintNodes(n *rawNode) []*rawNode {
	var nodes []*rawNode
	for _, child := range n.xsContentChildren() {
		switch child.Name.Local {
		case "key", "keyref", "unique":
			nodes = append(nodes, child)
		}
	}
	return nodes
}

func (c *compiler) declareAllIdentityConstraints() error {
	for _, doc := range c.docs {
		ctx := c.contexts[doc]
		if err := c.declareIdentityConstraintsInTree(doc.root, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) declareIdentityConstraintsInTree(n *rawNode, ctx *schemaContext) error {
	if n.Name.Space == xsdNamespaceURI && n.Name.Local == "element" {
		if _, err := c.declareIdentityConstraints(identityConstraintNodes(n), ctx); err != nil {
			return err
		}
	}
	for _, child := range n.Children {
		if err := c.declareIdentityConstraintsInTree(child, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) declareIdentityConstraints(nodes []*rawNode, ctx *schemaContext) ([]identityConstraintID, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	ids := make([]identityConstraintID, 0, len(nodes))
	for _, node := range nodes {
		if id, ok := c.identityDeclared[node]; ok {
			ids = append(ids, id)
			continue
		}
		name, ok := node.attr("name")
		if !ok || name == "" {
			return nil, schemaCompile(ErrSchemaIdentity, "identity constraint missing name")
		}
		q := c.rt.Names.InternQName(ctx.targetNS, name)
		if _, exists := c.rt.GlobalIdentities[q]; exists {
			return nil, schemaCompile(ErrSchemaDuplicate, "duplicate identity constraint "+c.rt.Names.Format(q))
		}
		id := identityConstraintID(len(c.rt.Identities))
		c.rt.Identities = append(c.rt.Identities, identityConstraint{Name: q, Refer: noIdentityConstraint})
		c.rt.GlobalIdentities[q] = id
		c.identityDeclared[node] = id
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *compiler) compileDeclaredIdentityConstraints(nodes []*rawNode, ids []identityConstraintID, ctx *schemaContext) error {
	for i, node := range nodes {
		id := ids[i]
		ic, err := c.compileIdentityConstraint(node, ctx)
		if err != nil {
			return err
		}
		ic.Name = c.rt.Identities[id].Name
		c.rt.Identities[id] = ic
	}
	return nil
}

func (c *compiler) validateIdentityReferences() error {
	for _, ic := range c.rt.Identities {
		if ic.Kind != identityKeyRef {
			continue
		}
		ref := c.rt.Identities[ic.Refer]
		if ref.Kind == identityKeyRef {
			return schemaCompile(ErrSchemaIdentity, "keyref refer cannot be keyref")
		}
		if len(ic.Fields) != len(ref.Fields) {
			return schemaCompile(ErrSchemaIdentity, "keyref field count does not match referenced key")
		}
	}
	return nil
}

func (c *compiler) compileIdentityConstraint(n *rawNode, ctx *schemaContext) (identityConstraint, error) {
	ic := identityConstraint{Refer: noIdentityConstraint}
	if err := validateIdentityConstraintSyntax(n); err != nil {
		return ic, err
	}
	switch n.Name.Local {
	case "key":
		ic.Kind = identityKey
	case "unique":
		ic.Kind = identityUnique
	case "keyref":
		ic.Kind = identityKeyRef
		refer, ok := n.attr("refer")
		if !ok {
			return ic, schemaCompile(ErrSchemaIdentity, "keyref missing refer")
		}
		q, err := c.resolveQNameChecked(n, ctx, refer)
		if err != nil {
			return ic, err
		}
		ref, ok := c.rt.GlobalIdentities[q]
		if !ok {
			return ic, schemaCompile(ErrSchemaReference, "unknown keyref refer "+c.rt.Names.Format(q))
		}
		ic.Refer = ref
	}
	selector := n.firstXS("selector")
	if selector == nil {
		return ic, schemaCompile(ErrSchemaIdentity, "identity constraint missing selector")
	}
	xpath, ok := selector.attr("xpath")
	if !ok {
		return ic, schemaCompile(ErrSchemaIdentity, "selector missing xpath")
	}
	paths, err := c.parseIdentityPaths(selector, xpath)
	if err != nil {
		return ic, err
	}
	ic.Selector = paths
	for _, field := range n.xsChildren("field") {
		xpath, ok := field.attr("xpath")
		if !ok {
			return ic, schemaCompile(ErrSchemaIdentity, "field missing xpath")
		}
		fieldPaths, err := c.parseIdentityFieldPaths(field, xpath)
		if err != nil {
			return ic, err
		}
		ic.Fields = append(ic.Fields, identityField{Paths: fieldPaths})
	}
	if len(ic.Fields) == 0 {
		return ic, schemaCompile(ErrSchemaIdentity, "identity constraint missing fields")
	}
	return ic, nil
}

func validateIdentityConstraintSyntax(n *rawNode) error {
	allowed := map[string]bool{"id": true, "name": true}
	if n.Name.Local == "keyref" {
		allowed["refer"] = true
	}
	if err := validateKnownAttributes(n, n.Name.Local, allowed); err != nil {
		return err
	}
	seenAnnotation := false
	seenSelector := false
	seenField := false
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case "annotation":
			if seenAnnotation {
				return schemaCompile(ErrSchemaContentModel, "identity constraint can contain at most one annotation")
			}
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, "identity constraint annotation must be first")
			}
			seenAnnotation = true
		case "selector":
			if seenSelector {
				return schemaCompile(ErrSchemaContentModel, "identity constraint can contain at most one selector")
			}
			if seenField {
				return schemaCompile(ErrSchemaContentModel, "identity constraint selector must precede fields")
			}
			if err := validateIdentityXPathChild(child, "selector"); err != nil {
				return err
			}
			seenSelector = true
			seenNonAnnotation = true
		case "field":
			if !seenSelector {
				return schemaCompile(ErrSchemaContentModel, "identity constraint field requires selector")
			}
			if err := validateIdentityXPathChild(child, "field"); err != nil {
				return err
			}
			seenField = true
			seenNonAnnotation = true
		default:
			return schemaCompile(ErrSchemaContentModel, "invalid identity constraint child "+child.Name.Local)
		}
	}
	if !seenSelector {
		return schemaCompile(ErrSchemaIdentity, "identity constraint missing selector")
	}
	if !seenField {
		return schemaCompile(ErrSchemaIdentity, "identity constraint missing fields")
	}
	return nil
}

func validateIdentityXPathChild(n *rawNode, label string) error {
	if err := validateKnownAttributes(n, label, map[string]bool{"id": true, "xpath": true}); err != nil {
		return err
	}
	xpath, ok := n.attr("xpath")
	if !ok {
		return schemaCompile(ErrSchemaIdentity, label+" missing xpath")
	}
	if strings.TrimSpace(xpath) == "" {
		return schemaCompile(ErrSchemaIdentity, label+" xpath is empty")
	}
	seenAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		if child.Name.Local != "annotation" {
			return schemaCompile(ErrSchemaContentModel, label+" can contain only annotation")
		}
		if seenAnnotation {
			return schemaCompile(ErrSchemaContentModel, label+" can contain at most one annotation")
		}
		seenAnnotation = true
	}
	return nil
}

func (c *compiler) parseIdentityPaths(n *rawNode, xpath string) ([]identityPath, error) {
	parts := strings.Split(xpath, "|")
	out := make([]identityPath, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, schemaCompile(ErrSchemaIdentity, "identity selector XPath branch is empty")
		}
		desc := false
		if rest, ok := parseIdentityDescendantPrefix(part); ok {
			desc = true
			part = rest
		}
		if part == "." && !desc {
			out = append(out, identityPath{Self: true})
			continue
		}
		steps, err := c.parseIdentitySteps(n, part)
		if err != nil {
			return nil, err
		}
		out = append(out, identityPath{Descendant: desc, Steps: steps})
	}
	return out, nil
}

func (c *compiler) parseIdentityFieldPaths(n *rawNode, xpath string) ([]identityFieldPath, error) {
	parts := strings.Split(xpath, "|")
	out := make([]identityFieldPath, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, schemaCompile(ErrSchemaIdentity, "identity field XPath branch is empty")
		}
		desc := false
		if rest, ok := parseIdentityDescendantPrefix(part); ok {
			desc = true
			part = rest
		}
		if part == "" {
			return nil, schemaCompile(ErrSchemaIdentity, "identity field XPath branch is empty")
		}
		if part == "." && !desc {
			out = append(out, identityFieldPath{Self: true})
			continue
		}
		attr := false
		var attrName qName
		attrWildcard := false
		attrNamespaceSet := false
		var attrNamespace namespaceID
		if strings.Contains(part, "@") {
			elementPath, name, ok := strings.Cut(part, "@")
			if !ok || name == "" {
				return nil, schemaCompile(ErrSchemaIdentity, "invalid identity field XPath "+part)
			}
			attr = true
			part = strings.TrimSuffix(elementPath, "/")
			if elementPath != "" && part == "" {
				return nil, schemaCompile(ErrSchemaIdentity, "identity field XPath has empty element path")
			}
			var err error
			attrName, attrWildcard, attrNamespaceSet, attrNamespace, err = c.parseIdentityAttributeName(n, name)
			if err != nil {
				return nil, err
			}
		} else {
			elementPath, name, ok, err := parseIdentityAttributeAxis(part)
			if err != nil {
				return nil, err
			}
			if ok {
				attr = true
				part = elementPath
				attrName, attrWildcard, attrNamespaceSet, attrNamespace, err = c.parseIdentityAttributeName(n, name)
				if err != nil {
					return nil, err
				}
			}
		}
		var steps []identityStep
		var err error
		if part != "" {
			steps, err = c.parseIdentitySteps(n, part)
			if err != nil {
				return nil, err
			}
		}
		out = append(out, identityFieldPath{Descendant: desc, Attr: attr, AttrWildcard: attrWildcard, AttrNamespaceSet: attrNamespaceSet, AttrNamespace: attrNamespace, Steps: steps, Attribute: attrName})
	}
	return out, nil
}

func parseIdentityDescendantPrefix(path string) (string, bool) {
	switch {
	case strings.HasPrefix(path, ".//"):
		return strings.TrimSpace(strings.TrimPrefix(path, ".//")), true
	case strings.HasPrefix(path, ". //"):
		return strings.TrimSpace(strings.TrimPrefix(path, ". //")), true
	default:
		return path, false
	}
}

func (c *compiler) parseIdentityAttributeName(n *rawNode, name string) (qName, bool, bool, namespaceID, error) {
	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, " \t\r\n") {
		return qName{}, false, false, 0, schemaCompile(ErrSchemaReference, "invalid QName "+name)
	}
	switch name {
	case "*":
		return qName{}, true, false, 0, nil
	default:
		prefix, local, ok := strings.Cut(name, ":")
		if ok && local == "*" {
			if prefix == "" || strings.Contains(prefix, ":") {
				return qName{}, false, false, 0, schemaCompile(ErrSchemaReference, "invalid QName "+name)
			}
			ns, ok := n.NS[prefix]
			if !ok {
				return qName{}, false, false, 0, schemaCompile(ErrSchemaReference, "unbound QName prefix "+prefix)
			}
			return qName{}, true, true, c.rt.Names.InternNamespace(ns), nil
		}
		q, err := c.resolveXPathQName(n, name)
		return q, false, false, 0, err
	}
}

func (c *compiler) parseIdentityNameTest(n *rawNode, lexical string) (identityStep, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "*" {
		return identityStep{wildcard: true}, nil
	}
	if strings.ContainsAny(lexical, " \t\r\n") {
		return identityStep{}, schemaCompile(ErrSchemaReference, "invalid QName "+lexical)
	}
	prefix, local, ok := strings.Cut(lexical, ":")
	if ok && local == "*" {
		if prefix == "" || strings.Contains(prefix, ":") {
			return identityStep{}, schemaCompile(ErrSchemaReference, "invalid QName "+lexical)
		}
		ns, ok := n.NS[prefix]
		if !ok {
			return identityStep{}, schemaCompile(ErrSchemaReference, "unbound QName prefix "+prefix)
		}
		return identityStep{wildcard: true, NamespaceSet: true, Namespace: c.rt.Names.InternNamespace(ns)}, nil
	}
	q, err := c.resolveXPathQName(n, lexical)
	if err != nil {
		return identityStep{}, err
	}
	return identityStep{Name: q}, nil
}

func parseIdentityAttributeAxis(path string) (string, string, bool, error) {
	idx := strings.LastIndex(path, "/")
	elementPath := ""
	step := path
	if idx >= 0 {
		elementPath = path[:idx]
		step = path[idx+1:]
	}
	name, ok := parseIdentityAxisStep(step, "attribute")
	if !ok {
		return "", "", false, nil
	}
	if name == "" {
		return "", "", true, schemaCompile(ErrSchemaIdentity, "invalid identity field XPath "+path)
	}
	return elementPath, name, true, nil
}

func (c *compiler) parseIdentitySteps(n *rawNode, path string) ([]identityStep, error) {
	if strings.Contains(path, "@") {
		return nil, schemaCompile(ErrSchemaIdentity, "invalid identity XPath "+path)
	}
	if strings.ContainsAny(path, "[]()") || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return nil, schemaCompile(ErrSchemaIdentity, "invalid identity XPath "+path)
	}
	parts := strings.Split(path, "/")
	steps := make([]identityStep, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, schemaCompile(ErrSchemaIdentity, "invalid identity XPath step")
		}
		if strings.Contains(part, "::") {
			name, ok := parseIdentityAxisStep(part, "child")
			if !ok {
				return nil, schemaCompile(ErrSchemaIdentity, "invalid identity XPath step "+part)
			}
			part = name
			if name == "" {
				return nil, schemaCompile(ErrSchemaIdentity, "invalid identity XPath step child::")
			}
		}
		if part == "." {
			continue
		}
		step, err := c.parseIdentityNameTest(n, part)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func parseIdentityAxisStep(part, axis string) (string, bool) {
	part = strings.TrimSpace(part)
	if !strings.HasPrefix(part, axis) {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(part, axis))
	if !strings.HasPrefix(rest, "::") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(rest, "::")), true
}

func (c *compiler) resolveXPathQName(n *rawNode, lexical string) (qName, error) {
	prefix, local, ok := strings.Cut(lexical, ":")
	if !ok {
		return c.rt.Names.InternQName("", lexical), nil
	}
	if prefix == "" || local == "" || strings.Contains(local, ":") {
		return qName{}, schemaCompile(ErrSchemaReference, "invalid QName "+lexical)
	}
	ns, ok := n.NS[prefix]
	if !ok {
		return qName{}, schemaCompile(ErrSchemaReference, "unbound QName prefix "+prefix)
	}
	return c.rt.Names.InternQName(ns, local), nil
}
