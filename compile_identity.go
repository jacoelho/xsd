package xsd

import "strings"

func identityConstraintNodes(n *rawNode) []*rawNode {
	var nodes []*rawNode
	for _, child := range n.xsContentChildren() {
		switch child.Name.Local {
		case xsdElemKey, xsdElemKeyref, xsdElemUnique:
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
	if n.Name.Space == xsdNamespaceURI && n.Name.Local == xsdElemElement {
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
		name, ok := node.attr(xsdAttrName)
		if !ok || name == "" {
			return nil, schemaCompile(ErrSchemaIdentity, "identity constraint missing name")
		}
		q, err := c.rt.Names.InternQName(ctx.targetNS, name)
		if err != nil {
			return nil, err
		}
		if _, exists := c.rt.GlobalIdentities[q]; exists {
			return nil, schemaCompile(ErrSchemaDuplicate, "duplicate identity constraint "+c.rt.Names.Format(q))
		}
		id, err := nextIdentityConstraintID(len(c.rt.Identities))
		if err != nil {
			return nil, err
		}
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
	case xsdElemKey:
		ic.Kind = identityKey
	case xsdElemUnique:
		ic.Kind = identityUnique
	case xsdElemKeyref:
		ic.Kind = identityKeyRef
		refer, ok := n.attr(xsdAttrRefer)
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
	selector := n.firstXS(xsdElemSelector)
	if selector == nil {
		return ic, schemaCompile(ErrSchemaIdentity, "identity constraint missing selector")
	}
	xpath, ok := selector.attr(xsdAttrXPath)
	if !ok {
		return ic, schemaCompile(ErrSchemaIdentity, "selector missing xpath")
	}
	paths, err := c.parseIdentityPaths(selector, xpath)
	if err != nil {
		return ic, err
	}
	ic.Selector = paths
	for _, field := range n.xsChildren(xsdElemField) {
		xpath, ok := field.attr(xsdAttrXPath)
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
	compileIdentityFieldLookup(&ic)
	return ic, nil
}

// compileIdentityFieldLookup materializes field lookup tables after path parsing.
func compileIdentityFieldLookup(ic *identityConstraint) {
	ic.ElementFields, ic.AttributeFields, ic.AttributeWildcardFields = buildIdentityFieldLookup(ic.Fields)
}

func buildIdentityFieldLookup(fields []identityField) ([]compiledIdentityField, map[qName][]compiledIdentityField, []compiledIdentityField) {
	var elementFields []compiledIdentityField
	var attrFields map[qName][]compiledIdentityField
	var attrWildcardFields []compiledIdentityField
	for fieldIndex := range fields {
		var elementPaths []identityFieldPath
		var wildcardAttrPaths []identityFieldPath
		var exactAttrPaths map[qName][]identityFieldPath
		for _, path := range fields[fieldIndex].Paths {
			if !path.Attr {
				elementPaths = append(elementPaths, path)
				continue
			}
			if path.AttrWildcard {
				wildcardAttrPaths = append(wildcardAttrPaths, path)
				continue
			}
			if exactAttrPaths == nil {
				exactAttrPaths = make(map[qName][]identityFieldPath)
			}
			exactAttrPaths[path.Attribute] = append(exactAttrPaths[path.Attribute], path)
		}
		if len(elementPaths) != 0 {
			elementFields = append(elementFields, compiledIdentityField{
				Field: fieldIndex,
				Paths: elementPaths,
			})
		}
		if len(wildcardAttrPaths) != 0 {
			attrWildcardFields = append(attrWildcardFields, compiledIdentityField{
				Field: fieldIndex,
				Paths: wildcardAttrPaths,
			})
		}
		for name, paths := range exactAttrPaths {
			if attrFields == nil {
				attrFields = make(map[qName][]compiledIdentityField)
			}
			attrFields[name] = append(attrFields[name], compiledIdentityField{
				Field: fieldIndex,
				Paths: paths,
			})
		}
	}
	return elementFields, attrFields, attrWildcardFields
}

func validateIdentityConstraintSyntax(n *rawNode) error {
	seenAnnotation := false
	seenSelector := false
	seenField := false
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case xsdElemAnnotation:
			if seenAnnotation {
				return schemaCompile(ErrSchemaContentModel, "identity constraint can contain at most one annotation")
			}
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, "identity constraint annotation must be first")
			}
			seenAnnotation = true
		case xsdElemSelector:
			if seenSelector {
				return schemaCompile(ErrSchemaContentModel, "identity constraint can contain at most one selector")
			}
			if seenField {
				return schemaCompile(ErrSchemaContentModel, "identity constraint selector must precede fields")
			}
			if err := validateIdentityXPathChild(child, xsdElemSelector); err != nil {
				return err
			}
			seenSelector = true
			seenNonAnnotation = true
		case xsdElemField:
			if !seenSelector {
				return schemaCompile(ErrSchemaContentModel, "identity constraint field requires selector")
			}
			if err := validateIdentityXPathChild(child, xsdElemField); err != nil {
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
	xpath, ok := n.attr(xsdAttrXPath)
	if !ok {
		return schemaCompile(ErrSchemaIdentity, label+" missing xpath")
	}
	if trimXMLWhitespace(xpath) == "" {
		return schemaCompile(ErrSchemaIdentity, label+" xpath is empty")
	}
	seenAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		if child.Name.Local != xsdElemAnnotation {
			return schemaCompile(ErrSchemaContentModel, label+" can contain only annotation")
		}
		if seenAnnotation {
			return schemaCompile(ErrSchemaContentModel, label+" can contain at most one annotation")
		}
		seenAnnotation = true
	}
	return nil
}

func isIdentityAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrName:
		return true
	default:
		return false
	}
}

func isKeyrefAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrName, xsdAttrRefer:
		return true
	default:
		return false
	}
}

func isIdentityXPathAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrXPath:
		return true
	default:
		return false
	}
}

func (c *compiler) parseIdentityPaths(n *rawNode, xpath string) ([]identityPath, error) {
	out := make([]identityPath, 0, strings.Count(xpath, "|")+1)
	for part := range strings.SplitSeq(xpath, "|") {
		part = trimXMLWhitespace(part)
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
	out := make([]identityFieldPath, 0, strings.Count(xpath, "|")+1)
	for part := range strings.SplitSeq(xpath, "|") {
		part = trimXMLWhitespace(part)
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
		var attrName identityAttributeName
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
			attrName, err = c.parseIdentityAttributeName(n, name)
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
				attrName, err = c.parseIdentityAttributeName(n, name)
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
		out = append(out, identityFieldPath{
			Descendant:       desc,
			Attr:             attr,
			AttrWildcard:     attrName.wildcard,
			AttrNamespaceSet: attrName.namespaceSet,
			AttrNamespace:    attrName.namespace,
			Steps:            steps,
			Attribute:        attrName.name,
		})
	}
	return out, nil
}

func parseIdentityDescendantPrefix(path string) (string, bool) {
	if rest, ok := strings.CutPrefix(path, ".//"); ok {
		return trimXMLWhitespace(rest), true
	}
	if rest, ok := strings.CutPrefix(path, ". //"); ok {
		return trimXMLWhitespace(rest), true
	}
	return path, false
}

type identityAttributeName struct {
	name         qName
	namespace    namespaceID
	wildcard     bool
	namespaceSet bool
}

func (c *compiler) parseIdentityAttributeName(n *rawNode, name string) (identityAttributeName, error) {
	parsed, err := c.parseIdentityNameTestParts(n, name)
	if err != nil {
		return identityAttributeName{}, err
	}
	return identityAttributeName(parsed), nil
}

func (c *compiler) parseIdentityNameTest(n *rawNode, lexical string) (identityStep, error) {
	parsed, err := c.parseIdentityNameTestParts(n, lexical)
	if err != nil {
		return identityStep{}, err
	}
	return identityStep{
		Name:         parsed.name,
		Namespace:    parsed.namespace,
		wildcard:     parsed.wildcard,
		NamespaceSet: parsed.namespaceSet,
	}, nil
}

type identityNameTest struct {
	name         qName
	namespace    namespaceID
	wildcard     bool
	namespaceSet bool
}

func (c *compiler) parseIdentityNameTestParts(n *rawNode, lexical string) (identityNameTest, error) {
	lexical = trimXMLWhitespace(lexical)
	if lexical == "*" {
		return identityNameTest{wildcard: true}, nil
	}
	prefix, wildcard, err := parseQNamePrefixWildcard(lexical)
	if err != nil {
		return identityNameTest{}, err
	}
	if wildcard {
		ns, ok := n.NS[prefix]
		if !ok {
			return identityNameTest{}, schemaCompile(ErrSchemaReference, "unbound QName prefix "+prefix)
		}
		nsID, nsErr := c.rt.Names.InternNamespace(ns)
		if nsErr != nil {
			return identityNameTest{}, nsErr
		}
		return identityNameTest{wildcard: true, namespaceSet: true, namespace: nsID}, nil
	}
	q, err := c.resolveXPathQName(n, lexical)
	if err != nil {
		return identityNameTest{}, err
	}
	return identityNameTest{name: q}, nil
}

func parseIdentityAttributeAxis(path string) (string, string, bool, error) {
	idx := strings.LastIndex(path, "/")
	elementPath := ""
	step := path
	if idx >= 0 {
		elementPath = path[:idx]
		step = path[idx+1:]
	}
	name, ok := parseIdentityAxisStep(step, xsdElemAttribute)
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
	steps := make([]identityStep, 0, strings.Count(path, "/")+1)
	for part := range strings.SplitSeq(path, "/") {
		part = trimXMLWhitespace(part)
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
	part = trimXMLWhitespace(part)
	rest, ok := strings.CutPrefix(part, axis)
	if !ok {
		return "", false
	}
	rest = trimXMLWhitespace(rest)
	rest, ok = strings.CutPrefix(rest, "::")
	if !ok {
		return "", false
	}
	return trimXMLWhitespace(rest), true
}

func (c *compiler) resolveXPathQName(n *rawNode, lexical string) (qName, error) {
	prefix, local, prefixed, err := parseQNameParts(lexical)
	if err != nil {
		return qName{}, err
	}
	if !prefixed {
		return c.rt.Names.InternQName("", local)
	}
	ns, ok := n.NS[prefix]
	if !ok {
		return qName{}, schemaCompile(ErrSchemaReference, "unbound QName prefix "+prefix)
	}
	return c.rt.Names.InternQName(ns, local)
}
