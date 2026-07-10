package validate

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentityStateRejectsDuplicateIDWithoutMutating(t *testing.T) {
	t.Parallel()

	var state IdentityState
	ctx := StartContext{Path: "/first", Line: 2, Column: 3}
	if err := recordValueForTest(&state, IdentityValue{IDs: "a"}, ctx); err != nil {
		t.Fatalf("RecordValue(first) error = %v", err)
	}
	err := recordValueForTest(&state, IdentityValue{IDs: "a"}, StartContext{Path: "/second", Line: 4, Column: 5})
	expectXSDCode(t, err, xsderrors.CodeValidationType)
	expectXSDMessage(t, err, "duplicate ID a first seen at /first")
	if state.entries != 1 {
		t.Fatalf("entries = %d, want 1", state.entries)
	}
	if got := state.ids["a"]; got != "/first" {
		t.Fatalf("ids[a] = %q, want /first", got)
	}
}

func TestIdentityStateResolvesIDREFAgainstLaterID(t *testing.T) {
	t.Parallel()

	var state IdentityState
	if err := recordValueForTest(&state, IdentityValue{IDRefs: "a"}, StartContext{Path: "/ref", Line: 2, Column: 3}); err != nil {
		t.Fatalf("RecordValue(IDREF) error = %v", err)
	}
	if err := recordValueForTest(&state, IdentityValue{IDs: "a"}, StartContext{Path: "/id", Line: 4, Column: 5}); err != nil {
		t.Fatalf("RecordValue(ID) error = %v", err)
	}
	err := state.CheckIDRefs(func(err error) error {
		t.Fatalf("CheckIDRefs reported resolved ref: %v", err)
		return nil
	})
	if err != nil {
		t.Fatalf("CheckIDRefs() error = %v", err)
	}
}

func TestIdentityStateReportsMissingIDREFAtOriginalLocation(t *testing.T) {
	t.Parallel()

	var state IdentityState
	if err := recordValueForTest(&state, IdentityValue{IDRefs: "missing"}, StartContext{Path: "/ref", Line: 2, Column: 3}); err != nil {
		t.Fatalf("RecordValue(IDREF) error = %v", err)
	}
	var got error
	err := state.CheckIDRefs(func(err error) error {
		got = err
		return nil
	})
	if err != nil {
		t.Fatalf("CheckIDRefs() error = %v", err)
	}
	expectXSDCode(t, got, xsderrors.CodeValidationType)
	expectXSDMessage(t, got, "IDREF does not resolve: missing")
	expectXSDLocation(t, got, "/ref", 2, 3)
}

func TestIdentityStateUsesXMLWhitespaceFields(t *testing.T) {
	t.Parallel()

	var state IdentityState
	if err := recordValueForTest(&state, IdentityValue{IDs: "a\tb\nc"}, StartContext{Path: "/ids", Line: 2, Column: 3}); err != nil {
		t.Fatalf("RecordValue(IDs) error = %v", err)
	}
	if len(state.ids) != 3 {
		t.Fatalf("ids = %d, want 3", len(state.ids))
	}
	if err := recordValueForTest(&state, IdentityValue{IDRefs: "a\u00a0b"}, StartContext{Path: "/refs", Line: 4, Column: 5}); err != nil {
		t.Fatalf("RecordValue(IDRefs) error = %v", err)
	}
	var got error
	if err := state.CheckIDRefs(func(err error) error {
		got = err
		return nil
	}); err != nil {
		t.Fatalf("CheckIDRefs() error = %v", err)
	}
	expectXSDMessage(t, got, "IDREF does not resolve: a\u00a0b")
}

func TestIdentityStateRecordsSimpleValueProjection(t *testing.T) {
	t.Parallel()

	var state IdentityState
	value := runtime.SimpleValue{
		IDs:       "id",
		IDRefs:    "ref",
		Canonical: "ignored",
		Identity:  "ignored",
		Type:      runtime.SimpleTypeID(3),
	}
	if err := recordSimpleValueForTest(&state, value, StartContext{Path: "/value", Line: 2, Column: 3}); err != nil {
		t.Fatalf("RecordSimpleValue() error = %v", err)
	}
	if got := state.ids["id"]; got != "/value" {
		t.Fatalf("ids[id] = %q, want /value", got)
	}
	var got error
	if err := state.CheckIDRefs(func(err error) error {
		got = err
		return nil
	}); err != nil {
		t.Fatalf("CheckIDRefs() error = %v", err)
	}
	expectXSDMessage(t, got, "IDREF does not resolve: ref")
}

func TestSimpleValueIdentityKey(t *testing.T) {
	t.Parallel()

	const booleanType runtime.SimpleTypeID = 7
	const canonicalTrue = "true"
	rt := simpleValuePrimitiveRuntimeStub{
		booleanType: runtime.PrimitiveBoolean,
	}
	tests := []struct {
		name  string
		value runtime.SimpleValue
		want  string
		ok    bool
	}{
		{
			name:  "precomputed identity",
			value: runtime.SimpleValue{Identity: "precomputed", Type: runtime.SimpleTypeID(99)},
			want:  "precomputed",
			ok:    true,
		},
		{
			name:  "untyped",
			value: runtime.SimpleValue{Canonical: "text", Type: runtime.NoSimpleType},
			want:  runtime.UntypedSimpleIdentityKey("text"),
			ok:    true,
		},
		{
			name:  "typed primitive",
			value: runtime.SimpleValue{Canonical: canonicalTrue, Type: booleanType},
			want:  runtime.SimpleIdentityKey(runtime.PrimitiveBoolean, canonicalTrue),
			ok:    true,
		},
		{
			name:  "invalid type",
			value: runtime.SimpleValue{Canonical: "x", Type: runtime.SimpleTypeID(99)},
			ok:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := SimpleValueIdentityKey(rt, tt.value)
			if ok != tt.ok {
				t.Fatalf("SimpleValueIdentityKey() ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("SimpleValueIdentityKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIdentityStateCaptureSimpleValueFields(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const id runtime.IdentityConstraintID = 1
	const booleanType runtime.SimpleTypeID = 7
	const canonicalTrue = "true"
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{id}, 1, "/root")
	state.StartSelection(0, 2, id, 1, StartContext{Path: "/root/flag", Line: 4, Column: 5})

	err := state.CaptureSimpleValueFields(
		simpleValuePrimitiveRuntimeStub{booleanType: runtime.PrimitiveBoolean},
		[]IdentityFieldMatch{{Selection: 0, Field: 0}},
		runtime.SimpleValue{Canonical: canonicalTrue, Type: booleanType},
		StartContext{Path: "/root/flag", Line: 6, Column: 7},
	)
	if err != nil {
		t.Fatalf("CaptureSimpleValueFields() error = %v", err)
	}
	want := runtime.SimpleIdentityKey(runtime.PrimitiveBoolean, canonicalTrue)
	if got := state.fieldValues[0].value; got != want {
		t.Fatalf("simple value field value = %q, want %q", got, want)
	}
}

func TestIdentityStateCaptureSimpleValueFieldsRejectsInvalidType(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const id runtime.IdentityConstraintID = 1
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{id}, 1, "/root")
	state.StartSelection(0, 2, id, 1, StartContext{Path: "/root/value", Line: 4, Column: 5})

	err := state.CaptureSimpleValueFields(
		simpleValuePrimitiveRuntimeStub{},
		[]IdentityFieldMatch{{Selection: 0, Field: 0}},
		runtime.SimpleValue{Canonical: "x", Type: runtime.SimpleTypeID(99)},
		StartContext{Path: "/root/value", Line: 6, Column: 7},
	)
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
	expectXSDMessage(t, err, "identity field value references invalid simple type")
}

func TestIdentityStateRecordAttributeSimpleValueRejectsSecondID(t *testing.T) {
	t.Parallel()

	var state IdentityState
	seenID := true
	err := recordAttributeSimpleValueForTest(&state, runtime.SimpleValue{IDs: "second"}, &seenID, StartContext{Path: "/attr", Line: 2, Column: 3})
	expectXSDCode(t, err, xsderrors.CodeValidationType)
	expectXSDMessage(t, err, "multiple ID attributes")
	if state.entries != 0 {
		t.Fatalf("entries = %d, want 0", state.entries)
	}
}

func TestIdentityStateLimitFailuresDoNotAppendOrInsert(t *testing.T) {
	t.Parallel()

	var state IdentityState
	if err := recordValueWithLimitsForTest(&state, IdentityValue{IDs: "a"}, IdentityLimits{Entries: 1}, StartContext{Path: "/id", Line: 2, Column: 3}); err != nil {
		t.Fatalf("RecordValue(ID) error = %v", err)
	}
	err := recordValueWithLimitsForTest(&state, IdentityValue{IDRefs: "b"}, IdentityLimits{Entries: 1}, StartContext{Path: "/ref", Line: 4, Column: 5})
	expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
	if len(state.idrefs) != 0 {
		t.Fatalf("idrefs = %d, want 0 after failed reserve", len(state.idrefs))
	}
	if state.entries != 1 {
		t.Fatalf("entries = %d, want 1", state.entries)
	}

	var tooLong IdentityState
	err = recordValueWithLimitsForTest(&tooLong, IdentityValue{IDs: "abcd"}, IdentityLimits{TupleBytes: 3}, StartContext{Path: "/id", Line: 2, Column: 3})
	expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
	if len(tooLong.ids) != 0 || tooLong.entries != 0 {
		t.Fatalf("tuple limit mutated state: ids=%d entries=%d", len(tooLong.ids), tooLong.entries)
	}
}

func TestIdentityStateResetClearsAndDropsOversizedState(t *testing.T) {
	t.Parallel()

	constraints, ok := runtime.ElementIdentityConstraintIDs([][]runtime.IdentityConstraintID{{1}}, 0)
	if !ok {
		t.Fatal("ElementIdentityConstraintIDs() rejected test fixture")
	}
	state := IdentityState{
		ids: map[string]string{"a": "/id"},
		idrefs: append(make([]identityRef, 0, 2),
			identityRef{Value: "a"},
		),
		scopes: append(make([]identityScope, 0, 2),
			identityScope{
				tables: map[runtime.IdentityConstraintID]map[string]identityTableEntry{
					1: {"a": {path: "/id"}},
				},
				constraints: constraints,
				refs:        []identityTupleRef{{key: "a"}},
			},
		),
		selections: append(make([]identitySelection, 0, 2),
			identitySelection{path: "/selected", fieldStart: 0, fieldLen: 1},
		),
		fieldValues: append(make([]identityFieldValue, 0, 2),
			identityFieldValue{value: "a", present: true},
		),
		matches: append(make([]IdentityFieldMatch, 0, 2),
			IdentityFieldMatch{Selection: 1, Field: 2},
		),
		entries:    3,
		nextNodeID: 4,
	}
	state.Reset(1, 2)
	if state.ids == nil {
		t.Fatalf("Reset() dropped bounded ID map")
	}
	if len(state.ids) != 0 ||
		len(state.idrefs) != 0 ||
		len(state.scopes) != 0 ||
		len(state.selections) != 0 ||
		len(state.fieldValues) != 0 ||
		len(state.matches) != 0 ||
		state.entries != 0 ||
		state.nextNodeID != 0 {
		t.Fatalf(
			"Reset() retained state: ids=%d idrefs=%d scopes=%d selections=%d fields=%d matches=%d entries=%d nextNodeID=%d",
			len(state.ids),
			len(state.idrefs),
			len(state.scopes),
			len(state.selections),
			len(state.fieldValues),
			len(state.matches),
			state.entries,
			state.nextNodeID,
		)
	}
	if got := state.scopes[:cap(state.scopes)][0]; got.tables != nil || got.constraints.Len() != 0 || got.refs != nil {
		t.Fatalf("Reset() retained scoped identity references: %+v", got)
	}
	if got := state.fieldValues[:cap(state.fieldValues)][0]; got.present || got.value != "" {
		t.Fatalf("Reset() retained field value: %+v", got)
	}

	state.ids = map[string]string{"a": "/a", "b": "/b"}
	state.idrefs = append(make([]identityRef, 0, 3), identityRef{Value: "a"})
	state.scopes = append(make([]identityScope, 0, 3), identityScope{})
	state.selections = append(make([]identitySelection, 0, 3), identitySelection{})
	state.fieldValues = append(make([]identityFieldValue, 0, 3), identityFieldValue{})
	state.matches = append(make([]IdentityFieldMatch, 0, 3), IdentityFieldMatch{})
	state.entries = 2
	state.nextNodeID = 5
	state.Reset(1, 2)
	if state.ids != nil ||
		state.idrefs != nil ||
		state.scopes != nil ||
		state.selections != nil ||
		state.fieldValues != nil ||
		state.matches != nil ||
		state.entries != 0 ||
		state.nextNodeID != 0 {
		t.Fatalf(
			"Reset() retained oversized state: ids=%v idrefs=%v scopes=%v selections=%v fields=%v matches=%v entries=%d nextNodeID=%d",
			state.ids,
			state.idrefs,
			state.scopes,
			state.selections,
			state.fieldValues,
			state.matches,
			state.entries,
			state.nextNodeID,
		)
	}
}

func TestIdentityStateStartScopeEnforcesLimit(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const (
		elem runtime.ElementID            = 0
		id   runtime.IdentityConstraintID = 1
	)
	constraints, ok := runtime.ElementIdentityConstraintIDs([][]runtime.IdentityConstraintID{{id}}, elem)
	if !ok {
		t.Fatal("ElementIdentityConstraintIDs() rejected test fixture")
	}
	rt := identityRuntimeStub{elements: map[runtime.ElementID]runtime.IdentityConstraintIDs{elem: constraints}}
	ctx := StartContext{Path: "/root", Line: 2, Column: 3}
	if err := state.StartElementScope(rt, elem, 1, 1, ctx); err != nil {
		t.Fatalf("StartElementScope(first) error = %v", err)
	}
	err := state.StartElementScope(rt, elem, 2, 1, ctx)
	expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, err, "identity scope limit exceeded")
	if len(state.scopes) != 1 {
		t.Fatalf("scopes = %d, want 1", len(state.scopes))
	}
}

func TestIdentityStateCaptureFieldsRejectsInvalidAndDuplicateMatches(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const id runtime.IdentityConstraintID = 1
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{id}, 1, "/root")
	state.StartSelection(0, 2, id, 1, StartContext{Path: "/row", Line: 4, Column: 5})

	err := state.CaptureFields([]IdentityFieldMatch{{Selection: 1, Field: 0}}, "a", StartContext{Path: "/row/id", Line: 6, Column: 7})
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)

	err = state.CaptureFields([]IdentityFieldMatch{{Selection: 0, Field: 0}}, "a", StartContext{Path: "/row/id", Line: 6, Column: 7})
	if err != nil {
		t.Fatalf("CaptureFields(first) error = %v", err)
	}
	err = state.CaptureFields([]IdentityFieldMatch{{Selection: 0, Field: 0}}, "b", StartContext{Path: "/row/id", Line: 8, Column: 9})
	expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, err, "identity field selects multiple values")
	expectXSDLocation(t, err, "/row", 8, 9)
}

func TestIdentityStateCaptureComplexElementFields(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const id runtime.IdentityConstraintID = 1
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{id}, 1, "/root")
	state.StartSelection(0, 2, id, 1, StartContext{Path: "/root/item", Line: 4, Column: 5})

	err := state.CaptureComplexElementFields(
		[]IdentityFieldMatch{{Selection: 0, Field: 0}},
		[]byte(" a\t b "),
		StartContext{Path: "/root/item", Line: 6, Column: 7},
	)
	if err != nil {
		t.Fatalf("CaptureComplexElementFields() error = %v", err)
	}
	want := runtime.SimpleIdentityKey(runtime.PrimitiveString, "a b")
	if got := state.fieldValues[0].value; got != want {
		t.Fatalf("complex element field value = %q, want %q", got, want)
	}
}

func TestNilledElementIdentityKeyIsStableAndDistinct(t *testing.T) {
	t.Parallel()

	got := NilledElementIdentityKey()
	if got != "\xff\x1e\x00nil" {
		t.Fatalf("NilledElementIdentityKey() = %q", got)
	}
	if got == runtime.SimpleIdentityKey(runtime.PrimitiveString, "nil") {
		t.Fatal("nilled element identity key collided with string nil")
	}
}

func TestEndIdentityCapture(t *testing.T) {
	t.Parallel()

	const elem runtime.ElementID = 1
	typ := runtime.ComplexRef(1)
	tests := []struct {
		name          string
		in            EndIdentityInput
		simpleContent bool
		want          EndIdentityCaptureAction
	}{
		{
			name: "simple content already captured",
			in: EndIdentityInput{
				Type:            typ,
				Element:         elem,
				ContentCaptured: true,
				Nilled:          true,
			},
			want: EndIdentityCaptureNone,
		},
		{
			name: "nilled declared element",
			in: EndIdentityInput{
				Type:    typ,
				Element: elem,
				Nilled:  true,
			},
			want: EndIdentityCaptureNilledElement,
		},
		{
			name: "nilled undeclared element without simple content",
			in: EndIdentityInput{
				Type:    typ,
				Element: runtime.NoElement,
				Nilled:  true,
			},
			want: EndIdentityCaptureComplexElement,
		},
		{
			name: "complex element",
			in: EndIdentityInput{
				Type:    typ,
				Element: elem,
			},
			want: EndIdentityCaptureComplexElement,
		},
		{
			name: "simple element without captured field",
			in: EndIdentityInput{
				Type:    typ,
				Element: elem,
			},
			simpleContent: true,
			want:          EndIdentityCaptureNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := EndIdentityCapture(endIdentityRuntimeStub{simpleContent: tt.simpleContent, ok: true}, tt.in)
			if err != nil {
				t.Fatalf("EndIdentityCapture() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("EndIdentityCapture() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEndIdentityCaptureRejectsMissingContentMetadata(t *testing.T) {
	t.Parallel()

	_, err := EndIdentityCapture(endIdentityRuntimeStub{}, EndIdentityInput{
		Type:    runtime.ComplexRef(1),
		Element: 1,
	})
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
}

func TestEndIdentityCaptureRejectsMissingContentMetadataBeforeNilled(t *testing.T) {
	t.Parallel()

	_, err := EndIdentityCapture(endIdentityRuntimeStub{}, EndIdentityInput{
		Type:    runtime.ComplexRef(1),
		Element: 1,
		Nilled:  true,
	})
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
}

type endIdentityRuntimeStub struct {
	simpleContent bool
	ok            bool
}

func (s endIdentityRuntimeStub) ElementHasSimpleContent(runtime.TypeID, runtime.ElementID) (bool, bool) {
	return s.simpleContent, s.ok
}

func TestIdentityStateCaptureComplexElementFieldsRejectsEmptyValue(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const id runtime.IdentityConstraintID = 1
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{id}, 1, "/root")
	state.StartSelection(0, 2, id, 1, StartContext{Path: "/root/item", Line: 4, Column: 5})

	err := state.CaptureComplexElementFields(
		[]IdentityFieldMatch{{Selection: 0, Field: 0}},
		[]byte(" \t\r\n"),
		StartContext{Path: "/root/item", Line: 6, Column: 7},
	)
	expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, err, "identity field has no simple value")
	expectXSDLocation(t, err, "/root/item", 6, 7)
	if state.fieldValues[0].present {
		t.Fatalf("field present after failed complex capture")
	}
}

func TestIdentityStateFinishSelectionsReportsMissingKeyField(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const keyID runtime.IdentityConstraintID = 1
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 1, "/root")
	state.StartSelection(0, 2, keyID, 1, StartContext{Path: "/row", Line: 4, Column: 5})

	var got error
	err := finishSelectionsForTest(&state, identityInfo(map[runtime.IdentityConstraintID]runtime.IdentityConstraintInfo{
		keyID: {Kind: runtime.IdentityKey},
	}), 2, StartContext{Path: "/row", Line: 6, Column: 7}, func(err error) error {
		got = err
		return nil
	})
	if err != nil {
		t.Fatalf("FinishSelections() error = %v", err)
	}
	expectXSDCode(t, got, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, got, "key field is missing")
	expectXSDLocation(t, got, "/row", 6, 7)
	if len(state.selections) != 0 || len(state.fieldValues) != 0 {
		t.Fatalf("unfinished selection state: selections=%d fields=%d", len(state.selections), len(state.fieldValues))
	}
}

func TestIdentityStateFinishSelectionsRejectsDuplicateKeyWithoutSecondReserve(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const keyID runtime.IdentityConstraintID = 1
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 1, "/root")
	state.StartSelection(0, 2, keyID, 1, StartContext{Path: "/first", Line: 4, Column: 5})
	state.StartSelection(0, 2, keyID, 1, StartContext{Path: "/second", Line: 6, Column: 7})
	captureIdentityField(t, &state, 0, "a")
	captureIdentityField(t, &state, 1, "a")

	var got error
	err := finishSelectionsForTest(&state, identityInfo(map[runtime.IdentityConstraintID]runtime.IdentityConstraintInfo{
		keyID: {Kind: runtime.IdentityKey},
	}), 2, StartContext{Path: "/second", Line: 8, Column: 9}, func(err error) error {
		got = err
		return nil
	})
	if err != nil {
		t.Fatalf("FinishSelections() error = %v", err)
	}
	expectXSDCode(t, got, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, got, "duplicate identity value first seen at /first")
	expectXSDLocation(t, got, "/second", 8, 9)
	if state.entries != 1 {
		t.Fatalf("entries = %d, want 1", state.entries)
	}
}

func TestIdentityStateCloseScopesResolvesKeyRefWithinScope(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const (
		keyID runtime.IdentityConstraintID = 1
		refID runtime.IdentityConstraintID = 2
	)
	info := identityInfo(map[runtime.IdentityConstraintID]runtime.IdentityConstraintInfo{
		keyID: {Kind: runtime.IdentityKey},
		refID: {Kind: runtime.IdentityKeyRef, Refer: keyID},
	})
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID, refID}, 1, "/root")
	state.StartSelection(0, 2, keyID, 1, StartContext{Path: "/key", Line: 4, Column: 5})
	state.StartSelection(0, 2, refID, 1, StartContext{Path: "/ref", Line: 6, Column: 7})
	captureIdentityField(t, &state, 0, "a")
	captureIdentityField(t, &state, 1, "a")
	if err := finishSelectionsForTest(&state, info, 2, StartContext{Path: "/root", Line: 8, Column: 9}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections() error = %v", err)
	}
	err := state.CloseScopes(1, failIdentityReport(t))
	if err != nil {
		t.Fatalf("CloseScopes() error = %v", err)
	}
}

func TestIdentityStateCloseScopesReportsUnresolvedKeyRef(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const (
		keyID runtime.IdentityConstraintID = 1
		refID runtime.IdentityConstraintID = 2
	)
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{refID}, 1, "/root")
	state.StartSelection(0, 2, refID, 1, StartContext{Path: "/ref", Line: 4, Column: 5})
	captureIdentityField(t, &state, 0, "missing")
	if err := finishSelectionsForTest(&state, identityInfo(map[runtime.IdentityConstraintID]runtime.IdentityConstraintInfo{
		refID: {Kind: runtime.IdentityKeyRef, Refer: keyID},
	}), 2, StartContext{Path: "/ref", Line: 6, Column: 7}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections() error = %v", err)
	}

	var got error
	err := state.CloseScopes(1, func(err error) error {
		got = err
		return nil
	})
	if err != nil {
		t.Fatalf("CloseScopes() error = %v", err)
	}
	expectXSDCode(t, got, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, got, "keyref does not resolve")
	expectXSDLocation(t, got, "/ref", 4, 5)
}

func TestIdentityStateMergedChildKeyConflictKeepsParentKeyRefUnresolved(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const (
		keyID runtime.IdentityConstraintID = 1
		refID runtime.IdentityConstraintID = 2
	)
	info := identityInfo(map[runtime.IdentityConstraintID]runtime.IdentityConstraintInfo{
		keyID: {Kind: runtime.IdentityKey},
		refID: {Kind: runtime.IdentityKeyRef, Refer: keyID},
	})
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{refID}, 1, "/root")

	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 2, "/root/a")
	state.StartSelection(1, 3, keyID, 1, StartContext{Path: "/root/a/id", Line: 4, Column: 5})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/a", Line: 6, Column: 7}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(first child) error = %v", err)
	}
	if err := state.CloseScopes(2, failIdentityReport(t)); err != nil {
		t.Fatalf("CloseScopes(first child) error = %v", err)
	}

	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 2, "/root/b")
	state.StartSelection(1, 3, keyID, 1, StartContext{Path: "/root/b/id", Line: 8, Column: 9})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/b", Line: 10, Column: 11}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(second child) error = %v", err)
	}
	if err := state.CloseScopes(2, failIdentityReport(t)); err != nil {
		t.Fatalf("CloseScopes(second child) error = %v", err)
	}

	state.StartSelection(0, 3, refID, 1, StartContext{Path: "/root/ref", Line: 12, Column: 13})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/ref", Line: 14, Column: 15}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(ref) error = %v", err)
	}

	var got error
	if err := state.CloseScopes(1, func(err error) error {
		got = err
		return nil
	}); err != nil {
		t.Fatalf("CloseScopes(parent) error = %v", err)
	}
	expectXSDCode(t, got, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, got, "keyref does not resolve")
	expectXSDLocation(t, got, "/root/ref", 12, 13)
}

func TestIdentityStateMergedChildKeyConflictUsesSelectedNodeNotPath(t *testing.T) {
	t.Parallel()

	var state IdentityState
	const (
		keyID runtime.IdentityConstraintID = 1
		refID runtime.IdentityConstraintID = 2
	)
	info := identityInfo(map[runtime.IdentityConstraintID]runtime.IdentityConstraintInfo{
		keyID: {Kind: runtime.IdentityKey},
		refID: {Kind: runtime.IdentityKeyRef, Refer: keyID},
	})
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{refID}, 1, "/root")

	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 2, "/root/group")
	state.StartSelection(1, 3, keyID, 1, StartContext{Path: "/root/group/id", Line: 4, Column: 5})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/group", Line: 6, Column: 7}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(first child) error = %v", err)
	}
	if err := state.CloseScopes(2, failIdentityReport(t)); err != nil {
		t.Fatalf("CloseScopes(first child) error = %v", err)
	}

	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 2, "/root/group")
	state.StartSelection(1, 3, keyID, 1, StartContext{Path: "/root/group/id", Line: 8, Column: 9})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/group", Line: 10, Column: 11}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(second child) error = %v", err)
	}
	if err := state.CloseScopes(2, failIdentityReport(t)); err != nil {
		t.Fatalf("CloseScopes(second child) error = %v", err)
	}

	state.StartSelection(0, 3, refID, 1, StartContext{Path: "/root/ref", Line: 12, Column: 13})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/ref", Line: 14, Column: 15}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(ref) error = %v", err)
	}

	var got error
	if err := state.CloseScopes(1, func(err error) error {
		got = err
		return nil
	}); err != nil {
		t.Fatalf("CloseScopes(parent) error = %v", err)
	}
	expectXSDCode(t, got, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, got, "keyref does not resolve")
	expectXSDLocation(t, got, "/root/ref", 12, 13)
}

type identityInfoRuntime func(runtime.IdentityConstraintID) runtime.IdentityConstraintInfo

func (rt identityInfoRuntime) IdentityConstraintInfo(id runtime.IdentityConstraintID) (runtime.IdentityConstraintInfo, bool) {
	return rt(id), true
}

func identityInfo(in map[runtime.IdentityConstraintID]runtime.IdentityConstraintInfo) identityInfoRuntime {
	return identityInfoRuntime(func(id runtime.IdentityConstraintID) runtime.IdentityConstraintInfo {
		return in[id]
	})
}

func recordAttributeSimpleValueForTest(s *IdentityState, value runtime.SimpleValue, seenID *bool, ctx StartContext) error {
	if value.IDs != "" {
		if seenID != nil && *seenID {
			return validation(ctx, xsderrors.CodeValidationType, "multiple ID attributes")
		}
		if seenID != nil {
			*seenID = true
		}
	}
	return recordValueForTest(s, SimpleValueIdentity(value), ctx)
}

func recordSimpleValueForTest(s *IdentityState, value runtime.SimpleValue, ctx StartContext) error {
	return recordValueForTest(s, SimpleValueIdentity(value), ctx)
}

func recordValueForTest(s *IdentityState, value IdentityValue, ctx StartContext) error {
	return recordValueWithLimitsForTest(s, value, IdentityLimits{}, ctx)
}

func recordValueWithLimitsForTest(s *IdentityState, value IdentityValue, limits IdentityLimits, ctx StartContext) error {
	if value.IDs == "" && value.IDRefs == "" {
		return nil
	}
	path := ctx.PathString()
	for canonical := range lex.XMLFieldsSeq(value.IDs) {
		if s.ids == nil {
			s.ids = make(map[string]string)
		}
		if prev, exists := s.ids[canonical]; exists {
			return validation(ctx, xsderrors.CodeValidationType, "duplicate ID "+canonical+" first seen at "+prev)
		}
		if err := s.ReserveEntry(canonical, limits, ctx); err != nil {
			return err
		}
		s.ids[canonical] = path
	}
	for canonical := range lex.XMLFieldsSeq(value.IDRefs) {
		if err := s.ReserveEntry(canonical, limits, ctx); err != nil {
			return err
		}
		s.idrefs = append(s.idrefs, identityRef{Value: canonical, Path: path, Line: ctx.Line, Col: ctx.Column})
	}
	return nil
}

func finishSelectionsForTest(
	s *IdentityState,
	rt IdentityConstraintRuntime,
	depth int,
	ctx StartContext,
	report func(error) error,
) error {
	if s == nil || len(s.selections) == 0 {
		return nil
	}
	orig := s.selections
	dst := s.selections[:0]
	for i := range s.selections {
		sel := s.selections[i]
		if sel.depth != depth {
			dst = append(dst, sel)
			continue
		}
		if err := s.finishSelection(rt, sel, IdentityLimits{}, ctx); err != nil {
			clear(s.selectionFields(sel))
			if recoverErr := report(err); recoverErr != nil {
				dst = append(dst, orig[i+1:]...)
				clear(orig[len(dst):])
				s.selections = dst
				s.truncateFieldValues()
				return recoverErr
			}
			continue
		}
		clear(s.selectionFields(sel))
	}
	clear(orig[len(dst):])
	s.selections = dst
	s.truncateFieldValues()
	return nil
}

func startIdentityScope(t *testing.T, state *IdentityState, constraints []runtime.IdentityConstraintID, depth int, path string) {
	t.Helper()
	const elem runtime.ElementID = 0
	constraintIDs, ok := runtime.ElementIdentityConstraintIDs([][]runtime.IdentityConstraintID{constraints}, elem)
	if !ok {
		t.Fatal("ElementIdentityConstraintIDs() rejected test fixture")
	}
	rt := identityRuntimeStub{elements: map[runtime.ElementID]runtime.IdentityConstraintIDs{elem: constraintIDs}}
	err := state.StartElementScope(rt, elem, depth, 0, StartContext{Path: path})
	if err != nil {
		t.Fatalf("StartElementScope(depth=%d) error = %v", depth, err)
	}
}

func captureIdentityField(t *testing.T, state *IdentityState, selection int, value string) {
	t.Helper()
	err := state.CaptureFields([]IdentityFieldMatch{{Selection: selection, Field: 0}}, value, StartContext{Path: "/field", Line: 1, Column: 1})
	if err != nil {
		t.Fatalf("CaptureFields(selection=%d) error = %v", selection, err)
	}
}

type simpleValuePrimitiveRuntimeStub map[runtime.SimpleTypeID]runtime.PrimitiveKind

func (r simpleValuePrimitiveRuntimeStub) SimpleTypePrimitive(id runtime.SimpleTypeID) (runtime.PrimitiveKind, bool) {
	primitive, ok := r[id]
	return primitive, ok
}

func failIdentityReport(t *testing.T) func(error) error {
	t.Helper()
	return func(err error) error {
		t.Fatalf("unexpected identity error: %v", err)
		return nil
	}
}

func expectXSDLocation(t *testing.T, err error, path string, line, col int) {
	t.Helper()
	var x *xsderrors.Error
	if !errors.As(err, &x) {
		t.Fatalf("error = %v, want *xsderrors.Error", err)
	}
	if x.Path != path || x.Line != line || x.Column != col {
		t.Fatalf("error location = %s %d:%d, want %s %d:%d", x.Path, x.Line, x.Column, path, line, col)
	}
}
