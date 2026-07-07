package compile

import (
	"cmp"
	"maps"
	"slices"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

type compilerSourceState struct {
	sourceDocs  map[string]*rawDoc
	resolvedRef map[source.ReferenceKey]string
	imports     map[string]map[string]bool
	adoptTarget map[string]string
	contexts    map[*rawDoc]*schemaContext
	graphDocs   []*rawDoc
	compileDocs []*rawDoc
}

func newCompilerSourceState() compilerSourceState {
	return compilerSourceState{
		sourceDocs:  make(map[string]*rawDoc),
		resolvedRef: make(map[source.ReferenceKey]string),
		imports:     make(map[string]map[string]bool),
		adoptTarget: make(map[string]string),
		contexts:    make(map[*rawDoc]*schemaContext),
	}
}

func (c *compiler) load(sources []source.Source) error {
	if err := c.loadSchemaDocuments(sources); err != nil {
		return err
	}
	return c.checkExplicitSchemaReferences()
}

func (c *compiler) loadSchemaDocuments(sources []source.Source) error {
	result, err := source.LoadSchemaDocuments(sources, c.limits.MaxSchemaSourceBytes, c.parseLoadedSchemaDocument)
	if err != nil {
		return err
	}
	maps.Copy(c.resolvedRef, result.ReferenceAliases)
	return c.appendLoadedSchemaDocuments(result.Documents)
}

func (c *compiler) parseLoadedSchemaDocument(loaded source.LoadedSource) (source.LoadedSchemaDocument, error) {
	doc, err := parseSchemaDocument(loaded.Name, loaded.Key, loaded.Data, c.limits)
	if err != nil {
		return source.LoadedSchemaDocument{}, err
	}
	c.sourceDocs[loaded.Key] = doc
	return source.LoadedSchemaDocument{
		TargetNamespace: doc.root.attrValue(vocab.XSDAttrTargetNamespace),
		References:      schemaDocumentRefs(doc),
	}, nil
}

func (c *compiler) appendLoadedSchemaDocuments(roles []source.LoadedDocumentRole) error {
	for _, role := range roles {
		doc := c.sourceDocs[role.Key]
		if doc == nil {
			return xsderrors.InternalInvariant("loaded schema document missing parsed source")
		}
		c.graphDocs = append(c.graphDocs, doc)
		if role.Index {
			c.compileDocs = append(c.compileDocs, doc)
		}
	}
	slices.SortFunc(c.graphDocs, func(a, b *rawDoc) int {
		return cmp.Compare(a.name, b.name)
	})
	slices.SortFunc(c.compileDocs, func(a, b *rawDoc) int {
		return cmp.Compare(a.name, b.name)
	})
	return nil
}

func schemaDocumentRefs(doc *rawDoc) []source.SchemaDocumentReference {
	var elements []source.SchemaReferenceElement
	for child := range doc.root.xsdChildren() {
		switch child.Name.Local {
		case vocab.XSDElemInclude, vocab.XSDElemImport:
		default:
			continue
		}
		elements = append(elements, source.SchemaReferenceElement{
			Local:  child.Name.Local,
			Attr:   child.attr,
			Line:   child.Line,
			Column: child.Column,
		})
	}
	return source.SchemaDocumentReferences(elements)
}

func schemaLocationAttr(n *rawNode) (string, bool) {
	return source.SchemaLocationAttr(n.attr)
}

func (c *compiler) checkExplicitSchemaReferences() error {
	if err := c.propagateChameleonTargets(); err != nil {
		return err
	}
	for _, doc := range c.graphDocs {
		for child := range doc.root.xsdChildren() {
			switch child.Name.Local {
			case vocab.XSDElemInclude:
				if err := c.checkExplicitInclude(child); err != nil {
					return err
				}
			case vocab.XSDElemImport:
				if err := c.checkExplicitImport(doc, child); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *compiler) checkExplicitInclude(child *rawNode) error {
	_, ok := schemaLocationAttr(child)
	if issue := source.CheckIncludeSchemaLocation(ok); issue != source.IncludeLocationOK {
		return schemaCompileAt(child, issue.Code(), issue.Message())
	}
	return nil
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
	referencedTarget := referenced.root.attrValue(vocab.XSDAttrTargetNamespace)
	if issue := source.CheckImportedTargetNamespace(namespace, referencedTarget); issue != source.TargetNamespaceOK {
		return schemaCompileAt(child, issue.Code(), issue.Message())
	}
	return nil
}

func (c *compiler) registerExplicitImportNamespace(doc *rawDoc, child *rawNode) (string, error) {
	attr, hasNamespace := child.attr(vocab.XSDAttrNamespace)
	namespace, issue := source.CheckImportNamespace(c.documentTargetNamespace(doc), attr, hasNamespace)
	if issue != source.ImportNamespaceOK {
		return "", schemaCompileAt(child, issue.Code(), issue.Message())
	}
	if c.imports[doc.key] == nil {
		c.imports[doc.key] = make(map[string]bool)
	}
	c.imports[doc.key][namespace] = true
	return namespace, nil
}

func (c *compiler) checkReferenceNamespace(n *rawNode, ctx *schemaContext, namespace string) (string, error) {
	if ctx == nil {
		return namespace, nil
	}
	visible := source.ReferenceNamespaces{
		Imports:         ctx.imports,
		TargetNamespace: ctx.targetNS,
		AdoptedTarget:   c.adoptTarget[ctx.doc.key] != "",
	}
	namespace, issue := source.CheckReferenceNamespace(namespace, visible)
	if issue != source.ReferenceNamespaceOK {
		return "", schemaCompileAt(n, issue.Code(), issue.Message(namespace))
	}
	return namespace, nil
}

func (c *compiler) propagateChameleonTargets() error {
	for {
		plan, issue := source.PlanChameleonIncludes(c.chameleonDocuments(), c.resolvedRef, c.adoptTarget)
		if issue.Issue != source.TargetNamespaceOK {
			return schemaCompileAtPosition(issue.Line, issue.Column, issue.Issue.Code(), issue.Issue.Message())
		}
		if !plan.Changed() {
			return nil
		}
		maps.Copy(c.adoptTarget, plan.AdoptedTargets)
		maps.Copy(c.resolvedRef, plan.ReferenceAliases)
		for _, clone := range plan.Clones {
			orig := c.sourceDocs[clone.SourceKey]
			if orig == nil {
				return xsderrors.InternalInvariant("chameleon clone references missing source")
			}
			c.appendChameleonClone(orig, clone.CloneKey)
		}
	}
}

func (c *compiler) chameleonDocuments() []source.ChameleonDocument {
	docs := make([]source.ChameleonDocument, 0, len(c.graphDocs))
	for _, doc := range c.graphDocs {
		_, loaded := c.sourceDocs[doc.key]
		docs = append(docs, chameleonDocumentProjection(doc, loaded))
	}
	return docs
}

func chameleonDocumentProjection(doc *rawDoc, loaded bool) source.ChameleonDocument {
	return source.ChameleonDocument{
		Key:             doc.key,
		Name:            doc.name,
		TargetNamespace: doc.root.attrValue(vocab.XSDAttrTargetNamespace),
		References:      schemaDocumentRefs(doc),
		Loaded:          loaded,
	}
}

// appendChameleonClone registers a per-namespace copy of a chameleon schema
// document. The copy gets its own node tree because compilation memoizes per
// *rawNode. Clones are not added to sourceDocs; source owns the aliases that
// make their schemaLocation references resolve through the original document.
func (c *compiler) appendChameleonClone(orig *rawDoc, cloneKey string) {
	clone := &rawDoc{root: cloneRawTree(orig.root), name: orig.name, key: cloneKey}
	c.graphDocs = append(c.graphDocs, clone)
	c.compileDocs = append(c.compileDocs, clone)
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
	if target := doc.root.attrValue(vocab.XSDAttrTargetNamespace); target != "" {
		return target
	}
	return c.adoptTarget[doc.key]
}

func (c *compiler) resolveLoadedSchemaLocation(doc *rawDoc, location string) (*rawDoc, string, bool) {
	key, ok := source.ResolveLoadedSchemaLocation(doc.name, doc.key, location, c.resolvedRef, func(key string) bool {
		_, loaded := c.sourceDocs[key]
		return loaded
	})
	if !ok {
		return nil, "", false
	}
	return c.sourceDocs[key], key, true
}
