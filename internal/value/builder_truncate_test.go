package value

import (
	"errors"
	"testing"
)

func reserveCompleted(t *testing.T, builder *Builder) TypeID {
	t.Helper()
	id, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := builder.Complete(id, minimalBuilderTypeSpec()); err != nil {
		t.Fatalf("Complete(%d) error = %v", id, err)
	}
	return id
}

func TestBuilderTruncateReissuesTheFirstRemovedID(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	kept := reserveCompleted(t, builder)
	removed := reserveCompleted(t, builder)
	reserveCompleted(t, builder)
	if err := builder.Truncate(removed, NoType); err != nil {
		t.Fatalf("Truncate(%d) error = %v", removed, err)
	}
	if _, ok := builder.TypeView(kept); !ok {
		t.Fatalf("TypeView(%d) reports no type, want the kept type", kept)
	}
	if _, ok := builder.TypeView(removed); ok {
		t.Fatalf("TypeView(%d) reports a type, want none after Truncate", removed)
	}
	if id, err := builder.Reserve(); err != nil || id != removed {
		t.Fatalf("Reserve() = %d, %v, want %d, nil", id, err, removed)
	}
}

func TestBuilderTruncateRefundsNoTypeLimit(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62, MaxTypes: uint32(BuiltinTypeCount) + 2})
	reserveCompleted(t, builder)
	removed := reserveCompleted(t, builder)
	if err := builder.Truncate(removed, NoType); err != nil {
		t.Fatalf("Truncate(%d) error = %v", removed, err)
	}
	if _, err := builder.Reserve(); !errors.Is(err, ErrLimit) {
		t.Fatalf("Reserve() after Truncate error = %v, want ErrLimit: the removed ID still counts against MaxTypes", err)
	}
}

func TestBuilderTruncateRefundsNoStorage(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62, MaxStorageBytes: 2 * typeStorageBase})
	reserveCompleted(t, builder)
	removed := reserveCompleted(t, builder)
	if err := builder.Truncate(removed, NoType); err != nil {
		t.Fatalf("Truncate(%d) error = %v", removed, err)
	}
	if _, err := builder.Reserve(); !errors.Is(err, ErrLimit) {
		t.Fatalf("Reserve() after Truncate error = %v, want ErrLimit: the removed slot's storage stays charged", err)
	}
}

func TestBuilderTruncateRejectsAnUncommittedSlot(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	completed := reserveCompleted(t, builder)
	reserved, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := builder.Truncate(completed, NoType); !errors.Is(err, ErrMetadata) {
		t.Fatalf("Truncate(%d) over a reserved slot error = %v, want ErrMetadata", completed, err)
	}
	if _, ok := builder.TypeView(completed); !ok {
		t.Fatalf("TypeView(%d) after a rejected Truncate reports no type, want the completed type", completed)
	}
	if err := builder.Complete(reserved, minimalBuilderTypeSpec()); err != nil {
		t.Fatalf("Complete(%d) after a rejected Truncate error = %v, want the slot still reserved", reserved, err)
	}
}

func TestBuilderTruncateRejectsIDsOutsideTheUserTypes(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	id := reserveCompleted(t, builder)
	for _, next := range []TypeID{BuiltinTypeCount - 1, id + 2, NoType} {
		if err := builder.Truncate(next, NoType); !errors.Is(err, ErrMetadata) {
			t.Errorf("Truncate(%d) error = %v, want ErrMetadata", next, err)
		}
	}
	if err := builder.Truncate(id+1, NoType); err != nil {
		t.Fatalf("Truncate(%d), removing nothing, error = %v", id+1, err)
	}
	if _, ok := builder.TypeView(id); !ok {
		t.Fatalf("TypeView(%d) reports no type, want it kept by the rejected and empty truncations", id)
	}
}

func TestBuilderTruncateMovesTheKeptTypeToTheFirstRemovedID(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	first := reserveCompleted(t, builder)
	reserveCompleted(t, builder)
	keep, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	spec := minimalBuilderTypeSpec()
	spec.Primitive = PrimitiveBoolean
	spec.Base = BuiltinType(PrimitiveBoolean)
	spec.Whitespace = WhitespaceCollapse
	if err := builder.Complete(keep, spec); err != nil {
		t.Fatalf("Complete(%d) error = %v", keep, err)
	}
	if err := builder.Truncate(first, keep); err != nil {
		t.Fatalf("Truncate(%d, %d) error = %v", first, keep, err)
	}
	if view, ok := builder.TypeView(first); !ok || view.Primitive != PrimitiveBoolean {
		t.Fatalf("TypeView(%d) = %+v, %v, want the kept boolean type", first, view, ok)
	}
	if id, err := builder.Reserve(); err != nil || id != first+1 {
		t.Fatalf("Reserve() = %d, %v, want %d, nil", id, err, first+1)
	}
}

func TestBuilderTruncateRejectsAKeptTypeNamingARemovedType(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	removed := reserveCompleted(t, builder)
	keep, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	spec := minimalBuilderTypeSpec()
	spec.Base = removed
	if err := builder.Complete(keep, spec); err != nil {
		t.Fatalf("Complete(%d) error = %v", keep, err)
	}
	if err := builder.Truncate(removed, keep); !errors.Is(err, ErrMetadata) {
		t.Fatalf("Truncate(%d, %d) error = %v, want ErrMetadata: the kept type's base is removed", removed, keep, err)
	}
	if _, ok := builder.TypeView(removed); !ok {
		t.Fatalf("TypeView(%d) after a rejected Truncate reports no type, want it kept", removed)
	}
	if view, ok := builder.TypeView(keep); !ok || view.Base != removed {
		t.Fatalf("TypeView(%d) after a rejected Truncate = %+v, %v, want it unmoved with base %d", keep, view, ok, removed)
	}
}

func TestBuilderReserveCapacityRejectsABuilderThatTruncated(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	removed := reserveCompleted(t, builder)
	if err := builder.Truncate(removed, NoType); err != nil {
		t.Fatalf("Truncate(%d) error = %v", removed, err)
	}
	if err := builder.ReserveCapacity(4); !errors.Is(err, ErrMetadata) {
		t.Fatalf("ReserveCapacity(4) after Truncate error = %v, want ErrMetadata", err)
	}
}
