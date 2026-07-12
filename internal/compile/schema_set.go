package compile

import (
	"bytes"
	"cmp"
	"errors"
	"hash/maphash"
	"os"
	"slices"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

type schemaSet struct {
	documents []schemaSetDocument
}

type schemaSetDocument struct {
	doc               *rawDoc
	imports           map[string]bool
	effectiveTargetNS string
	adoptedTarget     bool
	indexDeclarations bool
}

type schemaSetLoader struct {
	byKey        map[string]loadedSchemaDocument
	aliases      map[schemaReferenceKey]string
	admitted     map[string]struct{}
	documents    []schemaSetDocument
	loadedSource []loadedSchemaSource
	totalBytes   int64
	references   int
	parsedNodes  int
	limits       Limits
}

type loadedSchemaDocument struct {
	doc   *rawDoc
	index int
}

type loadedSchemaSource struct {
	doc  *rawDoc
	data []byte
}

type schemaLoadRequest struct {
	source   source.Source
	baseKey  string
	ref      schemaReference
	optional bool
	resolve  bool
}

type schemaReferenceKey struct {
	base     string
	location string
}

type schemaReferenceKind uint8

const (
	schemaReferenceInclude schemaReferenceKind = iota
	schemaReferenceImport
)

type schemaReference struct {
	location  string
	namespace string
	kind      schemaReferenceKind
}

type resolvedSchemaReference struct {
	location string
	target   int
	kind     schemaReferenceKind
}

type resolvedSchemaReferenceSpan struct {
	start int
	count int
}

type schemaTargetContext struct {
	target string
	source int
}

type schemaTargetDocumentState struct {
	primary      string
	references   resolvedSchemaReferenceSpan
	importTarget bool
	hasPrimary   bool
}

type schemaTargetContexts struct {
	documents     []schemaTargetDocumentState
	additional    []schemaTargetContext
	additionalSet map[schemaTargetContext]struct{}
	queue         []schemaTargetContext
	nodeCounts    []int
	next          int
	limit         int
	nodes         int
	nodeLimit     int
}

func newSchemaTargetContexts(documents []schemaSetDocument, limit, nodeLimit int) schemaTargetContexts {
	nodeCounts := make([]int, len(documents))
	for i := range documents {
		nodeCounts[i] = documents[i].doc.nodes
	}
	return schemaTargetContexts{
		documents:  make([]schemaTargetDocumentState, len(documents)),
		queue:      make([]schemaTargetContext, 0, len(documents)),
		limit:      limit,
		nodeCounts: nodeCounts,
		nodeLimit:  nodeLimit,
	}
}

func (c *schemaTargetContexts) checkAddLimits(source int) error {
	if len(c.queue) >= c.limit {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema target contexts exceed MaxSchemaTargetContexts")
	}
	nodes := c.nodeCounts[source]
	if nodes > c.nodeLimit-c.nodes {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema context nodes exceed MaxSchemaInstantiatedNodes")
	}
	return nil
}

func (c *schemaTargetContexts) add(source int, target string) error {
	context := schemaTargetContext{source: source, target: target}
	document := &c.documents[source]
	if document.hasPrimary && document.primary == target {
		return nil
	}
	if c.additionalSet == nil {
		if slices.Contains(c.additional, context) {
			return nil
		}
		if err := c.checkAddLimits(source); err != nil {
			return err
		}
		if len(c.additional) == 8 {
			c.additionalSet = make(map[schemaTargetContext]struct{}, len(c.additional)*2)
			for _, existing := range c.additional {
				c.additionalSet[existing] = struct{}{}
			}
		}
	} else {
		if _, ok := c.additionalSet[context]; ok {
			return nil
		}
		if err := c.checkAddLimits(source); err != nil {
			return err
		}
	}
	if !document.hasPrimary {
		document.hasPrimary = true
		document.primary = target
	} else {
		c.additional = append(c.additional, context)
		if c.additionalSet != nil {
			c.additionalSet[context] = struct{}{}
		}
	}
	c.queue = append(c.queue, context)
	c.nodes += c.nodeCounts[source]
	return nil
}

func loadSchemaSet(sources []source.Source, limits Limits) (schemaSet, error) {
	l := schemaSetLoader{
		limits:   limits,
		byKey:    make(map[string]loadedSchemaDocument),
		aliases:  make(map[schemaReferenceKey]string),
		admitted: make(map[string]struct{}),
	}
	if err := l.load(sources); err != nil {
		return schemaSet{}, err
	}
	slices.SortFunc(l.documents, func(a, b schemaSetDocument) int {
		return cmp.Compare(a.doc.name, b.doc.name)
	})
	for i := range l.documents {
		key := l.documents[i].doc.key
		loaded := l.byKey[key]
		loaded.index = i
		l.byKey[key] = loaded
	}
	if err := l.instantiateTargetContexts(); err != nil {
		return schemaSet{}, err
	}
	if err := l.checkIncludeTargets(); err != nil {
		return schemaSet{}, err
	}
	if err := l.checkExplicitReferences(); err != nil {
		return schemaSet{}, err
	}
	return schemaSet{documents: l.documents}, nil
}

func (l *schemaSetLoader) checkIncludeTargets() error {
	for i := range l.documents {
		document := &l.documents[i]
		for child := range document.doc.root.xsdChildren() {
			if child.Name.Local != vocab.XSDElemInclude {
				continue
			}
			if err := l.checkInclude(document, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *compiler) load(sources []source.Source) error {
	set, err := loadSchemaSet(sources, c.limits)
	if err != nil {
		return err
	}
	c.schemas = set
	return nil
}

func (l *schemaSetLoader) load(sources []source.Source) error {
	queue := make([]schemaLoadRequest, 0, len(sources))
	for _, src := range sources {
		if err := l.enqueueSource(&queue, schemaLoadRequest{source: src}); err != nil {
			return err
		}
	}
	for len(queue) != 0 {
		item := queue[0]
		queue[0] = schemaLoadRequest{}
		queue = queue[1:]
		if item.resolve {
			var ok bool
			var err error
			item, ok, err = l.resolveReference(item)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if err := l.admitSource(item.source); err != nil {
				return err
			}
		}
		loadedSource, ok, err := l.read(item, &queue)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		l.loadedSource = append(l.loadedSource, loadedSource)
	}
	for _, role := range planSchemaDocumentRoles(l.loadedSource) {
		l.documents = append(l.documents, schemaSetDocument{
			doc:               role.source.doc,
			indexDeclarations: role.index,
		})
	}
	return nil
}

func (l *schemaSetLoader) enqueueSource(queue *[]schemaLoadRequest, request schemaLoadRequest) error {
	if err := l.admitSource(request.source); err != nil {
		return err
	}
	*queue = append(*queue, request)
	return nil
}

func (l *schemaSetLoader) admitSource(src source.Source) error {
	name := src.Name()
	if name == "" {
		return nil
	}
	key := source.Key(name)
	if _, ok := l.byKey[key]; ok {
		return nil
	}
	if _, ok := l.admitted[key]; !ok {
		if len(l.admitted) >= l.limits.MaxSchemaSources {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema source count exceeds MaxSchemaSources")
		}
		l.admitted[key] = struct{}{}
	}
	return nil
}

func (l *schemaSetLoader) read(item schemaLoadRequest, queue *[]schemaLoadRequest) (loadedSchemaSource, bool, error) {
	src := item.source
	name := src.Name()
	if name == "" {
		return loadedSchemaSource{}, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source name is required")
	}
	key := source.Key(name)
	if _, ok := l.byKey[key]; ok {
		return loadedSchemaSource{}, false, nil
	}
	remaining := l.limits.MaxSchemaTotalBytes - l.totalBytes
	readLimit := min(l.limits.MaxSchemaSourceBytes, remaining)
	data, limitExceeded, err := src.ReadWithLimit(readLimit)
	dataBytes := int64(len(data))
	if dataBytes > remaining {
		return loadedSchemaSource{}, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema sources exceed MaxSchemaTotalBytes")
	}
	l.totalBytes += dataBytes
	if err != nil {
		if item.optional && (errors.Is(err, xsderrors.ErrSchemaNotFound) || errors.Is(err, os.ErrNotExist)) {
			return loadedSchemaSource{}, false, nil
		}
		if limitExceeded && readLimit == remaining {
			return loadedSchemaSource{}, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema sources exceed MaxSchemaTotalBytes")
		}
		if limitExceeded || source.IsSchemaLimitError(err) {
			return loadedSchemaSource{}, false, err
		}
		return loadedSchemaSource{}, false, xsderrors.SchemaParse(xsderrors.CodeSchemaRead, 0, 0, "read schema "+name, err)
	}
	parseLimits := l.limits
	parseLimits.MaxSchemaInstantiatedNodes -= l.parsedNodes
	doc, err := parseSchemaDocument(name, key, data, parseLimits)
	if err != nil {
		return loadedSchemaSource{}, false, err
	}
	l.parsedNodes += doc.nodes
	l.byKey[key] = loadedSchemaDocument{doc: doc}
	if err := l.enqueueReferences(src, key, schemaDocumentReferences(doc), queue); err != nil {
		return loadedSchemaSource{}, false, err
	}
	return loadedSchemaSource{doc: doc, data: data}, true, nil
}

func (l *schemaSetLoader) enqueueReferences(src source.Source, baseKey string, refs []schemaReference, queue *[]schemaLoadRequest) error {
	referenceCount := 0
	for _, ref := range refs {
		if ref.namespace != vocab.XMLNamespaceURI {
			referenceCount++
		}
	}
	if referenceCount > l.limits.MaxSchemaReferences-l.references {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema references exceed MaxSchemaReferences")
	}
	l.references += referenceCount
	for _, ref := range refs {
		if ref.namespace == vocab.XMLNamespaceURI {
			continue
		}
		*queue = append(*queue, schemaLoadRequest{source: src, baseKey: baseKey, ref: ref, optional: true, resolve: true})
	}
	return nil
}

func (l *schemaSetLoader) resolveReference(request schemaLoadRequest) (schemaLoadRequest, bool, error) {
	alias := schemaReferenceKey{base: request.baseKey, location: request.ref.location}
	if key, ok := l.aliases[alias]; ok {
		if _, loaded := l.byKey[key]; loaded {
			return schemaLoadRequest{}, false, nil
		}
	}
	next, found, err := request.source.Resolve(request.ref.location)
	if err != nil {
		return schemaLoadRequest{}, false, xsderrors.SchemaParse(xsderrors.CodeSchemaRead, 0, 0, "resolve schema "+request.ref.location, err)
	}
	if !found {
		return schemaLoadRequest{}, false, nil
	}
	if next.Name() != "" {
		l.aliases[alias] = source.Key(next.Name())
	}
	return schemaLoadRequest{source: next, optional: true}, true, nil
}

func schemaDocumentReferences(doc *rawDoc) []schemaReference {
	var refs []schemaReference
	for child := range doc.root.xsdChildren() {
		var kind schemaReferenceKind
		switch child.Name.Local {
		case vocab.XSDElemInclude:
			kind = schemaReferenceInclude
		case vocab.XSDElemImport:
			kind = schemaReferenceImport
		default:
			continue
		}
		location, ok := schemaLocationAttr(child)
		if !ok {
			continue
		}
		ref := schemaReference{kind: kind, location: location}
		if kind == schemaReferenceImport {
			ref.namespace, _ = child.attr(vocab.XSDAttrNamespace)
		}
		refs = append(refs, ref)
	}
	return refs
}

func schemaLocationAttr(n *rawNode) (string, bool) {
	location, ok := n.attr(vocab.XSDAttrSchemaLocation)
	if !ok {
		return "", false
	}
	return source.NormalizeSchemaLocation(location)
}

func (l *schemaSetLoader) checkExplicitReferences() error {
	for i := range l.documents {
		document := &l.documents[i]
		doc := document.doc
		for child := range doc.root.xsdChildren() {
			switch child.Name.Local {
			case vocab.XSDElemInclude:
				if _, ok := schemaLocationAttr(child); !ok {
					return schemaCompileAt(child, xsderrors.CodeSchemaReference, "include missing schemaLocation")
				}
			case vocab.XSDElemImport:
				if err := l.checkImport(document, child); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (l *schemaSetLoader) checkInclude(document *schemaSetDocument, child *rawNode) error {
	location, ok := schemaLocationAttr(child)
	if !ok {
		return nil
	}
	referenced, ok := l.resolveLoaded(document.doc, location)
	if !ok {
		return nil
	}
	referencedTarget := referenced.root.attrValue(vocab.XSDAttrTargetNamespace)
	if referencedTarget != "" && referencedTarget != document.effectiveTargetNS {
		return schemaCompileAt(child, xsderrors.CodeSchemaReference, "included schema targetNamespace does not match including schema")
	}
	return nil
}

func (l *schemaSetLoader) checkImport(document *schemaSetDocument, child *rawNode) error {
	doc := document.doc
	target := document.effectiveTargetNS
	namespace, hasNamespace := child.attr(vocab.XSDAttrNamespace)
	switch {
	case hasNamespace && namespace == "":
		return schemaCompileAt(child, xsderrors.CodeSchemaInvalidAttribute, "import namespace cannot be empty")
	case !hasNamespace && target == "":
		return schemaCompileAt(child, xsderrors.CodeSchemaReference, "import without namespace requires enclosing schema targetNamespace")
	case hasNamespace && namespace == target:
		return schemaCompileAt(child, xsderrors.CodeSchemaReference, "import namespace cannot match enclosing schema targetNamespace")
	}
	if document.imports == nil {
		document.imports = make(map[string]bool)
	}
	document.imports[namespace] = true
	location, ok := schemaLocationAttr(child)
	if !ok {
		return nil
	}
	referenced, ok := l.resolveLoaded(doc, location)
	if !ok {
		return nil
	}
	if referencedTarget := referenced.root.attrValue(vocab.XSDAttrTargetNamespace); referencedTarget != namespace {
		return schemaCompileAt(child, xsderrors.CodeSchemaReference, "import namespace does not match imported schema targetNamespace")
	}
	return nil
}

func (l *schemaSetLoader) instantiateTargetContexts() error {
	references, contexts := l.schemaTargetContextInputs()
	if contexts.documents == nil {
		if len(l.documents) > l.limits.MaxSchemaTargetContexts {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema target contexts exceed MaxSchemaTargetContexts")
		}
		for i := range l.documents {
			l.documents[i].effectiveTargetNS = l.documents[i].doc.root.attrValue(vocab.XSDAttrTargetNamespace)
		}
		return nil
	}

	for i, document := range l.documents {
		target := document.doc.root.attrValue(vocab.XSDAttrTargetNamespace)
		if target != "" || contexts.documents[i].importTarget {
			if err := contexts.add(i, target); err != nil {
				return err
			}
		}
	}
	if err := l.propagateTargetContexts(&contexts, references); err != nil {
		return err
	}
	for i := range l.documents {
		if contexts.documents[i].hasPrimary {
			continue
		}
		if err := contexts.add(i, ""); err != nil {
			return err
		}
	}
	if err := l.propagateTargetContexts(&contexts, references); err != nil {
		return err
	}

	l.applyTargetContexts(contexts, references)
	return nil
}

func (l *schemaSetLoader) schemaTargetContextInputs() ([]resolvedSchemaReference, schemaTargetContexts) {
	var references []resolvedSchemaReference
	var contexts schemaTargetContexts
	for i, document := range l.documents {
		start := len(references)
		for _, ref := range schemaDocumentReferences(document.doc) {
			targetKey, ok := l.resolveLoadedKey(document.doc, ref.location)
			if !ok {
				continue
			}
			if contexts.documents == nil {
				contexts = newSchemaTargetContexts(l.documents, l.limits.MaxSchemaTargetContexts, l.limits.MaxSchemaInstantiatedNodes)
			}
			target := l.byKey[targetKey].index
			references = append(references, resolvedSchemaReference{
				location: ref.location,
				target:   target,
				kind:     ref.kind,
			})
			if ref.kind == schemaReferenceImport {
				contexts.documents[target].importTarget = true
			}
		}
		if contexts.documents != nil {
			contexts.documents[i].references = resolvedSchemaReferenceSpan{start: start, count: len(references) - start}
		}
	}
	return references, contexts
}

func (l *schemaSetLoader) propagateTargetContexts(contexts *schemaTargetContexts, references []resolvedSchemaReference) error {
	for contexts.next < len(contexts.queue) {
		context := contexts.queue[contexts.next]
		contexts.next++
		span := contexts.documents[context.source].references
		for _, ref := range references[span.start : span.start+span.count] {
			if ref.kind != schemaReferenceInclude {
				continue
			}
			referenced := l.documents[ref.target].doc
			if referenced.root.attrValue(vocab.XSDAttrTargetNamespace) != "" {
				continue
			}
			if err := contexts.add(ref.target, context.target); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *schemaSetLoader) applyTargetContexts(contexts schemaTargetContexts, references []resolvedSchemaReference) {
	baseCount := len(l.documents)
	var clones []schemaSetDocument
	for i := range baseCount {
		document := &l.documents[i]
		state := contexts.documents[i]
		declaredTarget := document.doc.root.attrValue(vocab.XSDAttrTargetNamespace)
		document.effectiveTargetNS = state.primary
		document.adoptedTarget = declaredTarget == "" && state.primary != ""
	}
	for _, context := range contexts.additional {
		document := &l.documents[context.source]
		declaredTarget := document.doc.root.attrValue(vocab.XSDAttrTargetNamespace)
		cloneKey := document.doc.key + "\x00" + context.target
		clone := &rawDoc{root: cloneRawTree(document.doc.root), name: document.doc.name, key: cloneKey, nodes: document.doc.nodes}
		clones = append(clones, schemaSetDocument{
			doc:               clone,
			effectiveTargetNS: context.target,
			adoptedTarget:     declaredTarget == "" && context.target != "",
			indexDeclarations: true,
		})
		span := contexts.documents[context.source].references
		for _, ref := range references[span.start : span.start+span.count] {
			targetKey := l.documents[ref.target].doc.key
			l.aliases[schemaReferenceKey{base: cloneKey, location: ref.location}] = targetKey
		}
	}
	l.documents = append(l.documents, clones...)
}

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

func (l *schemaSetLoader) resolveLoaded(doc *rawDoc, location string) (*rawDoc, bool) {
	key, ok := l.resolveLoadedKey(doc, location)
	if !ok {
		return nil, false
	}
	return l.byKey[key].doc, true
}

func (l *schemaSetLoader) resolveLoadedKey(doc *rawDoc, location string) (string, bool) {
	location, ok := source.NormalizeSchemaLocation(location)
	if !ok {
		return "", false
	}
	if key, ok := l.aliases[schemaReferenceKey{base: doc.key, location: location}]; ok && l.byKey[key].doc != nil {
		return key, true
	}
	for _, key := range source.LocationKeys(doc.name, doc.key, location) {
		if l.byKey[key].doc != nil {
			return key, true
		}
	}
	return "", false
}

type schemaDocumentRole struct {
	source loadedSchemaSource
	index  bool
}

func planSchemaDocumentRoles(sources []loadedSchemaSource) []schemaDocumentRole {
	ordered := slices.Clone(sources)
	slices.SortFunc(ordered, func(a, b loadedSchemaSource) int { return cmp.Compare(a.doc.key, b.doc.key) })
	counts := make(map[string]int)
	for _, src := range ordered {
		if target := src.doc.root.attrValue(vocab.XSDAttrTargetNamespace); target != "" {
			counts[target]++
		}
	}
	seed := maphash.MakeSeed()
	type contentKey struct {
		target string
		size   int
		hash   uint64
	}
	seen := make(map[contentKey][][]byte)
	roles := make([]schemaDocumentRole, 0, len(ordered))
	for _, src := range ordered {
		role := schemaDocumentRole{source: src, index: true}
		target := src.doc.root.attrValue(vocab.XSDAttrTargetNamespace)
		if target != "" && counts[target] > 1 {
			key := contentKey{target: target, size: len(src.data), hash: maphash.Bytes(seed, src.data)}
			for _, data := range seen[key] {
				if bytes.Equal(data, src.data) {
					role.index = false
					break
				}
			}
			if role.index {
				seen[key] = append(seen[key], src.data)
			}
		}
		roles = append(roles, role)
	}
	return roles
}

func (c *compiler) checkReferenceNamespace(n *rawNode, ctx *schemaContext, namespace string) (string, error) {
	if ctx == nil {
		return namespace, nil
	}
	if namespace == "" && ctx.targetNS != "" && ctx.adoptedTarget {
		namespace = ctx.targetNS
	}
	if namespace == vocab.XSDNamespaceURI || namespace == vocab.XMLNamespaceURI || namespace == ctx.targetNS || ctx.imports[namespace] {
		return namespace, nil
	}
	return "", schemaCompileAt(n, xsderrors.CodeSchemaReference, "namespace is not imported: "+namespace)
}
