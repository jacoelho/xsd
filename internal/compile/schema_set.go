package compile

import (
	"cmp"
	"errors"
	"maps"
	"slices"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/uriref"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

type schemaPlan struct {
	documents []schemaSetDocument
}

type loadedSchemaGraph struct {
	byKey          map[string]loadedSchemaDocument
	dependencyWork *workBudget
	documents      []schemaSetDocument
	limits         Limits
}

type schemaSetDocument struct {
	doc               *rawDoc
	imports           map[string]bool
	effectiveTargetNS string
	contentIdentity   int
	adoptedTarget     bool
	explicitRoot      bool
	indexDeclarations bool
}

type schemaSetLoader struct {
	byKey             map[string]loadedSchemaDocument
	resolvedSources   map[string]struct{}
	pendingReferences map[string][]*schemaReference
	dependencyWork    *workBudget
	documents         []schemaSetDocument
	totalBytes        int64
	references        int
	resolvedLimit     int
	parsedNodes       int
	limits            Limits
}

type loadedSchemaDocument struct {
	doc          *rawDoc
	sources      []source.Source
	content      schemaContent
	index        int
	explicitRoot bool
}

// schemaContent identifies raw source bytes without retaining the source stream.
type schemaContent struct {
	bytes  int64
	digest [32]byte
}

type schemaSourceRequirement uint8

const (
	schemaSourceRequired schemaSourceRequirement = iota
	schemaSourceOptional
)

//nolint:govet // Field order keeps the source and its reference adjacent at queue call sites.
type schemaLoadRequest struct {
	source      source.Source
	ref         *schemaReference
	base        source.ReferenceBase
	referrer    string
	requirement schemaSourceRequirement
	resolve     bool
}

type schemaReferenceKind uint8

const (
	schemaReferenceInclude schemaReferenceKind = iota
	schemaReferenceImport
)

type schemaReference struct {
	node         *rawNode
	target       string
	location     uriref.Reference
	namespace    string
	rootBase     uriref.Reference
	localBase    uriref.Reference
	kind         schemaReferenceKind
	hasLocation  bool
	hasRootBase  bool
	hasLocalBase bool
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

func (c *schemaTargetContexts) checkAddLimits(sourceIndex int) error {
	if len(c.queue) >= c.limit {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema target contexts exceed MaxSchemaTargetContexts")
	}
	nodes := c.nodeCounts[sourceIndex]
	if nodes > c.nodeLimit-c.nodes {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema context nodes exceed MaxSchemaInstantiatedNodes")
	}
	return nil
}

func (c *schemaTargetContexts) add(sourceIndex int, target string) error {
	context := schemaTargetContext{source: sourceIndex, target: target}
	document := &c.documents[sourceIndex]
	if document.hasPrimary && document.primary == target {
		return nil
	}
	if c.hasAdditional(context) {
		return nil
	}
	if err := c.checkAddLimits(sourceIndex); err != nil {
		return err
	}
	c.promoteAdditionalSet()
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
	c.nodes += c.nodeCounts[sourceIndex]
	return nil
}

func (c *schemaTargetContexts) hasAdditional(context schemaTargetContext) bool {
	if c.additionalSet != nil {
		_, ok := c.additionalSet[context]
		return ok
	}
	return slices.Contains(c.additional, context)
}

func (c *schemaTargetContexts) promoteAdditionalSet() {
	if c.additionalSet != nil || len(c.additional) != 8 {
		return
	}
	c.additionalSet = make(map[schemaTargetContext]struct{}, len(c.additional)*2)
	for _, existing := range c.additional {
		c.additionalSet[existing] = struct{}{}
	}
}

func loadSchemaGraphOwned(
	sources []source.Source,
	limits Limits,
	dependencyWork *workBudget,
) (loadedSchemaGraph, error) {
	if dependencyWork == nil {
		return loadedSchemaGraph{}, xsderrors.InternalInvariant("schema dependency work budget is nil")
	}
	l := schemaSetLoader{
		limits:          limits,
		byKey:           make(map[string]loadedSchemaDocument),
		resolvedSources: make(map[string]struct{}),
		resolvedLimit:   limits.MaxSchemaSources - len(sources),
		dependencyWork:  dependencyWork,
	}
	if err := l.loadOwned(sources); err != nil {
		return loadedSchemaGraph{}, err
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
	return loadedSchemaGraph{
		byKey:          l.byKey,
		documents:      l.documents,
		dependencyWork: l.dependencyWork,
		limits:         limits,
	}, nil
}

func (g *loadedSchemaGraph) plan() (schemaPlan, error) {
	if err := g.validateReferenceTargets(); err != nil {
		return schemaPlan{}, err
	}
	if err := g.instantiateTargetContexts(); err != nil {
		return schemaPlan{}, err
	}
	g.selectDeclarationDocuments()
	return schemaPlan{documents: g.documents}, nil
}

func loadSchemaPlanOwned(sources []source.Source, limits Limits, dependencyWork *workBudget) (schemaPlan, error) {
	graph, err := loadSchemaGraphOwned(sources, limits, dependencyWork)
	if err != nil {
		return schemaPlan{}, err
	}
	return graph.plan()
}

func (c *compiler) loadOwned(sources []source.Source) error {
	plan, err := loadSchemaPlanOwned(sources, c.limits, &c.dependencyWork)
	if err != nil {
		return err
	}
	c.plan = plan
	return nil
}

func (l *schemaSetLoader) loadOwned(ordered []source.Source) error {
	sortSchemaSources(ordered)
	queue := make([]schemaLoadRequest, 0, len(ordered))
	for _, src := range ordered {
		queue = append(queue, schemaLoadRequest{source: src})
	}
	for len(queue) != 0 {
		item := queue[0]
		queue[0] = schemaLoadRequest{}
		queue = queue[1:]
		if err := l.processLoadRequest(item, &queue); err != nil {
			return err
		}
	}
	l.appendIdentifiedDocuments()
	return nil
}

func sortSchemaSources(ordered []source.Source) {
	slices.SortFunc(ordered, func(a, b source.Source) int {
		if nameOrder := cmp.Compare(a.Name(), b.Name()); nameOrder != 0 {
			return nameOrder
		}
		return cmp.Compare(source.Key(a.Name()), source.Key(b.Name()))
	})
}

func (l *schemaSetLoader) processLoadRequest(item schemaLoadRequest, queue *[]schemaLoadRequest) error {
	if item.resolve {
		resolved, ok, err := l.resolveReference(item)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		item = resolved
	}
	return l.read(item, queue)
}

func (l *schemaSetLoader) appendIdentifiedDocuments() {
	identifiedSources := l.identifySchemaDocumentContents()
	for _, identified := range identifiedSources {
		loaded := l.byKey[identified.doc.key]
		l.documents = append(l.documents, schemaSetDocument{
			doc:             identified.doc,
			imports:         schemaDocumentImports(identified.doc.references),
			explicitRoot:    loaded.explicitRoot,
			contentIdentity: identified.identity,
		})
	}
}

func (l *schemaSetLoader) admitResolvedSource(key string) error {
	if _, ok := l.resolvedSources[key]; ok {
		return nil
	}
	if len(l.resolvedSources) >= l.resolvedLimit {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema source count exceeds MaxSchemaSources")
	}
	l.resolvedSources[key] = struct{}{}
	return nil
}

func (l *schemaSetLoader) read(item schemaLoadRequest, queue *[]schemaLoadRequest) error {
	src := item.source
	name := src.Name()
	if name == "" {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source name is required")
	}
	key := source.Key(name)
	if loaded, ok := l.byKey[key]; ok {
		return l.readLoaded(item, key, loaded, queue)
	}
	doc, content, err := l.parseNewSource(item, key)
	if err != nil || doc == nil {
		return err
	}
	return l.registerDocument(item, key, doc, content, queue)
}

func (l *schemaSetLoader) parseNewSource(item schemaLoadRequest, key string) (*rawDoc, schemaContent, error) {
	src := item.source
	remaining := l.limits.MaxSchemaTotalBytes - l.totalBytes
	readLimit := min(l.limits.MaxSchemaSourceBytes, remaining)
	input, result := src.OpenInput(readLimit)
	var doc *rawDoc
	var parseErr error
	if input != nil {
		parseLimits := l.limits
		parseLimits.MaxSchemaInstantiatedNodes -= l.parsedNodes
		doc, parseErr = parseSchemaDocument(src.Name(), key, input, parseLimits)
		result = input.Finish()
	}
	missing, err := l.admitSourceResult(src.Name(), result, readLimit, remaining, item.requirement)
	if err != nil || missing {
		return nil, schemaContent{}, err
	}
	if parseErr != nil {
		return nil, schemaContent{}, parseErr
	}
	return doc, schemaContent{bytes: result.Bytes, digest: result.Digest}, nil
}

func (l *schemaSetLoader) admitSourceResult(name string, result source.ReadResult, readLimit, remaining int64, requirement schemaSourceRequirement) (bool, error) {
	if result.Bytes > remaining {
		return false, schemaTotalBytesLimitError(result.Err)
	}
	l.totalBytes += result.Bytes
	return classifySchemaAcquireResult(name, result, readLimit, remaining, requirement)
}

func classifySchemaAcquireResult(name string, result source.ReadResult, readLimit, remaining int64, requirement schemaSourceRequirement) (bool, error) {
	if result.Err == nil {
		return false, nil
	}
	switch requirement {
	case schemaSourceRequired:
	case schemaSourceOptional:
		if result.OpenNotFound {
			return true, nil
		}
	default:
		return false, xsderrors.InternalInvariant("unknown schema source requirement")
	}
	if result.LimitExceeded && readLimit == remaining {
		return false, schemaTotalBytesLimitError(result.Err)
	}
	if result.LimitExceeded || source.IsSchemaLimitError(result.Err) {
		return false, xsderrors.WithLocation(name, 0, 0, result.Err)
	}
	readErr := xsderrors.SchemaParse(xsderrors.CodeSchemaRead, "read schema "+name, result.Err)
	return false, xsderrors.WithLocation(name, 0, 0, readErr)
}

func (l *schemaSetLoader) registerDocument(item schemaLoadRequest, key string, doc *rawDoc, content schemaContent, queue *[]schemaLoadRequest) error {
	src := item.source
	var err error
	l.parsedNodes += doc.nodes
	if item.ref != nil {
		if targetErr := validateSchemaReferenceTarget(item.ref, doc); targetErr != nil {
			return targetErr
		}
	}
	doc.references, err = schemaDocumentReferences(doc)
	if err != nil {
		return err
	}
	if err := l.bindPendingReferences(key, doc); err != nil {
		return err
	}
	l.byKey[key] = loadedSchemaDocument{
		doc:          doc,
		sources:      []source.Source{src},
		content:      content,
		explicitRoot: item.ref == nil,
	}
	return l.enqueueReferences(src, doc.references, queue)
}

func (l *schemaSetLoader) readLoaded(
	item schemaLoadRequest,
	key string,
	loaded loadedSchemaDocument,
	queue *[]schemaLoadRequest,
) error {
	src := item.source
	sameSource := loadedSchemaContainsSource(loaded, src)
	if err := l.validateLoadedSourceBytes(item, key, loaded); err != nil {
		return err
	}
	if item.ref != nil {
		if err := validateSchemaReferenceTarget(item.ref, loaded.doc); err != nil {
			return err
		}
	}
	if item.ref == nil && !loaded.explicitRoot {
		loaded.explicitRoot = true
		l.byKey[key] = loaded
	}
	if sameSource {
		return nil
	}
	loaded.sources = append(loaded.sources, src)
	l.byKey[key] = loaded
	return l.enqueueReferences(src, loaded.doc.references, queue)
}

func loadedSchemaContainsSource(loaded loadedSchemaDocument, src source.Source) bool {
	for _, existing := range loaded.sources {
		if existing.SameResolutionContext(src) && existing.Name() == src.Name() {
			return true
		}
	}
	return false
}

func (l *schemaSetLoader) validateLoadedSourceBytes(item schemaLoadRequest, key string, loaded loadedSchemaDocument) error {
	src := item.source
	remaining := l.limits.MaxSchemaTotalBytes - l.totalBytes
	readLimit := min(l.limits.MaxSchemaSourceBytes, remaining)
	input, result := src.OpenInput(readLimit)
	if input != nil {
		result = input.Finish()
	}
	// A missing optional representation can use the already compiled document.
	// Its resolver context still participates in descendant resolution.
	useCached, err := l.admitSourceResult(src.Name(), result, readLimit, remaining, item.requirement)
	if err != nil {
		return err
	}
	if !useCached && (schemaContent{bytes: result.Bytes, digest: result.Digest}) != loaded.content {
		identityErr := xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "schema source identity resolves to different document content: "+key)
		if item.ref != nil {
			identityErr = withSchemaReferenceLocation(item, identityErr)
		} else {
			identityErr = xsderrors.WithLocation(src.Name(), 0, 0, identityErr)
		}
		return identityErr
	}
	return nil
}

func schemaTotalBytesLimitError(acquireErr error) error {
	limitErr := xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema sources exceed MaxSchemaTotalBytes")
	if acquireErr == nil || !hasNonSchemaLimitCause(acquireErr) {
		return limitErr
	}
	return errors.Join(limitErr, acquireErr)
}

func hasNonSchemaLimitCause(err error) bool {
	if err == nil {
		return false
	}
	if diagnostic, ok := err.(*xsderrors.Error); ok { //nolint:errorlint // Classify each error-tree node independently.
		if diagnostic.Code() != xsderrors.CodeSchemaLimit {
			return true
		}
		return hasNonSchemaLimitCause(diagnostic.Cause())
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return slices.ContainsFunc(joined.Unwrap(), hasNonSchemaLimitCause)
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return hasNonSchemaLimitCause(wrapped.Unwrap())
	}
	return true
}

func (l *schemaSetLoader) enqueueReferences(src source.Source, refs []schemaReference, queue *[]schemaLoadRequest) error {
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
	for i := range refs {
		ref := &refs[i]
		if !ref.hasLocation || ref.namespace == vocab.XMLNamespaceURI {
			continue
		}
		base, baseNode, err := schemaReferenceBase(src.Name(), ref)
		if err != nil {
			return schemaReferenceCompileAt(src, baseNode, "invalid xml:base: "+err.Error())
		}
		*queue = append(*queue, schemaLoadRequest{
			source: src, ref: ref, base: base, referrer: src.Name(), requirement: schemaSourceOptional, resolve: true,
		})
	}
	return nil
}

func (l *schemaSetLoader) resolveReference(request schemaLoadRequest) (schemaLoadRequest, bool, error) {
	if err := l.dependencyWork.spend(1); err != nil {
		return schemaLoadRequest{}, false, withSchemaReferenceLocation(request, err)
	}
	resolution, err := request.source.ResolveFrom(request.base, request.ref.location)
	if err != nil {
		return schemaLoadRequest{}, false, schemaResolutionError(request, err)
	}
	target := resolution.Target()
	next, found := resolution.Source()
	if !found {
		return schemaLoadRequest{}, false, l.bindUnopenedReference(request, target)
	}
	if request.ref.target != "" && request.ref.target != target {
		return schemaLoadRequest{}, false, schemaReferenceIdentityError(request)
	}
	if err := l.admitResolvedSource(target); err != nil {
		return schemaLoadRequest{}, false, withSchemaReferenceLocation(request, err)
	}
	request.ref.target = target
	return schemaLoadRequest{source: next, ref: request.ref, referrer: request.referrer, requirement: schemaSourceOptional}, true, nil
}

func schemaResolutionError(request schemaLoadRequest, err error) error {
	if source.IsReferenceResolutionError(err) {
		return schemaReferenceCompileAt(request.source, request.ref.node, "invalid schemaLocation: "+err.Error())
	}
	line, column := 0, 0
	if request.ref.node != nil {
		line, column = request.ref.node.Line, request.ref.node.Column
	}
	return xsderrors.WithLocation(request.source.Name(), line, column,
		xsderrors.SchemaParse(xsderrors.CodeSchemaRead, "resolve schema "+request.ref.location.Raw(), err))
}

func (l *schemaSetLoader) bindUnopenedReference(request schemaLoadRequest, target string) error {
	if target == "" || request.ref.target == target {
		return nil
	}
	if loaded, ok := l.byKey[target]; ok && loaded.doc != nil {
		if request.ref.target != "" {
			return schemaReferenceIdentityError(request)
		}
		if err := validateSchemaReferenceTarget(request.ref, loaded.doc); err != nil {
			return err
		}
		request.ref.target = target
		return nil
	}
	l.deferReferenceBinding(target, request.ref)
	return nil
}

func schemaReferenceIdentityError(request schemaLoadRequest) error {
	return schemaReferenceCompileAt(request.source, request.ref.node, "schema reference resolves to different document identities across resolver contexts")
}

func (l *schemaSetLoader) deferReferenceBinding(target string, ref *schemaReference) {
	if l.pendingReferences == nil {
		l.pendingReferences = make(map[string][]*schemaReference)
	}
	l.pendingReferences[target] = append(l.pendingReferences[target], ref)
}

func (l *schemaSetLoader) bindPendingReferences(target string, doc *rawDoc) error {
	pending := l.pendingReferences[target]
	for _, ref := range pending {
		if ref.target != "" && ref.target != target {
			return schemaCompileAt(ref.node, xsderrors.CodeSchemaReference, "schema reference resolves to different document identities across resolver contexts")
		}
		if err := validateSchemaReferenceTarget(ref, doc); err != nil {
			return err
		}
	}
	for _, ref := range pending {
		ref.target = target
	}
	delete(l.pendingReferences, target)
	return nil
}

func schemaDocumentReferences(doc *rawDoc) ([]schemaReference, error) {
	context, err := newSchemaDocumentReferenceContext(doc)
	if err != nil {
		return nil, err
	}
	var refs []schemaReference
	for child := range doc.root.xsdChildren() {
		ref, present, err := context.reference(child)
		if err != nil {
			return nil, err
		}
		if present {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

type schemaDocumentReferenceContext struct {
	targetNS      string
	validatedBase source.ReferenceBase
	rootBase      uriref.Reference
	hasRootBase   bool
}

func newSchemaDocumentReferenceContext(doc *rawDoc) (schemaDocumentReferenceContext, error) {
	context := schemaDocumentReferenceContext{
		validatedBase: source.NewReferenceBase(doc.name),
		targetNS:      doc.defaults.TargetNamespace,
	}
	rootBaseRaw, hasRootBase := doc.root.attrNS(vocab.XMLNamespaceURI, vocab.XMLAttrBase)
	context.hasRootBase = hasRootBase
	if !context.hasRootBase {
		return context, nil
	}
	rootBase, err := uriref.Parse(rootBaseRaw)
	if err != nil {
		return schemaDocumentReferenceContext{}, schemaCompileAt(doc.root, xsderrors.CodeSchemaReference, "invalid xml:base: "+err.Error())
	}
	context.rootBase = rootBase
	context.validatedBase, err = context.validatedBase.WithXMLBase(rootBase)
	if err != nil {
		return schemaDocumentReferenceContext{}, schemaCompileAt(doc.root, xsderrors.CodeSchemaReference, "invalid xml:base: "+err.Error())
	}
	return context, nil
}

func (c schemaDocumentReferenceContext) reference(child *rawNode) (schemaReference, bool, error) {
	kind, present := schemaReferenceKindForLocal(child.Name.Local)
	if !present {
		return schemaReference{}, false, nil
	}
	location, hasLocation, err := parseSchemaReferenceLocation(child, kind)
	if err != nil {
		return schemaReference{}, false, err
	}
	localBase, hasLocalBase, err := c.parseLocalBase(child)
	if err != nil {
		return schemaReference{}, false, err
	}
	ref := schemaReference{
		node: child, kind: kind, location: location, hasLocation: hasLocation,
		rootBase: c.rootBase, localBase: localBase, hasRootBase: c.hasRootBase, hasLocalBase: hasLocalBase,
	}
	if kind == schemaReferenceImport {
		if err := validateSchemaImportReference(child, c.targetNS, &ref); err != nil {
			return schemaReference{}, false, err
		}
	}
	return ref, true, nil
}

func schemaReferenceKindForLocal(local string) (schemaReferenceKind, bool) {
	switch local {
	case vocab.XSDElemInclude:
		return schemaReferenceInclude, true
	case vocab.XSDElemImport:
		return schemaReferenceImport, true
	default:
		return 0, false
	}
}

func parseSchemaReferenceLocation(child *rawNode, kind schemaReferenceKind) (uriref.Reference, bool, error) {
	raw, present := schemaLocationAttr(child)
	if kind == schemaReferenceInclude && !present {
		return uriref.Reference{}, false, schemaCompileAt(child, xsderrors.CodeSchemaReference, "include missing schemaLocation")
	}
	if !present {
		return uriref.Reference{}, false, nil
	}
	location, err := uriref.Parse(raw)
	if err != nil {
		return uriref.Reference{}, false, schemaCompileAt(child, xsderrors.CodeSchemaReference, "invalid schemaLocation: "+err.Error())
	}
	return location, true, nil
}

func (c schemaDocumentReferenceContext) parseLocalBase(child *rawNode) (uriref.Reference, bool, error) {
	raw, present := child.attrNS(vocab.XMLNamespaceURI, vocab.XMLAttrBase)
	if !present {
		return uriref.Reference{}, false, nil
	}
	localBase, err := uriref.Parse(raw)
	if err != nil {
		return uriref.Reference{}, false, schemaCompileAt(child, xsderrors.CodeSchemaReference, "invalid xml:base: "+err.Error())
	}
	if _, err := c.validatedBase.WithXMLBase(localBase); err != nil {
		return uriref.Reference{}, false, schemaCompileAt(child, xsderrors.CodeSchemaReference, "invalid xml:base: "+err.Error())
	}
	return localBase, true, nil
}

func validateSchemaImportReference(child *rawNode, target string, ref *schemaReference) error {
	namespace, hasNamespace := child.attr(vocab.XSDAttrNamespace)
	ref.namespace = namespace
	switch {
	case hasNamespace && namespace == "":
		return schemaCompileAt(child, xsderrors.CodeSchemaInvalidAttribute, "import namespace cannot be empty")
	case !hasNamespace && target == "":
		return schemaCompileAt(child, xsderrors.CodeSchemaReference, "import without namespace requires enclosing schema targetNamespace")
	case hasNamespace && namespace == target:
		return schemaCompileAt(child, xsderrors.CodeSchemaReference, "import namespace cannot match enclosing schema targetNamespace")
	default:
		return nil
	}
}

func schemaDocumentImports(refs []schemaReference) map[string]bool {
	var imports map[string]bool
	for _, ref := range refs {
		if ref.kind != schemaReferenceImport {
			continue
		}
		if imports == nil {
			imports = make(map[string]bool)
		}
		imports[ref.namespace] = true
	}
	return imports
}

func schemaReferenceBase(name string, ref *schemaReference) (source.ReferenceBase, *rawNode, error) {
	base := source.NewReferenceBase(name)
	if ref.hasRootBase {
		resolved, err := base.WithXMLBase(ref.rootBase)
		if err != nil {
			return source.ReferenceBase{}, ref.node.doc.root, err
		}
		base = resolved
	}
	if ref.hasLocalBase {
		resolved, err := base.WithXMLBase(ref.localBase)
		if err != nil {
			return source.ReferenceBase{}, ref.node, err
		}
		base = resolved
	}
	return base, nil, nil
}

func schemaReferenceCompileAt(src source.Source, node *rawNode, message string) error {
	line, column := 0, 0
	if node != nil {
		line, column = node.Line, node.Column
	}
	return xsderrors.WithLocation(src.Name(), line, column, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, message))
}

func withSchemaReferenceLocation(request schemaLoadRequest, err error) error {
	if request.ref == nil || request.ref.node == nil || err == nil {
		return err
	}
	path := request.referrer
	if path == "" {
		path = request.source.Name()
	}
	return xsderrors.WithLocation(path, request.ref.node.Line, request.ref.node.Column, err)
}

func schemaLocationAttr(n *rawNode) (string, bool) {
	return n.attr(vocab.XSDAttrSchemaLocation)
}

func (g *loadedSchemaGraph) validateReferenceTargets() error {
	for i := range g.documents {
		if err := g.validateDocumentReferenceTargets(g.documents[i].doc); err != nil {
			return err
		}
	}
	return nil
}

func (g *loadedSchemaGraph) validateDocumentReferenceTargets(doc *rawDoc) error {
	for i := range doc.references {
		ref := &doc.references[i]
		if ref.target == "" {
			continue
		}
		loaded, ok := g.byKey[ref.target]
		if !ok || loaded.doc == nil {
			continue
		}
		if err := validateSchemaReferenceTarget(ref, loaded.doc); err != nil {
			return err
		}
	}
	return nil
}

func validateSchemaReferenceTarget(ref *schemaReference, target *rawDoc) error {
	if ref == nil || target == nil {
		return xsderrors.InternalInvariant("schema reference target validation requires an edge and target")
	}
	referencedTarget := target.defaults.TargetNamespace
	switch ref.kind {
	case schemaReferenceInclude:
		declaredTarget := ref.node.doc.defaults.TargetNamespace
		if referencedTarget != "" && referencedTarget != declaredTarget {
			return schemaCompileAt(ref.node, xsderrors.CodeSchemaReference, "included schema targetNamespace does not match including schema")
		}
	case schemaReferenceImport:
		if referencedTarget != ref.namespace {
			return schemaCompileAt(ref.node, xsderrors.CodeSchemaReference, "import namespace does not match imported schema targetNamespace")
		}
	default:
		return xsderrors.InternalInvariant("schema reference target validation received invalid edge kind")
	}
	return nil
}

func (g *loadedSchemaGraph) instantiateTargetContexts() error {
	references, contexts := g.schemaTargetContextInputs()
	if contexts.documents == nil {
		return g.applyDeclaredTargetContexts()
	}
	if err := g.seedTargetContexts(&contexts); err != nil {
		return err
	}
	if err := g.propagateTargetContexts(&contexts, references); err != nil {
		return err
	}
	if err := g.seedUnassignedTargetContexts(&contexts); err != nil {
		return err
	}
	if err := g.propagateTargetContexts(&contexts, references); err != nil {
		return err
	}

	return g.applyTargetContexts(contexts)
}

func (g *loadedSchemaGraph) applyDeclaredTargetContexts() error {
	if len(g.documents) > g.limits.MaxSchemaTargetContexts {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema target contexts exceed MaxSchemaTargetContexts")
	}
	for i := range g.documents {
		g.documents[i].effectiveTargetNS = g.documents[i].doc.defaults.TargetNamespace
	}
	return nil
}

func (g *loadedSchemaGraph) seedTargetContexts(contexts *schemaTargetContexts) error {
	for i, document := range g.documents {
		target := document.doc.defaults.TargetNamespace
		if target == "" && !contexts.documents[i].importTarget && !document.explicitRoot {
			continue
		}
		if err := contexts.add(i, target); err != nil {
			return err
		}
	}
	return nil
}

func (g *loadedSchemaGraph) seedUnassignedTargetContexts(contexts *schemaTargetContexts) error {
	for i := range g.documents {
		if contexts.documents[i].hasPrimary {
			continue
		}
		if err := contexts.add(i, ""); err != nil {
			return err
		}
	}
	return nil
}

func (g *loadedSchemaGraph) schemaTargetContextInputs() ([]resolvedSchemaReference, schemaTargetContexts) {
	builder := schemaTargetContextInputBuilder{graph: g}
	for i, document := range g.documents {
		builder.addDocument(i, document.doc)
	}
	return builder.references, builder.contexts
}

type schemaTargetContextInputBuilder struct {
	graph      *loadedSchemaGraph
	references []resolvedSchemaReference
	contexts   schemaTargetContexts
}

func (b *schemaTargetContextInputBuilder) addDocument(index int, doc *rawDoc) {
	start := len(b.references)
	for _, ref := range doc.references {
		b.addReference(ref)
	}
	if b.contexts.documents != nil {
		b.contexts.documents[index].references = resolvedSchemaReferenceSpan{start: start, count: len(b.references) - start}
	}
}

func (b *schemaTargetContextInputBuilder) addReference(ref schemaReference) {
	if !ref.hasLocation {
		return
	}
	loaded := b.graph.byKey[ref.target]
	if loaded.doc == nil {
		return
	}
	if b.contexts.documents == nil {
		b.contexts = newSchemaTargetContexts(b.graph.documents, b.graph.limits.MaxSchemaTargetContexts, b.graph.limits.MaxSchemaInstantiatedNodes)
	}
	target := loaded.index
	b.references = append(b.references, resolvedSchemaReference{
		location: ref.location.Raw(), target: target, kind: ref.kind,
	})
	if ref.kind == schemaReferenceImport {
		b.contexts.documents[target].importTarget = true
	}
}

func (g *loadedSchemaGraph) propagateTargetContexts(contexts *schemaTargetContexts, references []resolvedSchemaReference) error {
	for contexts.next < len(contexts.queue) {
		context := contexts.queue[contexts.next]
		contexts.next++
		if err := g.propagateTargetContext(contexts, references, context); err != nil {
			return err
		}
	}
	return nil
}

func (g *loadedSchemaGraph) propagateTargetContext(contexts *schemaTargetContexts, references []resolvedSchemaReference, context schemaTargetContext) error {
	span := contexts.documents[context.source].references
	for _, ref := range references[span.start : span.start+span.count] {
		if err := g.dependencyWork.spend(1); err != nil {
			return err
		}
		if ref.kind != schemaReferenceInclude {
			continue
		}
		if g.documents[ref.target].doc.defaults.TargetNamespace != "" {
			continue
		}
		if err := contexts.add(ref.target, context.target); err != nil {
			return err
		}
	}
	return nil
}

func (g *loadedSchemaGraph) applyTargetContexts(contexts schemaTargetContexts) error {
	baseCount := len(g.documents)
	var clones []schemaSetDocument
	for i := range baseCount {
		document := &g.documents[i]
		state := contexts.documents[i]
		declaredTarget := document.doc.defaults.TargetNamespace
		document.effectiveTargetNS = state.primary
		document.adoptedTarget = declaredTarget == "" && state.primary != ""
	}
	for _, context := range contexts.additional {
		document := &g.documents[context.source]
		declaredTarget := document.doc.defaults.TargetNamespace
		cloneKey := document.doc.key + "\x00" + context.target
		clone := cloneRawDocument(document.doc, cloneKey)
		clones = append(clones, schemaSetDocument{
			doc:               clone,
			imports:           document.imports,
			effectiveTargetNS: context.target,
			contentIdentity:   document.contentIdentity,
			adoptedTarget:     declaredTarget == "" && context.target != "",
		})
	}
	g.documents = append(g.documents, clones...)
	return nil
}

func cloneRawDocument(doc *rawDoc, key string) *rawDoc {
	nodes := make(map[*rawNode]*rawNode)
	clone := &rawDoc{name: doc.name, key: key, defaults: doc.defaults, nodes: doc.nodes}
	clone.root = cloneRawTree(doc.root, nodes, clone)
	clone.references = make([]schemaReference, len(doc.references))
	copy(clone.references, doc.references)
	for i := range clone.references {
		clone.references[i].node = nodes[clone.references[i].node]
	}
	return clone
}

func cloneRawTree(n *rawNode, nodes map[*rawNode]*rawNode, doc *rawDoc) *rawNode {
	copied := *n
	copied.doc = doc
	nodes[n] = &copied
	if len(n.Children) > 0 {
		copied.Children = make([]*rawNode, len(n.Children))
		for i, child := range n.Children {
			copied.Children[i] = cloneRawTree(child, nodes, doc)
		}
	}
	return &copied
}

type identifiedSchemaDocument struct {
	doc      *rawDoc
	identity int
}

func (l *schemaSetLoader) identifySchemaDocumentContents() []identifiedSchemaDocument {
	ordered := slices.Sorted(maps.Keys(l.byKey))
	seen := make(map[schemaContent]int, len(ordered))
	identified := make([]identifiedSchemaDocument, 0, len(ordered))
	nextIdentity := 1
	for _, sourceKey := range ordered {
		src := l.byKey[sourceKey]
		identity := seen[src.content]
		if identity == 0 {
			identity = nextIdentity
			nextIdentity++
			seen[src.content] = identity
		}
		identified = append(identified, identifiedSchemaDocument{doc: src.doc, identity: identity})
	}
	return identified
}

func (g *loadedSchemaGraph) selectDeclarationDocuments() {
	type declarationKey struct {
		target  string
		content int
	}
	seen := make(map[declarationKey]struct{}, len(g.documents))
	for i := range g.documents {
		document := &g.documents[i]
		key := declarationKey{target: document.effectiveTargetNS, content: document.contentIdentity}
		_, duplicate := seen[key]
		document.indexDeclarations = !duplicate
		seen[key] = struct{}{}
	}
}

func checkReferenceNamespace(n *rawNode, ctx *schemaContext, namespace string) (string, error) {
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
