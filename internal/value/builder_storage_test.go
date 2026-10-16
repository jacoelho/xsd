package value

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

func minimalBuilderTypeSpec() TypeSpec {
	return TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	}
}

func metadataBuilderTypeSpec() TypeSpec {
	return TypeSpec{
		Variety:           Atomic,
		Primitive:         PrimitiveString,
		Whitespace:        WhitespacePreserve,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{
			Lexical: "value",
			Type:    NoType,
		}}},
	}
}

func TestBuilderReserveCapacityChargesSlotAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		maxStorage uint64
		userTypes  int
		wantErr    bool
	}{
		{name: "below", maxStorage: typeStorageBase - 1, userTypes: 1, wantErr: true},
		{name: "at", maxStorage: typeStorageBase, userTypes: 1},
		{name: "above", maxStorage: typeStorageBase, userTypes: 2, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			builder := NewBuilder(BuilderOptions{MaxStorageBytes: test.maxStorage})
			err := builder.ReserveCapacity(test.userTypes)
			if test.wantErr {
				if !errors.Is(err, ErrLimit) {
					t.Fatalf("ReserveCapacity() error = %v, want ErrLimit", err)
				}
				if test.name == "above" {
					id, reserveErr := builder.Reserve()
					if reserveErr != nil {
						t.Fatalf("Reserve() after rejected larger target = %v", reserveErr)
					}
					if completeErr := builder.Complete(id, minimalBuilderTypeSpec()); completeErr != nil {
						t.Fatalf("Complete() after rejected larger target = %v", completeErr)
					}
					if _, sealErr := builder.Seal(); sealErr != nil {
						t.Fatalf("Seal() after rejected larger target = %v", sealErr)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cap(builder.program.types) < test.userTypes || cap(builder.program.complete) < test.userTypes {
				t.Fatalf("ReserveCapacity did not grow both slices: types cap=%d complete cap=%d", cap(builder.program.types), cap(builder.program.complete))
			}
			if test.name == "at" {
				id, reserveErr := builder.Reserve()
				if reserveErr != nil {
					t.Fatalf("Reserve() at storage boundary = %v", reserveErr)
				}
				if completeErr := builder.Complete(id, minimalBuilderTypeSpec()); completeErr != nil {
					t.Fatalf("Complete() at storage boundary = %v", completeErr)
				}
				if _, reserveErr := builder.Reserve(); !errors.Is(reserveErr, ErrLimit) {
					t.Fatalf("second Reserve() at storage boundary = %v, want ErrLimit", reserveErr)
				}
				if _, sealErr := builder.Seal(); sealErr != nil {
					t.Fatalf("Seal() at storage boundary = %v", sealErr)
				}
			}
		})
	}
}

func TestBuilderReserveCapacityFailureCanBeRetried(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxStorageBytes: typeStorageBase})
	if err := builder.ReserveCapacity(2); !errors.Is(err, ErrLimit) {
		t.Fatalf("initial ReserveCapacity() error = %v, want ErrLimit", err)
	}
	if err := builder.ReserveCapacity(1); err != nil {
		t.Fatalf("retry ReserveCapacity() error = %v", err)
	}
	id, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() after retry = %v", err)
	}
	if err := builder.Complete(id, minimalBuilderTypeSpec()); err != nil {
		t.Fatalf("Complete() after retry = %v", err)
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("Seal() after retry = %v", err)
	}
}

func TestBuilderReserveCapacityKeepsCompletionStateAfterReserve(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxStorageBytes: 2 * typeStorageBase})
	first, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err = builder.ReserveCapacity(2); err != nil {
		t.Fatal(err)
	}
	second, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec := minimalBuilderTypeSpec()
	if err := builder.Complete(first, spec); err != nil {
		t.Fatalf("Complete(first) error = %v", err)
	}
	if err := builder.Complete(second, spec); err != nil {
		t.Fatalf("Complete(second) error = %v", err)
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
}

func TestBuilderReserveCompleteConsumesReservedSlotWithoutDoubleCharge(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxStorageBytes: typeStorageBase})
	if err := builder.ReserveCapacity(1); err != nil {
		t.Fatal(err)
	}
	reservedID, reserveErr := builder.Reserve()
	if reserveErr != nil {
		t.Fatalf("Reserve() error = %v", reserveErr)
	}
	if err := builder.Complete(reservedID, minimalBuilderTypeSpec()); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
}

func TestBuilderReserveCompleteChargesMetadataOnce(t *testing.T) {
	t.Parallel()

	spec := metadataBuilderTypeSpec()
	storage, ok := typeSpecStorage(spec)
	if !ok || storage <= typeStorageBase {
		t.Fatalf("metadata type storage = %d, %v; want more than slot base", storage, ok)
	}
	maxStorage := 2 * storage

	t.Run("reserve and complete", func(t *testing.T) {
		t.Parallel()

		builder := NewBuilder(BuilderOptions{MaxStorageBytes: maxStorage})
		for i := range 2 {
			id, err := builder.Reserve()
			if err != nil {
				t.Fatalf("Reserve(%d) = %v", i, err)
			}
			if err := builder.Complete(id, spec); err != nil {
				t.Fatalf("Complete(%d) = %v", i, err)
			}
		}
		if _, err := builder.Reserve(); !errors.Is(err, ErrLimit) {
			t.Fatalf("Reserve() after exact admission = %v, want ErrLimit", err)
		}
		if _, err := builder.Seal(); err != nil {
			t.Fatalf("Seal() = %v", err)
		}
	})
}

func TestBuilderCompletedTypesWithPreallocationChargeMetadataOnce(t *testing.T) {
	t.Parallel()

	spec := metadataBuilderTypeSpec()
	storage, ok := typeSpecStorage(spec)
	if !ok {
		t.Fatal("metadata type storage overflowed")
	}
	builder := NewBuilder(BuilderOptions{MaxStorageBytes: 2 * storage})
	if err := builder.ReserveCapacity(2); err != nil {
		t.Fatalf("ReserveCapacity() = %v", err)
	}
	for i := range 2 {
		reservedID, reserveErr := builder.Reserve()
		if reserveErr != nil {
			t.Fatalf("Reserve() error = %v", reserveErr)
		}
		if err := builder.Complete(reservedID, spec); err != nil {
			t.Fatalf("Complete(%d) = %v", i, err)
		}
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("Seal() = %v", err)
	}
}

func TestBuilderCompleteFailureCanRetryWithoutExtraCharge(t *testing.T) {
	t.Parallel()

	valid := metadataBuilderTypeSpec()
	invalid := valid
	invalid.Facets = FacetSpec{Present: FacetEnumeration}
	storage, ok := typeSpecStorage(valid)
	if !ok {
		t.Fatal("metadata type storage overflowed")
	}
	builder := NewBuilder(BuilderOptions{MaxStorageBytes: storage})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() = %v", err)
	}
	if err := builder.Complete(id, invalid); err == nil {
		t.Fatal("Complete(invalid) succeeded")
	}
	if err := builder.Complete(id, valid); err != nil {
		t.Fatalf("Complete(valid) after failure = %v", err)
	}
	if _, err := builder.Reserve(); !errors.Is(err, ErrLimit) {
		t.Fatalf("Reserve() after retry at exact budget = %v, want ErrLimit", err)
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("Seal() after retry = %v", err)
	}
}

func TestBuilderReserveCapacityRepeatedAdmissionIsIncremental(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxStorageBytes: 2 * typeStorageBase})
	if err := builder.ReserveCapacity(1); err != nil {
		t.Fatal(err)
	}
	if err := builder.ReserveCapacity(1); err != nil {
		t.Fatalf("same target ReserveCapacity() error = %v", err)
	}
	if err := builder.ReserveCapacity(2); err != nil {
		t.Fatalf("larger target ReserveCapacity() error = %v", err)
	}
	if cap(builder.program.types) < 2 || cap(builder.program.complete) < 2 {
		t.Fatalf("larger target capacities=%d/%d, want at least 2", cap(builder.program.types), cap(builder.program.complete))
	}
	if err := builder.ReserveCapacity(3); !errors.Is(err, ErrLimit) {
		t.Fatalf("over-budget ReserveCapacity() error = %v, want ErrLimit", err)
	}
	first, err := builder.Reserve()
	if err != nil {
		t.Fatalf("first Reserve() after rejected larger target = %v", err)
	}
	second, err := builder.Reserve()
	if err != nil {
		t.Fatalf("second Reserve() after rejected larger target = %v", err)
	}
	spec := minimalBuilderTypeSpec()
	if err := builder.Complete(first, spec); err != nil {
		t.Fatalf("Complete(first) after rejected larger target = %v", err)
	}
	if err := builder.Complete(second, spec); err != nil {
		t.Fatalf("Complete(second) after rejected larger target = %v", err)
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("Seal() after rejected larger target = %v", err)
	}
}

func TestBuilderReserveCapacityGrowsToRequestedTargetAfterSlack(t *testing.T) {
	t.Parallel()

	const firstTarget = 100
	const secondTarget = 200
	builder := NewBuilder(BuilderOptions{MaxStorageBytes: secondTarget * typeStorageBase})
	if err := builder.ReserveCapacity(firstTarget); err != nil {
		t.Fatal(err)
	}
	if err := builder.ReserveCapacity(secondTarget); err != nil {
		t.Fatal(err)
	}
	if cap(builder.program.types) < secondTarget || cap(builder.program.complete) < secondTarget {
		t.Fatalf("capacity after repeated growth=%d/%d, want at least %d", cap(builder.program.types), cap(builder.program.complete), secondTarget)
	}
}

func TestBuilderCompletionRequiresCompletedDependencies(t *testing.T) {
	t.Parallel()
	builder := NewBuilder(BuilderOptions{})
	derived, reserveErr := builder.Reserve()
	if reserveErr != nil {
		t.Fatal(reserveErr)
	}
	base, reserveErr := builder.Reserve()
	if reserveErr != nil {
		t.Fatal(reserveErr)
	}
	neighbor, reserveErr := builder.Reserve()
	if reserveErr != nil {
		t.Fatal(reserveErr)
	}
	if derived != BuiltinTypeCount || base != BuiltinTypeCount+1 || neighbor != BuiltinTypeCount+2 {
		t.Fatalf("reserved IDs = %d, %d, %d", derived, base, neighbor)
	}
	if err := builder.Complete(neighbor, minimalBuilderTypeSpec()); err != nil {
		t.Fatal(err)
	}
	derivedSpec := TypeSpec{Variety: Atomic, Primitive: PrimitiveDecimal, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: base, ListItem: NoType}
	if err := builder.Complete(derived, derivedSpec); !errors.Is(err, ErrMetadata) {
		t.Fatalf("Complete before base = %v, want ErrMetadata", err)
	}
	if _, available := builder.TypeView(derived); available {
		t.Fatal("failed completion made type metadata available")
	}
	if _, available := builder.InputRequirements(derived); available {
		t.Fatal("failed completion made input requirements available")
	}
	if _, err := builder.Validate(derived, "10", Resolver{}, 0, nil); !errors.Is(err, ErrMetadata) {
		t.Fatalf("Validate incomplete type = %v, want ErrMetadata", err)
	}
	if _, err := builder.Seal(); !errors.Is(err, ErrMetadata) {
		t.Fatalf("Seal incomplete builder = %v, want ErrMetadata", err)
	}
	if got, err := builder.Validate(neighbor, "unchanged", Resolver{}, NeedCanonical, nil); err != nil || got.CanonicalText() != "unchanged" {
		t.Fatalf("completed neighbor after failed Seal = %q, %v", got.CanonicalText(), err)
	}
	baseSpec := TypeSpec{Variety: Atomic, Primitive: PrimitiveDecimal, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType, Facets: FacetSpec{MinInclusive: BoundFacet{Present: true, LiteralSpec: LiteralSpec{Lexical: "10", Type: NoType}}}}
	if err := builder.Complete(base, baseSpec); err != nil {
		t.Fatal(err)
	}
	if err := builder.Complete(derived, derivedSpec); err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(derived, "9", Resolver{}, 0, 16<<20, nil); !errors.Is(err, ErrFacet) {
		t.Fatalf("derived value below inherited bound = %v, want ErrFacet", err)
	}
	if _, err := program.Validate(derived, "10", Resolver{}, 0, 16<<20, nil); err != nil {
		t.Fatalf("derived value at inherited bound = %v", err)
	}
	if got, err := program.Validate(neighbor, "unchanged", Resolver{}, NeedCanonical, 16<<20, nil); err != nil || got.CanonicalText() != "unchanged" {
		t.Fatalf("published neighbor = %q, %v", got.CanonicalText(), err)
	}
}

func TestBuilderCompleteOwnsSourceMetadata(t *testing.T) {
	t.Parallel()
	pattern, patternErr := xsdregex.Compile("true", xsdregex.CompileOptions{})
	if patternErr != nil {
		t.Fatal(patternErr)
	}
	replacement, replacementErr := xsdregex.Compile("false", xsdregex.CompileOptions{})
	if replacementErr != nil {
		t.Fatal(replacementErr)
	}
	spec := TypeSpec{Variety: Union, Whitespace: WhitespaceCollapse, WhitespacePresent: true, Base: NoType, ListItem: NoType, Union: []TypeID{builtinBoolean, builtinDecimal}, Facets: FacetSpec{Patterns: [][]*Pattern{{pattern}}, Enumeration: []LiteralSpec{{Lexical: "true", Type: NoType}}}}
	builder := NewBuilder(BuilderOptions{})
	id, reserveErr := builder.Reserve()
	if reserveErr != nil {
		t.Fatal(reserveErr)
	}
	if err := builder.Complete(id, spec); err != nil {
		t.Fatal(err)
	}
	spec.Union[0] = builtinDecimal
	spec.Facets.Patterns[0][0] = replacement
	spec.Facets.Enumeration[0].Lexical = "false"
	if got, err := builder.Validate(id, "true", Resolver{}, NeedCanonical, nil); err != nil || got.CanonicalText() != "true" || got.SelectedType() != builtinBoolean {
		t.Fatalf("builder original value = canonical %q selected %d error %v, want true/boolean", got.CanonicalText(), got.SelectedType(), err)
	}
	if _, err := builder.Validate(id, "false", Resolver{}, 0, nil); !errors.Is(err, ErrFacet) {
		t.Fatalf("builder replacement value = %v, want ErrFacet", err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := program.Validate(id, "true", Resolver{}, NeedCanonical, 16<<20, nil); err != nil || got.CanonicalText() != "true" || got.SelectedType() != builtinBoolean {
		t.Fatalf("published original value = canonical %q selected %d error %v, want true/boolean", got.CanonicalText(), got.SelectedType(), err)
	}
	if _, err := program.Validate(id, "false", Resolver{}, 0, 16<<20, nil); !errors.Is(err, ErrFacet) {
		t.Fatalf("published replacement value = %v, want ErrFacet", err)
	}
}
