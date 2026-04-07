package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestTrackValidatedUsesActualUnionMemberState(t *testing.T) {
	t.Parallel()

	var tracked []runtime.StringKind
	var lookedUp bool

	err := trackValidated(
		1,
		runtime.ValidatorsBundle{
			Meta: []runtime.ValidatorMeta{
				{},
				{Kind: runtime.VUnion, Flags: runtime.ValidatorMayTrackIDs},
				{Kind: runtime.VString, Flags: runtime.ValidatorMayTrackIDs, Index: 1},
			},
			String: []runtime.StringValidator{
				{},
				{Kind: runtime.StringIDREF},
			},
		},
		[]byte("abc"),
		2,
		Callbacks{
			Meta: func(id runtime.ValidatorID) (runtime.ValidatorMeta, bool, error) {
				if int(id) >= len([]runtime.ValidatorMeta{{}, {Kind: runtime.VUnion, Flags: runtime.ValidatorMayTrackIDs}, {Kind: runtime.VString, Flags: runtime.ValidatorMayTrackIDs, Index: 1}}) {
					return runtime.ValidatorMeta{}, false, nil
				}
				meta := []runtime.ValidatorMeta{
					{},
					{Kind: runtime.VUnion, Flags: runtime.ValidatorMayTrackIDs},
					{Kind: runtime.VString, Flags: runtime.ValidatorMayTrackIDs, Index: 1},
				}[id]
				return meta, true, nil
			},
			StringKind: func(meta runtime.ValidatorMeta) (runtime.StringKind, bool) {
				return runtime.StringIDREF, true
			},
			TrackString: func(kind runtime.StringKind, value []byte) error {
				tracked = append(tracked, kind)
				if string(value) != "abc" {
					t.Fatalf("TrackString() value = %q", value)
				}
				return nil
			},
			LookupUnionMember: func(runtime.ValidatorID, []byte) (runtime.ValidatorID, error) {
				lookedUp = true
				return 0, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("TrackValidated() error = %v", err)
	}
	if lookedUp {
		t.Fatal("TrackValidated() performed union lookup despite actual state")
	}
	if len(tracked) != 1 || tracked[0] != runtime.StringIDREF {
		t.Fatalf("TrackValidated() tracked = %v, want one IDREF", tracked)
	}
}

func TestTrackDefaultRecursesThroughListItems(t *testing.T) {
	t.Parallel()

	var tracked []string
	validators := runtime.ValidatorsBundle{
		Meta: []runtime.ValidatorMeta{
			{},
			{Kind: runtime.VList, Flags: runtime.ValidatorMayTrackIDs, Index: 1},
			{Kind: runtime.VString, Flags: runtime.ValidatorMayTrackIDs, Index: 1},
		},
		List: []runtime.ListValidator{
			{},
			{Item: 2},
		},
		String: []runtime.StringValidator{
			{},
			{Kind: runtime.StringID},
		},
	}

	err := trackDefault(
		1,
		validators,
		[]byte("one two"),
		0,
		Callbacks{
			Meta: func(id runtime.ValidatorID) (runtime.ValidatorMeta, bool, error) {
				if int(id) >= len(validators.Meta) {
					return runtime.ValidatorMeta{}, false, nil
				}
				return validators.Meta[id], true, nil
			},
			StringKind: func(meta runtime.ValidatorMeta) (runtime.StringKind, bool) {
				if int(meta.Index) >= len(validators.String) {
					return 0, false
				}
				return validators.String[meta.Index].Kind, true
			},
			TrackString: func(kind runtime.StringKind, value []byte) error {
				if kind != runtime.StringID {
					t.Fatalf("TrackString() kind = %v, want %v", kind, runtime.StringID)
				}
				tracked = append(tracked, string(value))
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("TrackDefault() error = %v", err)
	}
	if len(tracked) != 2 || tracked[0] != "one" || tracked[1] != "two" {
		t.Fatalf("TrackDefault() tracked = %v, want [one two]", tracked)
	}
}

func TestTrackValidatedIDsNilMetricsDoesNotPanic(t *testing.T) {
	t.Parallel()

	rt, validator := benchmarkCollapsedDoubleListRuntime()
	sess := NewSession(rt)
	input := benchmarkCollapsedDoubleList(16)

	if err := sess.trackValidatedIDs(validator, input, nil, nil); err != nil {
		t.Fatalf("trackValidatedIDs() error = %v", err)
	}
}
