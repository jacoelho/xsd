package xsd

import (
	"bytes"
	"cmp"
	"errors"
	"hash/maphash"
	"maps"
	"path/filepath"
	"slices"
)

func (c *compiler) load(sources []SchemaSource) error {
	queue := slices.Clone(sources)
	for len(queue) != 0 {
		s := queue[0]
		queue = queue[1:]
		if s.name == "" {
			return schemaCompile(ErrSchemaRead, "schema source name is required")
		}
		name := filepath.Clean(s.name)
		if _, ok := c.sources[name]; ok {
			continue
		}
		data, err := s.read(c.limits.maxSchemaSourceBytes)
		if err != nil {
			if isSchemaLimitError(err) {
				return err
			}
			return &Error{Category: SchemaParseErrorCategory, Code: ErrSchemaRead, Message: "read schema " + s.name, Err: err}
		}
		doc, err := parseSchemaDocument(name, data, c.limits)
		if err != nil {
			return err
		}
		c.sources[name] = data
		c.sourceDocs[name] = doc
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
			queue = append(queue, next)
		}
	}
	names := slices.Sorted(maps.Keys(c.sources))
	targetCounts, hasDuplicateTargets := c.schemaTargetCounts(names)
	if !hasDuplicateTargets {
		for _, name := range names {
			c.docs = append(c.docs, c.sourceDocs[name])
		}
	} else {
		seed := maphash.MakeSeed()
		seenContent := make(map[schemaContentKey][][]byte)
		for _, name := range names {
			data := c.sources[name]
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
	return c.checkExplicitSchemaReferences()
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
			if !ok || location == "" {
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
					if c.imports[doc.name] == nil {
						c.imports[doc.name] = make(map[string]bool)
					}
					c.imports[doc.name][importNamespace] = true
				}
				location, ok := child.attr("schemaLocation")
				if !ok || location == "" {
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
				if !ok || location == "" {
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
	return c.adoptTarget[doc.name]
}

func (c *compiler) resolveLoadedSchemaLocation(doc *rawDoc, location string) (*rawDoc, string, bool) {
	if resolved, ok := resolveLocalSchemaLocation(doc.name, location); ok {
		if _, loaded := c.sources[resolved]; loaded {
			return c.sourceDocs[resolved], resolved, true
		}
	}
	resolved := filepath.Clean(location)
	if _, ok := c.sources[resolved]; !ok {
		return nil, "", false
	}
	return c.sourceDocs[resolved], resolved, true
}
