package schema

import (
	"errors"
	"strconv"
	"testing"
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
