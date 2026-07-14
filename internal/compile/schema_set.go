package compile

import (
	"bytes"
	"cmp"
	"errors"
	"hash/maphash"
	"slices"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/uriref"
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
	contentIdentity   int
	adoptedTarget     bool
	explicitRoot      bool
	indexDeclarations bool
}

type schemaSetLoader struct {
	byKey             map[string]loadedSchemaDocument
	resolvedSources   map[string]struct{}
	pendingReferences map[string][]*schemaReference
	documents         []schemaSetDocument
	loadedSource      []loadedSchemaSource
	totalBytes        int64
	references        int
	resolvedLimit     int
	parsedNodes       int
	limits            Limits
}

type loadedSchemaDocument struct {
	doc          *rawDoc
	sources      []source.Source
	data         []byte
	index        int
	explicitRoot bool
}

type loadedSchemaSource struct {
	doc  *rawDoc
	data []byte
}

//nolint:govet // Field order keeps the source and its reference adjacent at queue call sites.
type schemaLoadRequest struct {
	source   source.Source
	ref      *schemaReference
	base     source.ReferenceBase
	referrer string
	optional bool
	resolve  bool
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

func loadSchemaSetOwned(sources []source.Source, limits Limits) (schemaSet, error) {
	l := schemaSetLoader{
		limits:          limits,
		byKey:           make(map[string]loadedSchemaDocument),
		resolvedSources: make(map[string]struct{}),
		resolvedLimit:   limits.MaxSchemaSources - len(sources),
	}
	if err := l.loadOwned(sources); err != nil {
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
	if err := l.validateLoadedReferenceTargets(); err != nil {
		return schemaSet{}, err
	}
	if err := l.instantiateTargetContexts(); err != nil {
		return schemaSet{}, err
	}
	l.selectDeclarationDocuments()
	return schemaSet{documents: l.documents}, nil
}

func (c *compiler) load(sources []source.Source) error {
	return c.loadOwned(slices.Clone(sources))
}

func (c *compiler) loadOwned(sources []source.Source) error {
	set, err := loadSchemaSetOwned(sources, c.limits)
	if err != nil {
		return err
	}
	c.schemas = set
	return nil
}

func (l *schemaSetLoader) loadOwned(ordered []source.Source) error {
	slices.SortFunc(ordered, func(a, b source.Source) int {
		if nameOrder := cmp.Compare(a.Name(), b.Name()); nameOrder != 0 {
			return nameOrder
		}
		return cmp.Compare(source.Key(a.Name()), source.Key(b.Name()))
	})
	queue := make([]schemaLoadRequest, 0, len(ordered))
	for _, src := range ordered {
		queue = append(queue, schemaLoadRequest{source: src})
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
	for _, identified := range identifySchemaDocumentContents(l.loadedSource) {
		loaded := l.byKey[identified.source.doc.key]
		l.documents = append(l.documents, schemaSetDocument{
			doc:             identified.source.doc,
			imports:         schemaDocumentImports(identified.source.doc.references),
			explicitRoot:    loaded.explicitRoot,
			contentIdentity: identified.identity,
		})
	}
	return nil
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

func (l *schemaSetLoader) read(item schemaLoadRequest, queue *[]schemaLoadRequest) (loadedSchemaSource, bool, error) {
	src := item.source
	name := src.Name()
	if name == "" {
		return loadedSchemaSource{}, false, xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source name is required")
	}
	key := source.Key(name)
	if loaded, ok := l.byKey[key]; ok {
		return l.readLoaded(item, key, loaded, queue)
	}
	remaining := l.limits.MaxSchemaTotalBytes - l.totalBytes
	readLimit := min(l.limits.MaxSchemaSourceBytes, remaining)
	result := src.Acquire(readLimit)
	data := result.Data
	dataBytes := int64(len(data))
	if dataBytes > remaining {
		return loadedSchemaSource{}, false, schemaTotalBytesLimitError(result.Err)
	}
	l.totalBytes += dataBytes
	if result.Err != nil {
		if item.optional && result.OpenNotFound {
			return loadedSchemaSource{}, false, nil
		}
		if result.LimitExceeded && readLimit == remaining {
			return loadedSchemaSource{}, false, schemaTotalBytesLimitError(result.Err)
		}
		if result.LimitExceeded || source.IsSchemaLimitError(result.Err) {
			return loadedSchemaSource{}, false, xsderrors.WithPath(name, result.Err)
		}
		readErr := xsderrors.SchemaParse(xsderrors.CodeSchemaRead, 0, 0, "read schema "+name, result.Err)
		return loadedSchemaSource{}, false, xsderrors.WithPath(name, readErr)
	}
	parseLimits := l.limits
	parseLimits.MaxSchemaInstantiatedNodes -= l.parsedNodes
	doc, err := parseSchemaDocument(name, key, data, parseLimits)
	if err != nil {
		return loadedSchemaSource{}, false, err
	}
	l.parsedNodes += doc.nodes
	if item.ref != nil {
		if targetErr := validateSchemaReferenceTarget(item.ref, doc); targetErr != nil {
			return loadedSchemaSource{}, false, targetErr
		}
	}
	doc.references, err = schemaDocumentReferences(doc)
	if err != nil {
		return loadedSchemaSource{}, false, err
	}
	if err := l.bindPendingReferences(key, doc); err != nil {
		return loadedSchemaSource{}, false, err
	}
	l.byKey[key] = loadedSchemaDocument{
		doc:          doc,
		sources:      []source.Source{src},
		data:         data,
		explicitRoot: item.ref == nil,
	}
	if err := l.enqueueReferences(src, doc.references, queue); err != nil {
		return loadedSchemaSource{}, false, err
	}
	return loadedSchemaSource{doc: doc, data: data}, true, nil
}

func (l *schemaSetLoader) readLoaded(
	item schemaLoadRequest,
	key string,
	loaded loadedSchemaDocument,
	queue *[]schemaLoadRequest,
) (loadedSchemaSource, bool, error) {
	src := item.source
	sameSource := false
	for _, existing := range loaded.sources {
		if !existing.SameResolutionContext(src) {
			continue
		}
		if existing.Name() == src.Name() {
			sameSource = true
		}
	}
	remaining := l.limits.MaxSchemaTotalBytes - l.totalBytes
	readLimit := min(l.limits.MaxSchemaSourceBytes, remaining)
	result := src.Acquire(readLimit)
	dataBytes := int64(len(result.Data))
	if dataBytes > remaining {
		return loadedSchemaSource{}, false, schemaTotalBytesLimitError(result.Err)
	}
	l.totalBytes += dataBytes
	// A resolved identity may reuse cached bytes when this optional context
	// cannot open its own representation. The context still owns descendant
	// resolution and must be registered below.
	useCached := item.optional && result.OpenNotFound
	if result.Err != nil && !useCached {
		if result.LimitExceeded && readLimit == remaining {
			return loadedSchemaSource{}, false, schemaTotalBytesLimitError(result.Err)
		}
		if result.LimitExceeded || source.IsSchemaLimitError(result.Err) {
			return loadedSchemaSource{}, false, xsderrors.WithPath(src.Name(), result.Err)
		}
		readErr := xsderrors.SchemaParse(xsderrors.CodeSchemaRead, 0, 0, "read schema "+src.Name(), result.Err)
		return loadedSchemaSource{}, false, xsderrors.WithPath(src.Name(), readErr)
	}
	if !useCached && !bytes.Equal(result.Data, loaded.data) {
		identityErr := xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "schema source identity resolves to different document content: "+key)
		if item.ref != nil {
			identityErr = withSchemaReferenceLocation(item, identityErr)
		} else {
			identityErr = xsderrors.WithPath(src.Name(), identityErr)
		}
		return loadedSchemaSource{}, false, identityErr
	}
	if item.ref != nil {
		if err := validateSchemaReferenceTarget(item.ref, loaded.doc); err != nil {
			return loadedSchemaSource{}, false, err
		}
	}
	if item.ref == nil && !loaded.explicitRoot {
		loaded.explicitRoot = true
		l.byKey[key] = loaded
	}
	if sameSource {
		return loadedSchemaSource{}, false, nil
	}
	loaded.sources = append(loaded.sources, src)
	l.byKey[key] = loaded
	if err := l.enqueueReferences(src, loaded.doc.references, queue); err != nil {
		return loadedSchemaSource{}, false, err
	}
	return loadedSchemaSource{}, false, nil
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
		if diagnostic.Code != xsderrors.CodeSchemaLimit {
			return true
		}
		return hasNonSchemaLimitCause(diagnostic.Err)
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
			source: src, ref: ref, base: base, referrer: src.Name(), optional: true, resolve: true,
		})
	}
	return nil
}

func (l *schemaSetLoader) resolveReference(request schemaLoadRequest) (schemaLoadRequest, bool, error) {
	resolution, err := request.source.ResolveFrom(request.base, request.ref.location)
	if err != nil {
		if source.IsReferenceResolutionError(err) {
			return schemaLoadRequest{}, false, schemaReferenceCompileAt(request.source, request.ref.node, "invalid schemaLocation: "+err.Error())
		}
		line, column := 0, 0
		if request.ref.node != nil {
			line, column = request.ref.node.Line, request.ref.node.Column
		}
		resolveErr := xsderrors.SchemaParse(xsderrors.CodeSchemaRead, line, column, "resolve schema "+request.ref.location.Raw(), err)
		resolveErr = xsderrors.WithPath(request.source.Name(), resolveErr)
		return schemaLoadRequest{}, false, resolveErr
	}
	target := resolution.Target()
	next, found := resolution.Source()
	if !found {
		if target == "" || request.ref.target == target {
			return schemaLoadRequest{}, false, nil
		}
		if loaded, ok := l.byKey[target]; ok && loaded.doc != nil {
			if request.ref.target != "" {
				return schemaLoadRequest{}, false, schemaReferenceCompileAt(request.source, request.ref.node, "schema reference resolves to different document identities across resolver contexts")
			}
			if err := validateSchemaReferenceTarget(request.ref, loaded.doc); err != nil {
				return schemaLoadRequest{}, false, err
			}
			request.ref.target = target
			return schemaLoadRequest{}, false, nil
		}
		l.deferReferenceBinding(target, request.ref)
		return schemaLoadRequest{}, false, nil
	}
	if request.ref.target != "" && request.ref.target != target {
		return schemaLoadRequest{}, false, schemaReferenceCompileAt(request.source, request.ref.node, "schema reference resolves to different document identities across resolver contexts")
	}
	if err := l.admitResolvedSource(target); err != nil {
		return schemaLoadRequest{}, false, withSchemaReferenceLocation(request, err)
	}
	request.ref.target = target
	return schemaLoadRequest{source: next, ref: request.ref, referrer: request.referrer, optional: true}, true, nil
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
	var refs []schemaReference
	rootBaseRaw, hasRootBase := doc.root.attrNS(vocab.XMLNamespaceURI, vocab.XMLAttrBase)
	var rootBase uriref.Reference
	validatedBase := source.NewReferenceBase(doc.name)
	if hasRootBase {
		var err error
		rootBase, err = uriref.Parse(rootBaseRaw)
		if err != nil {
			return nil, schemaCompileAt(doc.root, xsderrors.CodeSchemaReference, "invalid xml:base: "+err.Error())
		}
		validatedBase, err = validatedBase.WithXMLBase(rootBase)
		if err != nil {
			return nil, schemaCompileAt(doc.root, xsderrors.CodeSchemaReference, "invalid xml:base: "+err.Error())
		}
	}
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
		locationRaw, hasLocation := schemaLocationAttr(child)
		var location uriref.Reference
		if kind == schemaReferenceInclude && !hasLocation {
			return nil, schemaCompileAt(child, xsderrors.CodeSchemaReference, "include missing schemaLocation")
		}
		if hasLocation {
			var err error
			location, err = uriref.Parse(locationRaw)
			if err != nil {
				return nil, schemaCompileAt(child, xsderrors.CodeSchemaReference, "invalid schemaLocation: "+err.Error())
			}
		}
		localBaseRaw, hasLocalBase := child.attrNS(vocab.XMLNamespaceURI, vocab.XMLAttrBase)
		var localBase uriref.Reference
		if hasLocalBase {
			var err error
			localBase, err = uriref.Parse(localBaseRaw)
			if err != nil {
				return nil, schemaCompileAt(child, xsderrors.CodeSchemaReference, "invalid xml:base: "+err.Error())
			}
			if _, err := validatedBase.WithXMLBase(localBase); err != nil {
				return nil, schemaCompileAt(child, xsderrors.CodeSchemaReference, "invalid xml:base: "+err.Error())
			}
		}
		ref := schemaReference{
			node: child, kind: kind, location: location, hasLocation: hasLocation,
			rootBase: rootBase, localBase: localBase, hasRootBase: hasRootBase, hasLocalBase: hasLocalBase,
		}
		if kind == schemaReferenceImport {
			ref.namespace, _ = child.attr(vocab.XSDAttrNamespace)
			target := doc.defaults.TargetNamespace
			namespace, hasNamespace := child.attr(vocab.XSDAttrNamespace)
			switch {
			case hasNamespace && namespace == "":
				return nil, schemaCompileAt(child, xsderrors.CodeSchemaInvalidAttribute, "import namespace cannot be empty")
			case !hasNamespace && target == "":
				return nil, schemaCompileAt(child, xsderrors.CodeSchemaReference, "import without namespace requires enclosing schema targetNamespace")
			case hasNamespace && namespace == target:
				return nil, schemaCompileAt(child, xsderrors.CodeSchemaReference, "import namespace cannot match enclosing schema targetNamespace")
			}
		}
		refs = append(refs, ref)
	}
	return refs, nil
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
	return xsderrors.SchemaCompileAt(src.Name(), line, column, xsderrors.CodeSchemaReference, message)
}

func withSchemaReferenceLocation(request schemaLoadRequest, err error) error {
	if request.ref == nil || request.ref.node == nil || err == nil {
		return err
	}
	path := request.referrer
	if path == "" {
		path = request.source.Name()
	}
	return xsderrors.WithSchemaCompileLocation(path, request.ref.node.Line, request.ref.node.Column, err)
}

func schemaLocationAttr(n *rawNode) (string, bool) {
	return n.attr(vocab.XSDAttrSchemaLocation)
}

func (l *schemaSetLoader) validateLoadedReferenceTargets() error {
	for i := range l.documents {
		for j := range l.documents[i].doc.references {
			ref := &l.documents[i].doc.references[j]
			if ref.target == "" {
				continue
			}
			loaded, ok := l.byKey[ref.target]
			if !ok || loaded.doc == nil {
				continue
			}
			if err := validateSchemaReferenceTarget(ref, loaded.doc); err != nil {
				return err
			}
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

func (l *schemaSetLoader) instantiateTargetContexts() error {
	references, contexts := l.schemaTargetContextInputs()
	if contexts.documents == nil {
		if len(l.documents) > l.limits.MaxSchemaTargetContexts {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema target contexts exceed MaxSchemaTargetContexts")
		}
		for i := range l.documents {
			l.documents[i].effectiveTargetNS = l.documents[i].doc.defaults.TargetNamespace
		}
		return nil
	}

	for i, document := range l.documents {
		target := document.doc.defaults.TargetNamespace
		if target != "" || contexts.documents[i].importTarget || document.explicitRoot {
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

	l.applyTargetContexts(contexts)
	return nil
}

func (l *schemaSetLoader) schemaTargetContextInputs() ([]resolvedSchemaReference, schemaTargetContexts) {
	var references []resolvedSchemaReference
	var contexts schemaTargetContexts
	for i, document := range l.documents {
		start := len(references)
		for _, ref := range document.doc.references {
			if !ref.hasLocation {
				continue
			}
			if l.byKey[ref.target].doc == nil {
				continue
			}
			if contexts.documents == nil {
				contexts = newSchemaTargetContexts(l.documents, l.limits.MaxSchemaTargetContexts, l.limits.MaxSchemaInstantiatedNodes)
			}
			target := l.byKey[ref.target].index
			references = append(references, resolvedSchemaReference{
				location: ref.location.Raw(),
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
			if referenced.defaults.TargetNamespace != "" {
				continue
			}
			if err := contexts.add(ref.target, context.target); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *schemaSetLoader) applyTargetContexts(contexts schemaTargetContexts) {
	baseCount := len(l.documents)
	var clones []schemaSetDocument
	for i := range baseCount {
		document := &l.documents[i]
		state := contexts.documents[i]
		declaredTarget := document.doc.defaults.TargetNamespace
		document.effectiveTargetNS = state.primary
		document.adoptedTarget = declaredTarget == "" && state.primary != ""
	}
	for _, context := range contexts.additional {
		document := &l.documents[context.source]
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
	l.documents = append(l.documents, clones...)
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
	source   loadedSchemaSource
	identity int
}

func identifySchemaDocumentContents(sources []loadedSchemaSource) []identifiedSchemaDocument {
	ordered := slices.Clone(sources)
	slices.SortFunc(ordered, func(a, b loadedSchemaSource) int { return cmp.Compare(a.doc.key, b.doc.key) })
	seed := maphash.MakeSeed()
	type contentKey struct {
		size int
		hash uint64
	}
	type contentEntry struct {
		data     []byte
		identity int
	}
	seen := make(map[contentKey][]contentEntry)
	identified := make([]identifiedSchemaDocument, 0, len(ordered))
	nextIdentity := 1
	for _, src := range ordered {
		key := contentKey{size: len(src.data), hash: maphash.Bytes(seed, src.data)}
		identity := 0
		for _, entry := range seen[key] {
			if bytes.Equal(entry.data, src.data) {
				identity = entry.identity
				break
			}
		}
		if identity == 0 {
			identity = nextIdentity
			nextIdentity++
			seen[key] = append(seen[key], contentEntry{data: src.data, identity: identity})
		}
		identified = append(identified, identifiedSchemaDocument{source: src, identity: identity})
	}
	return identified
}

func (l *schemaSetLoader) selectDeclarationDocuments() {
	type declarationKey struct {
		target  string
		content int
	}
	seen := make(map[declarationKey]struct{}, len(l.documents))
	for i := range l.documents {
		document := &l.documents[i]
		key := declarationKey{target: document.effectiveTargetNS, content: document.contentIdentity}
		_, duplicate := seen[key]
		document.indexDeclarations = !duplicate
		seen[key] = struct{}{}
	}
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
