package compiler

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestApplyParsedSuccess(t *testing.T) {
	t.Parallel()

	sch := parser.NewSchema()
	var got []string
	type entry struct{}

	out, err := ApplyParsed(sch, ApplyCallbacks[entry]{
		Begin: func() (*entry, func(), error) {
			got = append(got, "begin")
			return &entry{}, func() { got = append(got, "cleanup") }, nil
		},
		Init: func(*entry) error {
			got = append(got, "init")
			return nil
		},
		ApplyDirectives: func() error {
			got = append(got, "apply")
			return nil
		},
		Commit: func(*entry) {
			got = append(got, "commit")
		},
		ResolvePending: func() error {
			got = append(got, "resolve")
			return nil
		},
		RollbackPending: func() { got = append(got, "rollback-pending") },
		Rollback:        func(*entry) { got = append(got, "rollback") },
	})
	if err != nil {
		t.Fatalf("ApplyParsed() error = %v", err)
	}
	if out != sch {
		t.Fatal("ApplyParsed() did not return schema")
	}
	want := []string{"begin", "init", "apply", "commit", "resolve", "cleanup"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyParsed() steps = %v, want %v", got, want)
	}
}

func TestApplyParsedResolveErrorRollsBackCommittedState(t *testing.T) {
	t.Parallel()

	resolveErr := errors.New("resolve failed")
	var got []string
	type entry struct{}

	_, err := ApplyParsed(parser.NewSchema(), ApplyCallbacks[entry]{
		Begin: func() (*entry, func(), error) {
			got = append(got, "begin")
			return &entry{}, func() { got = append(got, "cleanup") }, nil
		},
		Commit: func(*entry) {
			got = append(got, "commit")
		},
		ResolvePending: func() error {
			got = append(got, "resolve")
			return resolveErr
		},
		RollbackPending: func() { got = append(got, "rollback-pending") },
		Rollback:        func(*entry) { got = append(got, "rollback") },
	})
	if !errors.Is(err, resolveErr) {
		t.Fatalf("ApplyParsed() error = %v, want %v", err, resolveErr)
	}
	want := []string{"begin", "commit", "resolve", "rollback-pending", "rollback", "cleanup"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyParsed() steps = %v, want %v", got, want)
	}
}

func TestApplyParsedDirectiveErrorDoesNotRollbackEntry(t *testing.T) {
	t.Parallel()

	applyErr := errors.New("apply failed")
	var got []string
	type entry struct{}

	_, err := ApplyParsed(parser.NewSchema(), ApplyCallbacks[entry]{
		Begin: func() (*entry, func(), error) {
			got = append(got, "begin")
			return &entry{}, func() { got = append(got, "cleanup") }, nil
		},
		ApplyDirectives: func() error {
			got = append(got, "apply")
			return applyErr
		},
		RollbackPending: func() { got = append(got, "rollback-pending") },
		Rollback:        func(*entry) { got = append(got, "rollback") },
	})
	if !errors.Is(err, applyErr) {
		t.Fatalf("ApplyParsed() error = %v, want %v", err, applyErr)
	}
	want := []string{"begin", "apply", "cleanup"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ApplyParsed() steps = %v, want %v", got, want)
	}
}
