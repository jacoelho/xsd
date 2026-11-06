package schema

import (
	"errors"
	"maps"
	"slices"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

var (
	// errNameLimit reports that the configured total name limit was exceeded.
	errNameLimit = xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema name limit exceeded")
	// errNamespaceLimit reports that namespace IDs exceeded the runtime ID range.
	errNamespaceLimit = xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema namespace limit exceeded")
	// errLocalNameLimit reports that local-name IDs exceeded the runtime ID range.
	errLocalNameLimit = xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema local-name limit exceeded")
)

// ExpandedName is a namespace URI and local-name pair.
type ExpandedName struct {
	Namespace string
	Local     string
}

// nameTable interns namespace URIs and local names for a runtime schema.
type nameTable struct {
	nsIndex    map[string]namespaceID
	localIndex map[string]localNameID
	namespaces []string
	locals     []string
	maxNames   int
}

// nameReadView is a validation view over frozen runtime names.
type nameReadView struct {
	nsIndex    map[string]namespaceID
	localIndex map[string]localNameID
	namespaces []string
}

// newNameReadView returns an owned immutable read view of names.
func newNameReadView(names *nameTable) nameReadView {
	if names == nil {
		return nameReadView{}
	}
	return nameReadView{
		nsIndex:    maps.Clone(names.nsIndex),
		localIndex: maps.Clone(names.localIndex),
		namespaces: slices.Clone(names.namespaces),
	}
}

// LookupQName returns the interned QName for ns and local.
func (v nameReadView) LookupQName(ns, local string) (QName, bool) {
	localID, ok := v.localIndex[local]
	if !ok {
		return QName{}, false
	}
	if ns == "" {
		return QName{Namespace: emptyNamespaceID, Local: localID}, true
	}
	nsID, ok := v.nsIndex[ns]
	if !ok {
		return QName{}, false
	}
	return QName{Namespace: nsID, Local: localID}, true
}

// Namespace returns the URI for id, or "" when id is not valid.
func (v nameReadView) Namespace(id namespaceID) string {
	if !validRuntimeID(uint32(id), len(v.namespaces)) {
		return ""
	}
	return v.namespaces[id]
}

var requiredRuntimeNamespaces = []string{
	vocab.EmptyNamespaceURI,
	vocab.XSDNamespaceURI,
	vocab.XSINamespaceURI,
	vocab.XMLNamespaceURI,
	vocab.XMLNSNamespaceURI,
}

var requiredRuntimeNames = builtinXSIAttributeNames()

// newRuntimeNameTable returns a runtime name table seeded with required XML
// Schema namespaces and XSI attribute names.
func newRuntimeNameTable(maxNames int) (nameTable, error) {
	return newBoundedNameTable(
		maxNames,
		requiredRuntimeNamespaces,
		requiredRuntimeNames,
		len(requiredRuntimeNamespaces),
		runtimeNameLocalCapacity(),
	)
}

// newNameTable returns a name table seeded with required runtime names.
// Every table reserves emptyNamespaceID for the empty namespace independently
// of the caller's seed order.
func newNameTable(maxNames int, requiredNamespaces []string, requiredNames []ExpandedName) (nameTable, error) {
	return newBoundedNameTable(maxNames, requiredNamespaces, requiredNames, len(requiredNamespaces), len(requiredNames))
}

func newBoundedNameTable(maxNames int, requiredNamespaces []string, requiredNames []ExpandedName, namespaceCap, localCap int) (nameTable, error) {
	namespaceCap = max(1, namespaceCap, len(requiredNamespaces))
	localCap = max(localCap, len(requiredNames))
	n := nameTable{
		nsIndex:    make(map[string]namespaceID, namespaceCap),
		localIndex: make(map[string]localNameID, localCap),
		namespaces: make([]string, 0, namespaceCap),
		locals:     make([]string, 0, localCap),
		maxNames:   maxNames,
	}
	interner := newNameInterner(&n)
	if _, err := interner.InternNamespace(vocab.EmptyNamespaceURI); err != nil {
		return nameTable{}, err
	}
	for _, uri := range requiredNamespaces {
		if uri == vocab.EmptyNamespaceURI {
			continue
		}
		if _, err := interner.InternNamespace(uri); err != nil {
			return nameTable{}, err
		}
	}
	for _, name := range requiredNames {
		if _, err := interner.InternQName(name.Namespace, name.Local); err != nil {
			return nameTable{}, err
		}
	}
	return n, nil
}

func runtimeNameLocalCapacity() int {
	return len(requiredRuntimeNames) +
		builtinSimpleTypeCount() +
		1 // xs:anyType
}

// Clone returns a deep copy of n.
func (n *nameTable) Clone() nameTable {
	return nameTable{
		nsIndex:    maps.Clone(n.nsIndex),
		localIndex: maps.Clone(n.localIndex),
		namespaces: slices.Clone(n.namespaces),
		locals:     slices.Clone(n.locals),
		maxNames:   n.maxNames,
	}
}

// NameCount returns the total number of interned namespace URIs and local names.
func (n *nameTable) NameCount() int {
	return len(n.namespaces) + len(n.locals)
}

// validateRuntimeNameTable validates runtime name-table shape and required
// runtime namespace/name seeds.
func validateRuntimeNameTable(n *nameTable) error {
	if n == nil {
		return errors.New("runtime name table is missing")
	}
	return n.Validate(requiredRuntimeNamespaces, requiredRuntimeNames)
}

// Validate reports whether n is internally consistent and contains required names.
func (n *nameTable) Validate(requiredNamespaces []string, requiredNames []ExpandedName) error {
	if n == nil || len(n.namespaces) == 0 || n.namespaces[emptyNamespaceID] != vocab.EmptyNamespaceURI {
		return errors.New("name table must reserve namespace zero for the empty namespace")
	}
	if len(n.nsIndex) != len(n.namespaces) {
		return errors.New("name table namespace index size does not match namespace slice")
	}
	if len(n.localIndex) != len(n.locals) {
		return errors.New("name table local index size does not match local slice")
	}
	if err := n.validateNamespaceIndex(); err != nil {
		return err
	}
	if err := n.validateLocalIndex(); err != nil {
		return err
	}
	if err := n.validateRequiredNamespaces(requiredNamespaces); err != nil {
		return err
	}
	return n.validateRequiredNames(requiredNames)
}

func (n *nameTable) validateNamespaceIndex() error {
	for i, uri := range n.namespaces {
		id, ok := n.nsIndex[uri]
		if !ok || id != namespaceID(i) {
			return errors.New("name table namespace index does not match namespace slice")
		}
	}
	for uri, id := range n.nsIndex {
		if !validRuntimeID(uint32(id), len(n.namespaces)) || n.namespaces[id] != uri {
			return errors.New("name table namespace slice does not match namespace index")
		}
	}
	return nil
}

func (n *nameTable) validateLocalIndex() error {
	for i, local := range n.locals {
		id, ok := n.localIndex[local]
		if !ok || id != localNameID(i) {
			return errors.New("name table local index does not match local slice")
		}
	}
	for local, id := range n.localIndex {
		if !validRuntimeID(uint32(id), len(n.locals)) || n.locals[id] != local {
			return errors.New("name table local slice does not match local index")
		}
	}
	return nil
}

func (n *nameTable) validateRequiredNamespaces(required []string) error {
	for _, uri := range required {
		id, ok := n.LookupNamespace(uri)
		if !ok || n.Namespace(id) != uri {
			return errors.New("name table is missing required namespace")
		}
	}
	return nil
}

func (n *nameTable) validateRequiredNames(required []ExpandedName) error {
	for _, name := range required {
		q, ok := n.LookupQName(name.Namespace, name.Local)
		if !ok || !n.ValidQName(q) {
			return errors.New("name table is missing required name")
		}
	}
	return nil
}

// ValidQName reports whether q indexes interned namespace and local names.
func (n *nameTable) ValidQName(q QName) bool {
	return validRuntimeID(uint32(q.Namespace), len(n.namespaces)) &&
		validRuntimeID(uint32(q.Local), len(n.locals))
}

// ValidNamespaceID reports whether id indexes an interned namespace URI.
func (n *nameTable) ValidNamespaceID(id namespaceID) bool {
	return validRuntimeID(uint32(id), len(n.namespaces))
}

// LookupNamespace returns the interned namespace ID for uri.
func (n *nameTable) LookupNamespace(uri string) (namespaceID, bool) {
	id, ok := n.nsIndex[uri]
	return id, ok
}

// LookupLocal returns the interned local-name ID for local.
func (n *nameTable) LookupLocal(local string) (localNameID, bool) {
	id, ok := n.localIndex[local]
	return id, ok
}

// LookupQName returns the interned QName for ns and local.
func (n *nameTable) LookupQName(ns, local string) (QName, bool) {
	if ns == "" {
		localID, ok := n.LookupLocal(local)
		if !ok {
			return QName{}, false
		}
		return QName{Namespace: emptyNamespaceID, Local: localID}, true
	}
	nsID, ok := n.LookupNamespace(ns)
	if !ok {
		return QName{}, false
	}
	localID, ok := n.LookupLocal(local)
	if !ok {
		return QName{}, false
	}
	return QName{Namespace: nsID, Local: localID}, true
}

// Namespace returns the URI for id, or "" when id is not valid.
func (n *nameTable) Namespace(id namespaceID) string {
	if !n.ValidNamespaceID(id) {
		return ""
	}
	return n.namespaces[id]
}

// Local returns the local name for id, or "" when id is not valid.
func (n *nameTable) Local(id localNameID) string {
	if !validRuntimeID(uint32(id), len(n.locals)) {
		return ""
	}
	return n.locals[id]
}

// Format returns q as a bounded, escaped expanded name for diagnostics.
func (n *nameTable) Format(q QName) string {
	return diagnostictext.ExpandedName(n.Namespace(q.Namespace), n.Local(q.Local))
}

// nameInterner interns names into a nameTable.
type nameInterner struct {
	table *nameTable
}

// newNameInterner returns an interner that writes to table.
func newNameInterner(table *nameTable) nameInterner {
	return nameInterner{table: table}
}

// InternNamespace interns uri and returns its namespace ID.
func (n nameInterner) InternNamespace(uri string) (namespaceID, error) {
	table := n.table
	if id, ok := table.nsIndex[uri]; ok {
		return id, nil
	}
	if err := table.checkLimit(1); err != nil {
		return 0, err
	}
	id, err := nextNamespaceID(len(table.namespaces))
	if err != nil {
		return 0, err
	}
	table.namespaces = append(table.namespaces, uri)
	table.nsIndex[uri] = id
	return id, nil
}

// InternQName interns ns and local and returns the corresponding QName.
func (n nameInterner) InternQName(ns, local string) (QName, error) {
	table := n.table
	nsID, nsOK := table.nsIndex[ns]
	localID, localOK := table.localIndex[local]
	need := 0
	if !nsOK {
		need++
	}
	if !localOK {
		need++
	}
	if err := table.checkLimit(need); err != nil {
		return QName{}, err
	}
	if !nsOK {
		next, err := nextNamespaceID(len(table.namespaces))
		if err != nil {
			return QName{}, err
		}
		nsID = next
		table.namespaces = append(table.namespaces, ns)
		table.nsIndex[ns] = nsID
	}
	if !localOK {
		next, err := nextLocalNameID(len(table.locals))
		if err != nil {
			return QName{}, err
		}
		localID = next
		table.locals = append(table.locals, local)
		table.localIndex[local] = localID
	}
	return QName{Namespace: nsID, Local: localID}, nil
}

func (n *nameTable) checkLimit(need int) error {
	if n.maxNames <= 0 || need <= 0 {
		return nil
	}
	if n.NameCount()+need > n.maxNames {
		return errNameLimit
	}
	return nil
}

func nextNamespaceID(n int) (namespaceID, error) {
	if n < 0 || uint64(n) >= uint64(invalidID) {
		return 0, errNamespaceLimit
	}
	return namespaceID(n), nil
}

func nextLocalNameID(n int) (localNameID, error) {
	if n < 0 || uint64(n) >= uint64(invalidID) {
		return 0, errLocalNameLimit
	}
	return localNameID(n), nil
}
