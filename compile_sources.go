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
	queue := make([]schemaLoad, len(sources))
	for i, source := range sources {
		queue[i] = schemaLoad{source: source}
	}
	sourceData := make(map[string][]byte)
	for len(queue) != 0 {
		item := queue[0]
		queue = queue[1:]
		s := item.source
		if s.name == "" {
			return schemaCompile(ErrSchemaRead, "schema source name is required")
		}
		name := s.name
		key := schemaSourceKey(name)
		if _, ok := c.sourceDocs[key]; ok {
			continue
		}
		data, err := s.read(c.limits.maxSchemaSourceBytes)
		if err != nil {
			if item.optionalMissing && (errors.Is(err, ErrSchemaNotFound) || os.IsNotExist(err)) {
				continue
			}
			if isSchemaLimitError(err) {
				return err
			}
			return &Error{Category: SchemaParseErrorCategory, Code: ErrSchemaRead, Message: "read schema " + s.name, Err: err}
		}
		doc, err := parseSchemaDocument(name, key, data, c.limits)
		if err != nil {
			return err
		}
		sourceData[key] = data
		c.sourceDocs[key] = doc
		if s.resolver == nil {
			continue
		}
		for _, ref := range schemaDocumentRefs(doc) {
			if ref.namespace == xmlNamespaceURI {
				continue
			}
			next, err := s.resolver.ResolveSchema(name, ref.location)
			if err != nil {
				if errors.Is(err, ErrSchemaNotFound) {
					continue
				}
				return &Error{Category: SchemaParseErrorCategory, Code: ErrSchemaRead, Message: "resolve schema " + ref.location, Err: err}
			}
			if next.resolver == nil {
				next.resolver = s.resolver
			}
			if next.name != "" {
				c.resolvedRef[schemaReferenceKey{base: key, location: ref.location}] = schemaSourceKey(next.name)
			}
			queue = append(queue, schemaLoad{source: next, optionalMissing: true})
		}
	}
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
			if target := doc.root.attrDefault("targetNamespace", ""); target != "" && targetCounts[target] > 1 {
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
	return nil
}

func (c *compiler) schemaTargetCounts(names []string) (map[string]int, bool) {
	if len(names) < 2 {
		return nil, false
	}
	counts := make(map[string]int, len(names))
	hasDuplicate := false
	for _, name := range names {
		target := c.sourceDocs[name].root.attrDefault("targetNamespace", "")
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
		case "include", "import":
			location, ok := child.attr("schemaLocation")
			if !ok || trimXMLWhitespace(location) == "" {
				continue
			}
			ref := schemaDocumentRef{location: location}
			if child.Name.Local == "import" {
				ref.namespace = child.attrDefault("namespace", "")
			}
			refs = append(refs, ref)
		}
	}
	return refs
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
			case "include", "import":
				importNamespace := ""
				if child.Name.Local == "import" {
					namespace, hasNamespace := child.attr("namespace")
					if hasNamespace && namespace == "" {
						return schemaCompile(ErrSchemaInvalidAttribute, "import namespace cannot be empty")
					}
					if !hasNamespace && c.documentTargetNamespace(doc) == "" {
						return schemaCompile(ErrSchemaReference, "import without namespace requires enclosing schema targetNamespace")
					}
					if hasNamespace && namespace == c.documentTargetNamespace(doc) {
						return schemaCompile(ErrSchemaReference, "import namespace cannot match enclosing schema targetNamespace")
					}
					importNamespace = namespace
					if c.imports[doc.key] == nil {
						c.imports[doc.key] = make(map[string]bool)
					}
					c.imports[doc.key][importNamespace] = true
				}
				location, ok := child.attr("schemaLocation")
				if !ok || trimXMLWhitespace(location) == "" {
					if child.Name.Local == "include" {
						return schemaCompile(ErrSchemaReference, "include missing schemaLocation")
					}
					continue
				}
				referenced, resolved, ok := c.resolveLoadedSchemaLocation(doc, location)
				if !ok {
					continue
				}
				referencedTarget := referenced.root.attrDefault("targetNamespace", "")
				if child.Name.Local == "include" {
					target := c.documentTargetNamespace(doc)
					if referencedTarget != "" && referencedTarget != target {
						return schemaCompile(ErrSchemaReference, "included schema targetNamespace does not match including schema")
					}
					if target != "" && referencedTarget == "" {
						if existing := c.adoptTarget[resolved]; existing != "" && existing != target {
							return schemaCompile(ErrSchemaReference, "chameleon include used with multiple target namespaces")
						}
						c.adoptTarget[resolved] = target
					}
				}
				if child.Name.Local == "import" {
					if importNamespace != referencedTarget {
						return schemaCompile(ErrSchemaReference, "import namespace does not match imported schema targetNamespace")
					}
				}
			}
		}
	}
	return nil
}

func (c *compiler) propagateChameleonTargets() error {
	for {
		changed := false
		for _, doc := range c.docs {
			target := c.documentTargetNamespace(doc)
			if target == "" {
				continue
			}
			for _, child := range doc.root.Children {
				if child.Name.Space != xsdNamespaceURI || child.Name.Local != "include" {
					continue
				}
				location, ok := child.attr("schemaLocation")
				if !ok || trimXMLWhitespace(location) == "" {
					continue
				}
				referenced, resolved, ok := c.resolveLoadedSchemaLocation(doc, location)
				if !ok {
					continue
				}
				referencedTarget := referenced.root.attrDefault("targetNamespace", "")
				if referencedTarget != "" && referencedTarget != target {
					return schemaCompile(ErrSchemaReference, "included schema targetNamespace does not match including schema")
				}
				if referencedTarget != "" {
					continue
				}
				existing := c.adoptTarget[resolved]
				if existing != "" && existing != target {
					return schemaCompile(ErrSchemaReference, "chameleon include used with multiple target namespaces")
				}
				if existing == "" {
					c.adoptTarget[resolved] = target
					changed = true
				}
			}
		}
		if !changed {
			return nil
		}
	}
}

func (c *compiler) documentTargetNamespace(doc *rawDoc) string {
	if target := doc.root.attrDefault("targetNamespace", ""); target != "" {
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
	if resolved, ok := c.resolvedRef[schemaReferenceKey{base: doc.key, location: location}]; ok {
		if referenced, key, ok := try(resolved); ok {
			return referenced, key, true
		}
	}
	loc := trimXMLWhitespace(location)
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
		for _, existing := range keys {
			if existing == key {
				return
			}
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
