package xsd

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

func (c *compiler) load(sources []SchemaSource) error {
	queue := append([]SchemaSource(nil), sources...)
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
		data, err := s.read()
		if err != nil {
			return &Error{Category: SchemaParseErrorCategory, Code: ErrSchemaRead, Message: "read schema " + s.name, Err: err}
		}
		c.sources[name] = data
		if s.resolver == nil {
			continue
		}
		doc, err := parseSchemaDocument(name, data, c.limits)
		if err != nil {
			return err
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
	seenContent := make(map[string]bool)
	for _, name := range names {
		data := c.sources[name]
		doc, err := parseSchemaDocument(name, data, c.limits)
		if err != nil {
			return err
		}
		if target := doc.root.attrDefault("targetNamespace", ""); target != "" {
			key := target + "\x00" + string(data)
			if seenContent[key] {
				continue
			}
			seenContent[key] = true
		}
		c.docs = append(c.docs, doc)
	}
	slices.SortFunc(c.docs, func(a, b *rawDoc) int {
		return strings.Compare(a.name, b.name)
	})
	return c.checkExplicitSchemaReferences()
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
					if !hasNamespace && doc.root.attrDefault("targetNamespace", "") == "" {
						return schemaCompile(ErrSchemaReference, "import without namespace requires enclosing schema targetNamespace")
					}
					if hasNamespace && namespace == doc.root.attrDefault("targetNamespace", "") {
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
				if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
					continue
				}
				resolved := filepath.Clean(filepath.Join(filepath.Dir(doc.name), location))
				if _, ok := c.sources[resolved]; !ok {
					if _, ok := c.sources[filepath.Clean(location)]; !ok {
						continue
					}
					resolved = filepath.Clean(location)
				}
				referenced, err := parseSchemaDocument(resolved, c.sources[resolved], c.limits)
				if err != nil {
					return err
				}
				referencedTarget := referenced.root.attrDefault("targetNamespace", "")
				if child.Name.Local == "include" {
					target := doc.root.attrDefault("targetNamespace", "")
					if target == "" {
						target = c.adoptTarget[doc.name]
					}
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
