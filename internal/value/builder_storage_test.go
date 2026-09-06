package value

import (
	"errors"
	"testing"
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

func TestBuilderAddConsumesReservedSlotWithoutDoubleCharge(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxStorageBytes: typeStorageBase})
	if err := builder.ReserveCapacity(1); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Add(minimalBuilderTypeSpec()); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := builder.Seal(); err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
}

func TestBuilderAddAndReserveCompleteChargeMetadataOnce(t *testing.T) {
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

	t.Run("queue and seal", func(t *testing.T) {
		t.Parallel()

		builder := NewBuilder(BuilderOptions{MaxStorageBytes: maxStorage})
		for i := range 2 {
			if _, err := builder.Add(spec); err != nil {
				t.Fatalf("Add(%d) = %v", i, err)
			}
		}
		if _, err := builder.Add(spec); !errors.Is(err, ErrLimit) {
			t.Fatalf("Add() after exact admission = %v, want ErrLimit", err)
		}
		if _, err := builder.Seal(); err != nil {
			t.Fatalf("Seal() = %v", err)
		}
	})
}

func TestBuilderQueuedTypesWithPreallocationChargeMetadataOnce(t *testing.T) {
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
		if _, err := builder.Add(spec); err != nil {
			t.Fatalf("Add(%d) = %v", i, err)
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
