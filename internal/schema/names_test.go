package schema

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

func TestNameIDAllocatorsReserveAbsentSentinel(t *testing.T) {
	names, err := newNameTable(0, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	if names.ValidQName(noQName()) {
		t.Fatal("NameTable.ValidQName() accepted NoQName()")
	}
	if _, err := nextNamespaceID(-1); !errors.Is(err, errNamespaceLimit) {
		t.Fatalf("nextNamespaceID(-1) error = %v, want ErrNamespaceLimit", err)
	}
	if _, err := nextLocalNameID(-1); !errors.Is(err, errLocalNameLimit) {
		t.Fatalf("nextLocalNameID(-1) error = %v, want ErrLocalNameLimit", err)
	}
	if strconv.IntSize <= 32 {
		return
	}
	sentinel := uint64(invalidID)
	if id, err := nextNamespaceID(int(sentinel - 1)); err != nil || uint64(id) != sentinel-1 {
		t.Fatalf("nextNamespaceID(invalidID-1) = %d, %v", id, err)
	}
	if _, err := nextNamespaceID(int(sentinel)); !errors.Is(err, errNamespaceLimit) {
		t.Fatalf("nextNamespaceID(invalidID) error = %v, want ErrNamespaceLimit", err)
	}
	if id, err := nextLocalNameID(int(sentinel - 1)); err != nil || uint64(id) != sentinel-1 {
		t.Fatalf("nextLocalNameID(invalidID-1) = %d, %v", id, err)
	}
	if _, err := nextLocalNameID(int(sentinel)); !errors.Is(err, errLocalNameLimit) {
		t.Fatalf("nextLocalNameID(invalidID) error = %v, want ErrLocalNameLimit", err)
	}
	if _, err := nextLocalNameID(int(sentinel)); !errors.Is(err, errLocalNameLimit) {
		t.Fatalf("nextLocalNameID(local exhausted) error = %v, want ErrLocalNameLimit", err)
	}
}

var testRequiredNamespaces = []string{
	EmptyNamespaceURI,
	XSDNamespaceURI,
	XSINamespaceURI,
}

var testRequiredNames = []ExpandedName{
	{Namespace: XSINamespaceURI, Local: "type"},
	{Namespace: XSINamespaceURI, Local: "nil"},
}

func TestNameTableOwnsEmptyNamespaceIdentity(t *testing.T) {
	for _, namespaces := range [][]string{nil, {"urn:other"}, {"urn:other", ""}, {"", "urn:other", ""}} {
		names, err := newNameTable(0, namespaces, []ExpandedName{{Local: "item"}})
		if err != nil {
			t.Fatal(err)
		}
		name, ok := names.LookupQName("", "item")
		if !ok || names.Namespace(name.Namespace) != "" {
			t.Fatalf("unqualified name with seeds %q resolves to %q, found %v", namespaces, names.Namespace(name.Namespace), ok)
		}
		if name.Namespace != emptyNamespaceID {
			t.Fatalf("empty namespace ID = %d, want %d", name.Namespace, emptyNamespaceID)
		}
		view := newNameReadView(&names)
		frozen, ok := view.LookupQName("", "item")
		if !ok || view.Namespace(frozen.Namespace) != "" {
			t.Fatal("published name view changed the unqualified name's namespace")
		}
		if err := names.Validate(namespaces, []ExpandedName{{Local: "item"}}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNameTableChargesRequiredEmptyNamespace(t *testing.T) {
	if _, err := newNameTable(1, []string{"urn:other"}, nil); !errors.Is(err, errNameLimit) {
		t.Fatalf("one-slot table = %v, want name limit for two namespaces", err)
	}
	if _, err := newNameTable(2, []string{"urn:other"}, nil); err != nil {
		t.Fatalf("two-slot table = %v, want success", err)
	}
}

func TestNameTableValidationRejectsMisplacedEmptyNamespace(t *testing.T) {
	names := nameTable{
		namespaces: []string{"urn:other", ""},
		nsIndex:    map[string]namespaceID{"urn:other": 0, "": 1},
	}
	if err := names.Validate(nil, nil); err == nil {
		t.Fatal("Validate accepted an empty namespace with a nonzero ID")
	}
}

func TestNameTableLimitStopsGrowthAfterFailure(t *testing.T) {
	t.Parallel()

	seed, err := newNameTable(0, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	names, err := newNameTable(seed.NameCount()+1, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() with limit error = %v", err)
	}
	base := names.NameCount()
	interner := newNameInterner(&names)

	if _, err := interner.InternQName("urn:new", "new"); !errors.Is(err, errNameLimit) {
		t.Fatalf("InternQName() error = %v, want ErrNameLimit", err)
	}
	if got := names.NameCount(); got != base {
		t.Fatalf("name count after first failure = %d, want %d", got, base)
	}

	if _, err := interner.InternQName("urn:other", "other"); !errors.Is(err, errNameLimit) {
		t.Fatalf("second InternQName() error = %v, want ErrNameLimit", err)
	}
	if got := names.NameCount(); got != base {
		t.Fatalf("name count after second failure = %d, want %d", got, base)
	}
}

func TestNameTableValidateRejectsInconsistentIndexes(t *testing.T) {
	t.Parallel()

	names, err := newNameTable(0, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}

	missingLocal := names.Clone()
	delete(missingLocal.localIndex, "type")
	if err := missingLocal.Validate(testRequiredNamespaces, testRequiredNames); err == nil {
		t.Fatal("Validate() accepted missing local reverse map")
	}

	staleNamespace := names.Clone()
	staleNamespace.nsIndex[XSDNamespaceURI] = emptyNamespaceID
	if err := staleNamespace.Validate(testRequiredNamespaces, testRequiredNames); err == nil {
		t.Fatal("Validate() accepted stale namespace reverse map")
	}
}

func TestRuntimeNameTableIncludesRequiredSeeds(t *testing.T) {
	t.Parallel()

	names, err := newRuntimeNameTable(0)
	if err != nil {
		t.Fatalf("NewRuntimeNameTable() error = %v", err)
	}
	if err := validateRuntimeNameTable(&names); err != nil {
		t.Fatalf("ValidateRuntimeNameTable() error = %v", err)
	}
	for _, uri := range []string{
		EmptyNamespaceURI,
		XSDNamespaceURI,
		XSINamespaceURI,
		XMLNamespaceURI,
		XMLNSNamespaceURI,
	} {
		if _, ok := names.LookupNamespace(uri); !ok {
			t.Fatalf("runtime name table missing namespace %q", uri)
		}
	}
	for _, name := range []ExpandedName{
		{Namespace: XSINamespaceURI, Local: "type"},
		{Namespace: XSINamespaceURI, Local: "nil"},
		{Namespace: XSINamespaceURI, Local: "schemaLocation"},
		{Namespace: XSINamespaceURI, Local: "noNamespaceSchemaLocation"},
	} {
		if _, ok := names.LookupQName(name.Namespace, name.Local); !ok {
			t.Fatalf("runtime name table missing name %+v", name)
		}
	}

	missing := names.Clone()
	delete(missing.localIndex, "type")
	if err := validateRuntimeNameTable(&missing); err == nil {
		t.Fatal("ValidateRuntimeNameTable() accepted missing required name")
	}
}

func TestNameTableCloneDoesNotAlias(t *testing.T) {
	t.Parallel()

	names, err := newNameTable(0, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	clone := names.Clone()
	interner := newNameInterner(&names)
	if _, err := interner.InternQName("urn:new", "new"); err != nil {
		t.Fatalf("InternQName() error = %v", err)
	}

	if names.NameCount() == clone.NameCount() {
		t.Fatal("clone name count changed with original")
	}
	if _, ok := clone.LookupQName("urn:new", "new"); ok {
		t.Fatal("clone observed name interned after clone")
	}
}

func TestInternedNamesDoNotRetainTheirSourceString(t *testing.T) {
	names, err := newRuntimeNameTable(0)
	if err != nil {
		t.Fatal(err)
	}
	const source = `p:row | .//urn:only-in-this-attribute/cell`
	within := func(s string) bool {
		start := uintptr(unsafe.Pointer(unsafe.StringData(source))) //nolint:gosec // Compares string backing addresses only; nothing is dereferenced.
		at := uintptr(unsafe.Pointer(unsafe.StringData(s)))         //nolint:gosec // Compares string backing addresses only; nothing is dereferenced.
		return at >= start && at < start+uintptr(len(source))
	}
	namespace := source[strings.Index(source, "urn:"):strings.LastIndex(source, "/")]
	local := source[strings.LastIndex(source, "/")+1:]
	interner := newNameInterner(&names)
	q, err := interner.InternQName(namespace, local)
	if err != nil {
		t.Fatal(err)
	}
	if gotNS, gotLocal := names.Namespace(q.Namespace), names.Local(q.Local); within(gotNS) || within(gotLocal) || gotNS != "urn:only-in-this-attribute" || gotLocal != "cell" {
		t.Fatalf("interned {%s}%s shares the backing array of its source string", gotNS, gotLocal)
	}
	id, err := interner.InternNamespace(source[2:5])
	if err != nil {
		t.Fatal(err)
	}
	if ns := names.Namespace(id); within(ns) || ns != "row" {
		t.Fatalf("interned namespace %q shares the backing array of its source string", ns)
	}
	if allocs := testing.AllocsPerRun(10, func() {
		if _, err := interner.InternQName(namespace, local); err != nil {
			t.Fatal(err)
		}
	}); allocs != 0 {
		t.Fatalf("interning a held name allocated %v times, want 0", allocs)
	}
}

func TestInterningDistinctNamesAllocatesPerArenaChunkNotPerName(t *testing.T) {
	const count, length = 10_000, 12
	names, err := newBoundedNameTable(0, nil, nil, 1, count)
	if err != nil {
		t.Fatal(err)
	}
	source := make([]string, count)
	for i := range source {
		source[i] = fmt.Sprintf("name-%07d", i)
	}
	interner := newNameInterner(&names)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var last QName
	for _, local := range source {
		if last, err = interner.InternQName("", local); err != nil {
			t.Fatal(err)
		}
	}
	runtime.ReadMemStats(&after)
	const rampChunks = 4 // the arena with its first chunk, then 512, 1024 and 2048 bytes
	if nameArenaChunk>>rampChunks != nameArenaFirstChunk {
		t.Fatalf("rampChunks = %d does not match chunk sizes %d to %d", rampChunks, nameArenaFirstChunk, nameArenaChunk)
	}
	const chunks = (count*length+nameArenaChunk-1)/nameArenaChunk + rampChunks
	if allocs := after.Mallocs - before.Mallocs; allocs > chunks+2 {
		t.Fatalf("interning %d distinct %d-byte names allocated %d times, want at most %d arena chunks and 2 more", count, length, allocs, chunks)
	}
	if got := names.Local(last.Local); got != source[count-1] {
		t.Fatalf("last interned local name = %q, want %q", got, source[count-1])
	}
}

func TestNameArenaStoresANameLongerThanTheNextChunkInOneAllocation(t *testing.T) {
	short := strings.Repeat("s", nameArenaFirstChunk-1)
	long := strings.Repeat("l", 2*nameArenaFirstChunk+1)
	var stored string
	allocs := testing.AllocsPerRun(10, func() {
		arena := newNameArena()
		arena.store(short)
		stored = arena.store(long)
	})
	if allocs != 2 {
		t.Fatalf("a fresh arena storing a %d-byte name after a %d-byte one allocated %v times, want 2: the arena and one chunk", len(long), len(short), allocs)
	}
	if stored != long {
		t.Fatalf("stored name = %q, want %q", stored, long)
	}
}

func TestNamespaceURISortAndSearchFollowTheExpandedNameNamespaceOrder(t *testing.T) {
	t.Parallel()

	uris := []string{"urn:b", "", "urn:a", "http://example.com/", "urn:a:b", "URN:A"}
	sortNamespaceURIs(uris)
	if !slices.IsSortedFunc(uris, compareNamespaceURI) {
		t.Fatalf("sortNamespaceURIs() = %q, not in compareNamespaceURI order", uris)
	}
	for _, uri := range uris {
		if !searchNamespaceURI(uris, uri) {
			t.Fatalf("searchNamespaceURI(%q, %q) = false, want true", uris, uri)
		}
	}
	if searchNamespaceURI(uris, "urn:c") {
		t.Fatalf("searchNamespaceURI(%q, %q) = true, want false", uris, "urn:c")
	}
}
