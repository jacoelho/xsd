package xsd

import (
	"bytes"
	"cmp"
	"errors"
	"hash/maphash"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
)

type schemaLoad struct {
	source          SchemaSource
	optionalMissing bool
}

func (c *compiler) load(sources []SchemaSource) error {
	if err := c.loadSchemaDocuments(sources); err != nil {
		return err
	}
	return c.checkExplicitSchemaReferences()
}

func (c *compiler) loadSchemaDocuments(sources []SchemaSource) error {
	queue := make([]schemaLoad, 0, len(sources))
	for _, source := range sources {
		queue = append(queue, schemaLoad{source: source})
	}
	sourceData, err := c.readSchemaLoadQueue(queue)
	if err != nil {
		return err
	}
	c.appendLoadedSchemaDocuments(sourceData)
	return nil
}

func (c *compiler) readSchemaLoadQueue(queue []schemaLoad) (map[string][]byte, error) {
	sourceData := make(map[string][]byte)
	for len(queue) != 0 {
		item := queue[0]
		queue = queue[1:]
		next, err := c.readSchemaLoad(item, sourceData)
		if err != nil {
			return nil, err
		}
		queue = append(queue, next...)
	}
	return sourceData, nil
}

func (c *compiler) readSchemaLoad(item schemaLoad, sourceData map[string][]byte) ([]schemaLoad, error) {
	s := item.source
	if s.name == "" {
		return nil, schemaCompile(ErrSchemaRead, "schema source name is required")
	}
	name := s.name
	key := schemaSourceKey(name)
	if _, ok := c.sourceDocs[key]; ok {
		return nil, nil
	}
	data, err := s.read(c.limits.maxSchemaSourceBytes)
	if err != nil {
		if item.optionalMissing && (errors.Is(err, ErrSchemaNotFound) || os.IsNotExist(err)) {
			return nil, nil
		}
		if isSchemaLimitError(err) {
			return nil, err
		}
		return nil, &Error{Category: SchemaParseErrorCategory, Code: ErrSchemaRead, Message: "read schema " + s.name, Err: err}
	}
	doc, err := parseSchemaDocument(name, key, data, c.limits)
	if err != nil {
		return nil, err
	}
	sourceData[key] = data
	c.sourceDocs[key] = doc
	if s.resolver == nil {
		return nil, nil
	}
	return c.resolveSchemaRefs(s, key, doc)
}

func (c *compiler) resolveSchemaRefs(s SchemaSource, key string, doc *rawDoc) ([]schemaLoad, error) {
	var queue []schemaLoad
	for _, ref := range schemaDocumentRefs(doc) {
		if ref.namespace == xmlNamespaceURI {
			continue
		}
		next, err := s.resolver.ResolveSchema(s.name, ref.location)
		if err != nil {
			if errors.Is(err, ErrSchemaNotFound) {
				continue
			}
			return nil, &Error{Category: SchemaParseErrorCategory, Code: ErrSchemaRead, Message: "resolve schema " + ref.location, Err: err}
		}
		if next.resolver == nil {
			next.resolver = s.resolver
		}
		if next.name != "" {
			c.resolvedRef[schemaReferenceKey{base: key, location: ref.location}] = schemaSourceKey(next.name)
		}
		queue = append(queue, schemaLoad{source: next, optionalMissing: true})
	}
	return queue, nil
}

func (c *compiler) appendLoadedSchemaDocuments(sourceData map[string][]byte) {
	names := slices.Sorted(maps.Keys(c.sourceDocs))
	targetCounts, hasDuplicateTargets := c.schemaTargetCounts(names)
	if !hasDuplicateTargets {
		for _, name := range names {
			c.docs = append(c.docs, c.sourceDocs[name])
		}
	} else {
		seed := maphash.MakeSeed()
		seenContent := make(map[schemaContentKey][][]byte)
		for _, name := range names {
			data := sourceData[name]
			doc := c.sourceDocs[name]
			if target := doc.root.attrDefault(xsdAttrTargetNamespace, ""); target != "" && targetCounts[target] > 1 {
				key := schemaContentKey{target: target, size: len(data), hash: maphash.Bytes(seed, data)}
				if schemaContentSeen(seenContent[key], data) {
					continue
				}
				seenContent[key] = append(seenContent[key], data)
			}
			c.docs = append(c.docs, doc)
		}
	}
	slices.SortFunc(c.docs, func(a, b *rawDoc) int {
		return cmp.Compare(a.name, b.name)
	})
}

func (c *compiler) schemaTargetCounts(names []string) (map[string]int, bool) {
	if len(names) < 2 {
		return nil, false
	}
	counts := make(map[string]int, len(names))
	hasDuplicate := false
	for _, name := range names {
		target := c.sourceDocs[name].root.attrDefault(xsdAttrTargetNamespace, "")
		if target == "" {
			continue
		}
		counts[target]++
		if counts[target] == 2 {
			hasDuplicate = true
		}
	}
	return counts, hasDuplicate
}

type schemaContentKey struct {
	target string
	size   int
	hash   uint64
}

func schemaContentSeen(bucket [][]byte, data []byte) bool {
	for _, item := range bucket {
		if bytes.Equal(item, data) {
			return true
		}
	}
	return false
}

func isSchemaLimitError(err error) bool {
	x, ok := errors.AsType[*Error](err)
	return ok && x.Code == ErrSchemaLimit
}

type schemaDocumentRef struct {
	namespace string
	location  string
}

func schemaDocumentRefs(doc *rawDoc) []schemaDocumentRef {
	var refs []schemaDocumentRef
	for _, child := range doc.root.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case xsdElemInclude, xsdElemImport:
			location, ok := schemaLocationAttr(child)
			if !ok {
				continue
			}
			ref := schemaDocumentRef{location: location}
			if child.Name.Local == xsdElemImport {
				ref.namespace = child.attrDefault(xsdAttrNamespace, "")
			}
			refs = append(refs, ref)
		}
	}
	return refs
}

func schemaLocationAttr(n *rawNode) (string, bool) {
	location, ok := n.attr(xsdAttrSchemaLocation)
	if !ok {
		return "", false
	}
	location = collapseXMLWhitespace(location)
	return location, location != ""
}

func (c *compiler) checkExplicitSchemaReferences() error {
	if err := c.propagateChameleonTargets(); err != nil {
		return err
	}
	for _, doc := range c.docs {
		for _, child := range doc.root.Children {
			if child.Name.Space != xsdNamespaceURI {
				continue
			}
			switch child.Name.Local {
			case xsdElemInclude:
				if err := c.checkExplicitInclude(doc, child); err != nil {
					return err
				}
			case xsdElemImport:
				if err := c.checkExplicitImport(doc, child); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *compiler) checkExplicitInclude(doc *rawDoc, child *rawNode) error {
	location, ok := schemaLocationAttr(child)
	if !ok {
		return schemaCompileAt(child, ErrSchemaReference, "include missing schemaLocation")
	}
	_, err := c.adoptChameleonInclude(doc, location, c.documentTargetNamespace(doc))
	return withSchemaCompileLocation(child, err)
}

func (c *compiler) checkExplicitImport(doc *rawDoc, child *rawNode) error {
	namespace, err := c.registerExplicitImportNamespace(doc, child)
	if err != nil {
		return err
	}
	location, ok := schemaLocationAttr(child)
	if !ok {
		return nil
	}
	referenced, _, ok := c.resolveLoadedSchemaLocation(doc, location)
	if !ok {
		return nil
	}
	if namespace != referenced.root.attrDefault(xsdAttrTargetNamespace, "") {
		return schemaCompileAt(child, ErrSchemaReference, "import namespace does not match imported schema targetNamespace")
	}
	return nil
}

func (c *compiler) registerExplicitImportNamespace(doc *rawDoc, child *rawNode) (string, error) {
	namespace, hasNamespace := child.attr(xsdAttrNamespace)
	target := c.documentTargetNamespace(doc)
	if hasNamespace && namespace == "" {
		return "", schemaCompileAt(child, ErrSchemaInvalidAttribute, "import namespace cannot be empty")
	}
	if !hasNamespace && target == "" {
		return "", schemaCompileAt(child, ErrSchemaReference, "import without namespace requires enclosing schema targetNamespace")
	}
	if hasNamespace && namespace == target {
		return "", schemaCompileAt(child, ErrSchemaReference, "import namespace cannot match enclosing schema targetNamespace")
	}
	if c.imports[doc.key] == nil {
		c.imports[doc.key] = make(map[string]bool)
	}
	c.imports[doc.key][namespace] = true
	return namespace, nil
}

func (c *compiler) propagateChameleonTargets() error {
	for {
		changed := false
		for _, doc := range c.docs {
			docChanged, err := c.propagateChameleonTarget(doc)
			if err != nil {
				return err
			}
			if docChanged {
				changed = true
			}
		}
		if !changed {
			return nil
		}
	}
}

func (c *compiler) propagateChameleonTarget(doc *rawDoc) (bool, error) {
	target := c.documentTargetNamespace(doc)
	if target == "" {
		return false, nil
	}
	changed := false
	for _, child := range doc.root.Children {
		if child.Name.Space != xsdNamespaceURI || child.Name.Local != xsdElemInclude {
			continue
		}
		location, ok := schemaLocationAttr(child)
		if !ok {
			continue
		}
		adopted, err := c.adoptChameleonInclude(doc, location, target)
		if err != nil {
			return false, withSchemaCompileLocation(child, err)
		}
		if adopted {
			changed = true
		}
	}
	return changed, nil
}

func (c *compiler) adoptChameleonInclude(doc *rawDoc, location, target string) (bool, error) {
	referenced, resolved, ok := c.resolveLoadedSchemaLocation(doc, location)
	if !ok {
		return false, nil
	}
	referencedTarget := referenced.root.attrDefault(xsdAttrTargetNamespace, "")
	if referencedTarget != "" && referencedTarget != target {
		return false, schemaCompile(ErrSchemaReference, "included schema targetNamespace does not match including schema")
	}
	if target == "" || referencedTarget != "" {
		return false, nil
	}
	existing := c.adoptTarget[resolved]
	if existing == "" {
		c.adoptTarget[resolved] = target
		return true, nil
	}
	if existing == target {
		return false, nil
	}
	cloneKey := resolved + "\x00" + target
	if _, ok := c.adoptTarget[cloneKey]; ok {
		return false, nil
	}
	c.cloneAdoptedChameleon(referenced, cloneKey, target)
	return true, nil
}

// cloneAdoptedChameleon registers a per-namespace copy of a chameleon schema
// document that is already adopted by another namespace. The copy gets its
// own node tree because compilation memoizes per *rawNode, and pre-resolved
// include/import locations because path-based resolution cannot work on the
// synthetic clone key. Clones are never added to sourceDocs: location
// resolution always finds original documents, and the adoptTarget entry for
// the clone key both records the namespace and deduplicates clone creation.
func (c *compiler) cloneAdoptedChameleon(orig *rawDoc, cloneKey, target string) {
	clone := &rawDoc{root: cloneRawTree(orig.root), name: orig.name, key: cloneKey}
	c.adoptTarget[cloneKey] = target
	for _, ref := range schemaDocumentRefs(orig) {
		if _, resolved, ok := c.resolveLoadedSchemaLocation(orig, ref.location); ok {
			c.resolvedRef[schemaReferenceKey{base: cloneKey, location: ref.location}] = resolved
		}
	}
	c.docs = append(c.docs, clone)
}

// cloneRawTree copies the node structure. Per-node payloads (NS maps, Attr
// slices, text, names, positions) are immutable after parse and stay shared.
func cloneRawTree(n *rawNode) *rawNode {
	copied := *n
	if len(n.Children) > 0 {
		copied.Children = make([]*rawNode, len(n.Children))
		for i, child := range n.Children {
			copied.Children[i] = cloneRawTree(child)
		}
	}
	return &copied
}

func (c *compiler) documentTargetNamespace(doc *rawDoc) string {
	if target := doc.root.attrDefault(xsdAttrTargetNamespace, ""); target != "" {
		return target
	}
	return c.adoptTarget[doc.key]
}

func (c *compiler) resolveLoadedSchemaLocation(doc *rawDoc, location string) (*rawDoc, string, bool) {
	try := func(resolved string) (*rawDoc, string, bool) {
		if loadedDoc, loaded := c.sourceDocs[resolved]; loaded {
			return loadedDoc, resolved, true
		}
		return nil, "", false
	}
	loc := collapseXMLWhitespace(location)
	if resolved, ok := c.resolvedRef[schemaReferenceKey{base: doc.key, location: loc}]; ok {
		if referenced, key, ok := try(resolved); ok {
			return referenced, key, true
		}
	}
	if loc == "" {
		return nil, "", false
	}
	for _, resolved := range schemaLocationKeys(doc.name, doc.key, loc) {
		if referenced, key, ok := try(resolved); ok {
			return referenced, key, true
		}
	}
	return nil, "", false
}

func schemaSourceKey(name string) string {
	if filepath.VolumeName(name) != "" {
		return filepath.Clean(name)
	}
	u, err := url.Parse(name)
	if err == nil && u.Scheme != "" {
		if path, ok := localFileURIPath(u); ok {
			return path
		}
		if u.Opaque != "" {
			return name
		}
		if u.Host != "" || u.Path != "" {
			if u.Path != "" {
				u.Path = path.Clean(u.Path)
				if u.Path == "." {
					u.Path = ""
				}
			}
			return u.String()
		}
	}
	return filepath.Clean(name)
}

func schemaLocationKeys(baseName, baseKey, loc string) []string {
	var keys []string
	add := func(key string) {
		if slices.Contains(keys, key) {
			return
		}
		keys = append(keys, key)
	}
	baseURL, baseURLErr := url.Parse(baseName)
	baseIsURL := baseURLErr == nil && baseURL.Scheme != "" && baseURL.Opaque == "" && (baseURL.Host != "" || baseURL.Path != "")
	if baseIsURL {
		if ref, err := url.Parse(loc); err == nil && ref.Opaque == "" && (ref.Scheme == "" || ref.Host != "" || ref.Path != "") {
			add(schemaSourceKey(baseURL.ResolveReference(ref).String()))
		}
	}
	if !baseIsURL {
		if resolved, ok := resolveLocalSchemaLocation(baseKey, loc); ok {
			add(schemaSourceKey(resolved))
		}
	}
	add(schemaSourceKey(loc))
	return keys
}
