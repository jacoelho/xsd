package runtime

import (
	"errors"
	"strconv"
	"testing"
)

func TestNameIDAllocatorsReserveAbsentSentinel(t *testing.T) {
	names, err := NewNameTable(0, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	if names.ValidQName(NoQName()) {
		t.Fatal("NameTable.ValidQName() accepted NoQName()")
	}
	if _, err := nextNamespaceID(-1); !errors.Is(err, ErrNamespaceLimit) {
		t.Fatalf("nextNamespaceID(-1) error = %v, want ErrNamespaceLimit", err)
	}
	if _, err := nextLocalNameID(-1); !errors.Is(err, ErrLocalNameLimit) {
		t.Fatalf("nextLocalNameID(-1) error = %v, want ErrLocalNameLimit", err)
	}
	if strconv.IntSize <= 32 {
		return
	}
	sentinel := uint64(invalidID)
	if id, err := nextNamespaceID(int(sentinel - 1)); err != nil || uint64(id) != sentinel-1 {
		t.Fatalf("nextNamespaceID(invalidID-1) = %d, %v", id, err)
	}
	if _, err := nextNamespaceID(int(sentinel)); !errors.Is(err, ErrNamespaceLimit) {
		t.Fatalf("nextNamespaceID(invalidID) error = %v, want ErrNamespaceLimit", err)
	}
	if id, err := nextLocalNameID(int(sentinel - 1)); err != nil || uint64(id) != sentinel-1 {
		t.Fatalf("nextLocalNameID(invalidID-1) = %d, %v", id, err)
	}
	if _, err := nextLocalNameID(int(sentinel)); !errors.Is(err, ErrLocalNameLimit) {
		t.Fatalf("nextLocalNameID(invalidID) error = %v, want ErrLocalNameLimit", err)
	}
	if _, _, err := nextQNameIDs(0, int(sentinel), true, true); !errors.Is(err, ErrLocalNameLimit) {
		t.Fatalf("nextQNameIDs(local exhausted) error = %v, want ErrLocalNameLimit", err)
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

func TestNameTableLimitStopsGrowthAfterFailure(t *testing.T) {
	t.Parallel()

	seed, err := NewNameTable(0, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	names, err := NewNameTable(seed.NameCount()+1, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() with limit error = %v", err)
	}
	base := names.NameCount()
	interner := NewNameInterner(&names)

	if _, err := interner.InternQName("urn:new", "new"); !errors.Is(err, ErrNameLimit) {
		t.Fatalf("InternQName() error = %v, want ErrNameLimit", err)
	}
	if got := names.NameCount(); got != base {
		t.Fatalf("name count after first failure = %d, want %d", got, base)
	}

	if _, err := interner.InternQName("urn:other", "other"); !errors.Is(err, ErrNameLimit) {
		t.Fatalf("second InternQName() error = %v, want ErrNameLimit", err)
	}
	if got := names.NameCount(); got != base {
		t.Fatalf("name count after second failure = %d, want %d", got, base)
	}
}

func TestNameTableValidateRejectsInconsistentIndexes(t *testing.T) {
	t.Parallel()

	names, err := NewNameTable(0, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}

	missingLocal := names.Clone()
	delete(missingLocal.localIndex, "type")
	if err := missingLocal.Validate(testRequiredNamespaces, testRequiredNames); err == nil {
		t.Fatal("Validate() accepted missing local reverse map")
	}

	staleNamespace := names.Clone()
	staleNamespace.nsIndex[XSDNamespaceURI] = EmptyNamespaceID
	if err := staleNamespace.Validate(testRequiredNamespaces, testRequiredNames); err == nil {
		t.Fatal("Validate() accepted stale namespace reverse map")
	}
}

func TestRuntimeNameTableIncludesRequiredSeeds(t *testing.T) {
	t.Parallel()

	names, err := NewRuntimeNameTable(0)
	if err != nil {
		t.Fatalf("NewRuntimeNameTable() error = %v", err)
	}
	if err := ValidateRuntimeNameTable(&names); err != nil {
		t.Fatalf("ValidateRuntimeNameTable() error = %v", err)
	}
	for _, uri := range []string{
		EmptyNamespaceURI,
		XSDNamespaceURI,
		XSINamespaceURI,
		XMLNamespaceURI,
		XLinkNamespaceURI,
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
	if err := ValidateRuntimeNameTable(&missing); err == nil {
		t.Fatal("ValidateRuntimeNameTable() accepted missing required name")
	}
}

func TestNameTableCloneDoesNotAlias(t *testing.T) {
	t.Parallel()

	names, err := NewNameTable(0, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	clone := names.Clone()
	interner := NewNameInterner(&names)
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

func TestValidateNameReadProjection(t *testing.T) {
	t.Parallel()

	names, err := NewNameTable(0, testRequiredNamespaces, testRequiredNames)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	read := NewNameReadView(&names)
	if err := ValidateNameReadProjection(read, &names); err != nil {
		t.Fatalf("ValidateNameReadProjection() error = %v", err)
	}

	changed := names.Clone()
	interner := NewNameInterner(&changed)
	if _, err := interner.InternQName("urn:new", "new"); err != nil {
		t.Fatalf("InternQName() error = %v", err)
	}
	if err := ValidateNameReadProjection(read, &changed); err == nil || err.Error() != "name read projection does not match name table" {
		t.Fatalf("ValidateNameReadProjection(changed) error = %v, want name read invariant", err)
	}
}
