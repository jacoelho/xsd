package compiler

import (
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/internal/schemaast"
)

func TestJournalRollbackReplaysOperationsInReverse(t *testing.T) {
	var got []string
	journal := Journal[string]{}
	journal.RecordMarkMerged(schemaast.DirectiveImport, "base", "target")
	journal.RecordAppendPendingDirective(schemaast.DirectiveInclude, "source", "dep")
	journal.RecordIncPendingCount("dep")

	journal.Rollback(RollbackCallbacks[string]{
		UnmarkMerged: func(kind schemaast.DirectiveKind, baseKey, targetKey string) {
			got = append(got, "unmark:"+baseKey+"->"+targetKey+":"+string(rune('0'+kind)))
		},
		RemovePendingDirective: func(kind schemaast.DirectiveKind, sourceKey, targetKey string) {
			got = append(got, "remove:"+sourceKey+"->"+targetKey+":"+string(rune('0'+kind)))
		},
		DecPendingCount: func(targetKey string) {
			got = append(got, "dec:"+targetKey)
		},
		CleanupKey: func(key string) {
			got = append(got, "cleanup:"+key)
		},
	})

	want := []string{
		"dec:dep",
		"cleanup:dep",
		"remove:source->dep:0",
		"cleanup:source",
		"unmark:base->target:1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rollback ops = %v, want %v", got, want)
	}
}

func TestJournalAppendPreservesOperationOrder(t *testing.T) {
	var got []string
	var left Journal[string]
	var right Journal[string]
	left.RecordMarkMerged(schemaast.DirectiveInclude, "left", "dep")
	right.RecordIncPendingCount("dep")

	left.Append(&right)
	left.Rollback(RollbackCallbacks[string]{
		UnmarkMerged: func(kind schemaast.DirectiveKind, baseKey, targetKey string) {
			got = append(got, "unmark:"+baseKey+"->"+targetKey)
		},
		DecPendingCount: func(targetKey string) {
			got = append(got, "dec:"+targetKey)
		},
	})

	want := []string{
		"dec:dep",
		"unmark:left->dep",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rollback ops = %v, want %v", got, want)
	}
}
