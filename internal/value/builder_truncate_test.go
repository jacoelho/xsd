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
	if err := builder.Truncate(removed); err != nil {
		t.Fatalf("Truncate(%d) error = %v", removed, err)
	}
	if got := builder.UserTypeCount(); got != 1 {
		t.Fatalf("UserTypeCount() = %d, want 1", got)
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
	if err := builder.Truncate(removed); err != nil {
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
	if err := builder.Truncate(removed); err != nil {
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
	if _, err := builder.Reserve(); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := builder.Truncate(completed); !errors.Is(err, ErrMetadata) {
		t.Fatalf("Truncate(%d) over a reserved slot error = %v, want ErrMetadata", completed, err)
	}
	if got := builder.UserTypeCount(); got != 2 {
		t.Fatalf("UserTypeCount() after a rejected Truncate = %d, want 2", got)
	}
}

func TestBuilderTruncateRejectsIDsOutsideTheUserTypes(t *testing.T) {
	t.Parallel()

	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	id := reserveCompleted(t, builder)
	for _, next := range []TypeID{BuiltinTypeCount - 1, id + 2, NoType} {
		if err := builder.Truncate(next); !errors.Is(err, ErrMetadata) {
			t.Errorf("Truncate(%d) error = %v, want ErrMetadata", next, err)
		}
	}
	if err := builder.Truncate(id + 1); err != nil {
		t.Fatalf("Truncate(%d), removing nothing, error = %v", id+1, err)
	}
	if got := builder.UserTypeCount(); got != 1 {
		t.Fatalf("UserTypeCount() = %d, want 1", got)
	}
}
