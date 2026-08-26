package validate

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// startSelectionForTest starts an unbounded selection for white-box state tests.
func (s *identityState) startSelectionForTest(scope, depth int, constraint runtime.IdentityConstraintID, fieldCount int, ctx StartContext) {
	if err := s.startSelection(scope, depth, constraint, fieldCount, 0, ctx); err != nil {
		panic(err)
	}
}

func TestIdentitySelectionLimitPrecedesPendingStateAllocation(t *testing.T) {
	var state identityState
	state.startSelectionForTest(0, 1, 0, 2, StartContext{Path: "/root/a", Line: 2, Column: 3})
	wantSelections := len(state.selections)
	wantFields := len(state.fieldValues)
	wantNodeID := state.nextNodeID

	err := state.startSelection(0, 2, 0, 3, 1, StartContext{Path: "/root/a/a", Line: 4, Column: 5})
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)
	if len(state.selections) != wantSelections || len(state.fieldValues) != wantFields || state.nextNodeID != wantNodeID {
		t.Fatalf(
			"denied selection mutated state: selections=%d fields=%d node=%d, want %d/%d/%d",
			len(state.selections),
			len(state.fieldValues),
			state.nextNodeID,
			wantSelections,
			wantFields,
			wantNodeID,
		)
	}
}

func TestIdentityStateRejectsDuplicateIDWithoutMutating(t *testing.T) {
	t.Parallel()

	var evaluation identityEvaluation
	ctx := StartContext{Path: "/first", Line: 2, Column: 3}
	if err := evaluation.recordIdentityFields("a", "", ctx); err != nil {
		t.Fatalf("recordValue(first) error = %v", err)
	}
	err := evaluation.recordIdentityFields("a", "", StartContext{Path: "/second", Line: 4, Column: 5})
	expectXSDCode(t, err, xsderrors.CodeValidationType)
	expectXSDMessage(t, err, "duplicate ID a first seen at /first")
	if evaluation.entries != 1 {
		t.Fatalf("entries = %d, want 1", evaluation.entries)
	}
	if got := evaluation.ids["a"]; got != "/first" {
		t.Fatalf("ids[a] = %q, want /first", got)
	}
}

func TestIdentityStateResolvesIDREFAgainstLaterID(t *testing.T) {
	t.Parallel()

	var evaluation identityEvaluation
	if err := evaluation.recordIdentityFields("", "a", StartContext{Path: "/ref", Line: 2, Column: 3}); err != nil {
		t.Fatalf("recordValue(IDREF) error = %v", err)
	}
	if err := evaluation.recordIdentityFields("a", "", StartContext{Path: "/id", Line: 4, Column: 5}); err != nil {
		t.Fatalf("recordValue(ID) error = %v", err)
	}
	err := evaluation.endDocument(func(err error) error {
		t.Fatalf("checkIDRefs reported resolved ref: %v", err)
		return nil
	})

	if err != nil {
		t.Fatalf("checkIDRefs() error = %v", err)
	}
}

func TestIdentityStateReportsMissingIDREFAtOriginalLocation(t *testing.T) {
	t.Parallel()

	var evaluation identityEvaluation
	if err := evaluation.recordIdentityFields("", "missing", StartContext{Path: "/ref", Line: 2, Column: 3}); err != nil {
		t.Fatalf("recordValue(IDREF) error = %v", err)
	}
	var got error
	err := evaluation.endDocument(func(err error) error {
		got = err
		return nil
	})

	if err != nil {
		t.Fatalf("checkIDRefs() error = %v", err)
	}
	expectXSDCode(t, got, xsderrors.CodeValidationType)
	expectXSDMessage(t, got, "IDREF does not resolve: missing")
	expectXSDLocation(t, got, "/ref", 2, 3)
}

func TestIdentityStateUsesXMLWhitespaceFields(t *testing.T) {
	t.Parallel()

	var evaluation identityEvaluation
	if err := evaluation.recordIdentityFields("a\tb\nc", "", StartContext{Path: "/ids", Line: 2, Column: 3}); err != nil {
		t.Fatalf("recordValue(IDs) error = %v", err)
	}
	if len(evaluation.ids) != 3 {
		t.Fatalf("ids = %d, want 3", len(evaluation.ids))
	}
	if err := evaluation.recordIdentityFields("", "a\u00a0b", StartContext{Path: "/refs", Line: 4, Column: 5}); err != nil {
		t.Fatalf("recordValue(IDRefs) error = %v", err)
	}
	var got error
	if err := evaluation.endDocument(func(err error) error {
		got = err
		return nil
	}); err != nil {
		t.Fatalf("checkIDRefs() error = %v", err)
	}
	expectXSDMessage(t, got, "IDREF does not resolve: a\u00a0b")
}

func TestSimpleValueIdentityKey(t *testing.T) {
	t.Parallel()

	const canonicalTrue = "true"
	rt, booleanType := booleanRuntimeForTest(t)
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

			got, ok := simpleValueIdentityKey(rt, tt.value)
			if ok != tt.ok {
				t.Fatalf("simpleValueIdentityKey() ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("simpleValueIdentityKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIdentityStateLimitFailuresDoNotAppendOrInsert(t *testing.T) {
	t.Parallel()

	evaluation := identityEvaluation{limits: identityLimits{Entries: 1}}
	if err := evaluation.recordIdentityFields("a", "", StartContext{Path: "/id", Line: 2, Column: 3}); err != nil {
		t.Fatalf("recordValue(ID) error = %v", err)
	}
	err := evaluation.recordIdentityFields("", "b", StartContext{Path: "/ref", Line: 4, Column: 5})
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)
	if len(evaluation.idrefs) != 0 {
		t.Fatalf("idrefs = %d, want 0 after failed reserve", len(evaluation.idrefs))
	}
	if evaluation.entries != 1 {
		t.Fatalf("entries = %d, want 1", evaluation.entries)
	}

	tooLong := identityEvaluation{limits: identityLimits{TupleBytes: 3}}
	err = tooLong.recordIdentityFields("abcd", "", StartContext{Path: "/id", Line: 2, Column: 3})
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)
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
	state := identityState{
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
			identityFieldValue{value: "a", state: identityFieldPresent},
		),
		matches: append(make([]identityFieldMatch, 0, 2),
			identityFieldMatch{Selection: 1, Field: 2},
		),
		entries:    3,
		nextNodeID: 4,
	}
	state.reset(1, 2)
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
	if got := state.fieldValues[:cap(state.fieldValues)][0]; got.state != identityFieldAbsent || got.value != "" {
		t.Fatalf("Reset() retained field value: %+v", got)
	}

	state.ids = map[string]string{"a": "/a", "b": "/b"}
	state.idrefs = append(make([]identityRef, 0, 3), identityRef{Value: "a"})
	state.scopes = append(make([]identityScope, 0, 3), identityScope{})
	state.selections = append(make([]identitySelection, 0, 3), identitySelection{})
	state.fieldValues = append(make([]identityFieldValue, 0, 3), identityFieldValue{})
	state.matches = append(make([]identityFieldMatch, 0, 3), identityFieldMatch{})
	state.entries = 2
	state.nextNodeID = 5
	state.reset(1, 2)
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

	var state identityState
	const id runtime.IdentityConstraintID = 1
	constraints, ok := runtime.ElementIdentityConstraintIDs([][]runtime.IdentityConstraintID{{id}}, 0)
	if !ok {
		t.Fatal("ElementIdentityConstraintIDs() rejected test fixture")
	}
	ctx := StartContext{Path: "/root", Line: 2, Column: 3}
	if err := state.startScope(constraints, 1, 1, ctx); err != nil {
		t.Fatalf("startScope(first) error = %v", err)
	}
	err := state.startScope(constraints, 2, 1, ctx)
	expectXSDCode(t, err, xsderrors.CodeValidationLimit)
	expectXSDMessage(t, err, "identity scope limit exceeded")
	if len(state.scopes) != 1 {
		t.Fatalf("scopes = %d, want 1", len(state.scopes))
	}
}

func TestIdentityStateCaptureFieldsRejectsInvalidAndDuplicateMatches(t *testing.T) {
	t.Parallel()

	var state identityState
	const id runtime.IdentityConstraintID = 1
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{id}, 1, "/root")
	state.startSelectionForTest(0, 2, id, 1, StartContext{Path: "/row", Line: 4, Column: 5})

	err := state.captureFields([]identityFieldMatch{{Selection: 1, Field: 0}}, "a", StartContext{Path: "/row/id", Line: 6, Column: 7})
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)

	err = state.captureFields([]identityFieldMatch{{Selection: 0, Field: 0}}, "a", StartContext{Path: "/row/id", Line: 6, Column: 7})
	if err != nil {
		t.Fatalf("captureFields(first) error = %v", err)
	}
	err = state.captureFields([]identityFieldMatch{{Selection: 0, Field: 0}}, "b", StartContext{Path: "/row/id", Line: 8, Column: 9})
	expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, err, "identity field selects multiple values")
	expectXSDLocation(t, err, "/row", 8, 9)
	if state.fieldValues[0].state != identityFieldInvalid {
		t.Fatal("duplicate field match did not invalidate field")
	}
	if !state.scopes[0].invalid {
		t.Fatal("duplicate field match did not invalidate its owning scope")
	}
	if err := state.finishSelectionWithConstraint(
		runtime.IdentityUnique,
		runtime.NoIdentityConstraint,
		state.selections[0],
		identityLimits{},
		StartContext{Path: "/row", Line: 10, Column: 11},
	); err != nil {
		t.Fatalf("finishSelectionWithInfo(invalid duplicate) error = %v", err)
	}
	if state.scopes[0].tables != nil {
		t.Fatal("invalid duplicate field published an identity tuple")
	}
}

func TestIdentityStateRejectFieldsWithoutSimpleValueInvalidatesField(t *testing.T) {
	t.Parallel()

	var state identityState
	const id runtime.IdentityConstraintID = 1
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{id}, 1, "/root")
	state.startSelectionForTest(0, 2, id, 1, StartContext{Path: "/root/item", Line: 4, Column: 5})

	err := state.rejectFieldsWithoutSimpleValue(
		[]identityFieldMatch{{Selection: 0, Field: 0}},
		StartContext{Path: "/root/item", Line: 6, Column: 7},
	)
	expectXSDCode(t, err, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, err, "identity field has no simple value")
	expectXSDLocation(t, err, "/root/item", 6, 7)
	if state.fieldValues[0].state != identityFieldInvalid {
		t.Fatal("field not invalid after rejected field node")
	}
	if !state.scopes[0].invalid {
		t.Fatal("rejected field node did not invalidate its owning scope")
	}
	if err := state.finishSelectionWithConstraint(
		runtime.IdentityKey,
		runtime.NoIdentityConstraint,
		state.selections[0],
		identityLimits{},
		StartContext{Path: "/root/item", Line: 8, Column: 9},
	); err != nil {
		t.Fatalf("finishSelectionWithInfo(invalid key field) error = %v", err)
	}
}

func TestNilledElementIdentityKeyIsStableAndDistinct(t *testing.T) {
	t.Parallel()

	got := nilledElementIdentityKey
	if got != "\xff\x1e\x00nil" {
		t.Fatalf("nilledElementIdentityKeyForTest() = %q", got)
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
		in            endIdentityInput
		simpleContent bool
		want          endIdentityCaptureAction
	}{
		{
			name: "simple content already captured",
			in: endIdentityInput{
				Type:            typ,
				Element:         elem,
				ContentCaptured: true,
				Nilled:          true,
			},
			want: endIdentityCaptureNone,
		},
		{
			name: "nilled declared element with simple content",
			in: endIdentityInput{
				Type:    typ,
				Element: elem,
				Nilled:  true,
			},
			simpleContent: true,
			want:          endIdentityCaptureNilledElement,
		},
		{
			name: "nilled declared element with complex content",
			in: endIdentityInput{
				Type:    typ,
				Element: elem,
				Nilled:  true,
			},
			want: endIdentityCaptureComplexElement,
		},
		{
			name: "nilled undeclared element without simple content",
			in: endIdentityInput{
				Type:    typ,
				Element: runtime.NoElement,
				Nilled:  true,
			},
			want: endIdentityCaptureComplexElement,
		},
		{
			name: "complex element",
			in: endIdentityInput{
				Type:    typ,
				Element: elem,
			},
			want: endIdentityCaptureComplexElement,
		},
		{
			name: "simple element without captured field",
			in: endIdentityInput{
				Type:    typ,
				Element: elem,
			},
			simpleContent: true,
			want:          endIdentityCaptureNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := endIdentityCapture(tt.simpleContent, tt.in)
			if tt.in.ContentCaptured {
				var err error
				got, err = endIdentityCaptureForElement(nil, tt.in)
				if err != nil {
					t.Fatalf("endIdentityCaptureForElement() error = %v", err)
				}
			}
			if got != tt.want {
				t.Fatalf("endIdentityCaptureForElement() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEndIdentityCaptureRejectsMissingContentMetadata(t *testing.T) {
	t.Parallel()

	_, err := endIdentityCaptureForElement(nil, endIdentityInput{
		Type:    runtime.ComplexRef(1),
		Element: 1,
	})
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
}

func TestIdentityStateFinishSelectionsReportsMissingKeyField(t *testing.T) {
	t.Parallel()

	var state identityState
	const keyID runtime.IdentityConstraintID = 1
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 1, "/root")
	state.startSelectionForTest(0, 2, keyID, 1, StartContext{Path: "/row", Line: 4, Column: 5})

	var got error
	err := finishSelectionsForTest(&state, identityInfo(map[runtime.IdentityConstraintID]identityConstraintInfoForTest{
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

	var state identityState
	const keyID runtime.IdentityConstraintID = 1
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 1, "/root")
	state.startSelectionForTest(0, 2, keyID, 1, StartContext{Path: "/first", Line: 4, Column: 5})
	state.startSelectionForTest(0, 2, keyID, 1, StartContext{Path: "/second", Line: 6, Column: 7})
	captureIdentityField(t, &state, 0, "a")
	captureIdentityField(t, &state, 1, "a")

	var got error
	err := finishSelectionsForTest(&state, identityInfo(map[runtime.IdentityConstraintID]identityConstraintInfoForTest{
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

	var state identityState
	const (
		keyID runtime.IdentityConstraintID = 1
		refID runtime.IdentityConstraintID = 2
	)
	info := identityInfo(map[runtime.IdentityConstraintID]identityConstraintInfoForTest{
		keyID: {Kind: runtime.IdentityKey},
		refID: {Kind: runtime.IdentityKeyRef, Refer: keyID},
	})
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID, refID}, 1, "/root")
	state.startSelectionForTest(0, 2, keyID, 1, StartContext{Path: "/key", Line: 4, Column: 5})
	state.startSelectionForTest(0, 2, refID, 1, StartContext{Path: "/ref", Line: 6, Column: 7})
	captureIdentityField(t, &state, 0, "a")
	captureIdentityField(t, &state, 1, "a")
	if err := finishSelectionsForTest(&state, info, 2, StartContext{Path: "/root", Line: 8, Column: 9}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections() error = %v", err)
	}
	invalid, err := state.closeScopes(1, failIdentityReport(t))
	if err != nil {
		t.Fatalf("closeScopes() error = %v", err)
	}
	if invalid {
		t.Fatal("closeScopes() invalid = true for resolved constraints")
	}
}

func TestIdentityStateCloseScopesReportsUnresolvedKeyRef(t *testing.T) {
	t.Parallel()

	var state identityState
	const (
		keyID runtime.IdentityConstraintID = 1
		refID runtime.IdentityConstraintID = 2
	)
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{refID}, 1, "/root")
	state.startSelectionForTest(0, 2, refID, 1, StartContext{Path: "/ref", Line: 4, Column: 5})
	captureIdentityField(t, &state, 0, "missing")
	if err := finishSelectionsForTest(&state, identityInfo(map[runtime.IdentityConstraintID]identityConstraintInfoForTest{
		refID: {Kind: runtime.IdentityKeyRef, Refer: keyID},
	}), 2, StartContext{Path: "/ref", Line: 6, Column: 7}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections() error = %v", err)
	}

	var got error
	invalid, err := state.closeScopes(1, func(err error) error {
		got = err
		return nil
	})
	if err != nil {
		t.Fatalf("closeScopes() error = %v", err)
	}
	if !invalid {
		t.Fatal("closeScopes() invalid = false, want true")
	}
	expectXSDCode(t, got, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, got, "keyref does not resolve")
	expectXSDLocation(t, got, "/ref", 4, 5)
}

func TestIdentityStateMergedChildKeyConflictKeepsParentKeyRefUnresolved(t *testing.T) {
	t.Parallel()

	var state identityState
	const (
		keyID runtime.IdentityConstraintID = 1
		refID runtime.IdentityConstraintID = 2
	)
	info := identityInfo(map[runtime.IdentityConstraintID]identityConstraintInfoForTest{
		keyID: {Kind: runtime.IdentityKey},
		refID: {Kind: runtime.IdentityKeyRef, Refer: keyID},
	})
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{refID}, 1, "/root")

	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 2, "/root/a")
	state.startSelectionForTest(1, 3, keyID, 1, StartContext{Path: "/root/a/id", Line: 4, Column: 5})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/a", Line: 6, Column: 7}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(first child) error = %v", err)
	}
	if _, err := state.closeScopes(2, failIdentityReport(t)); err != nil {
		t.Fatalf("closeScopes(first child) error = %v", err)
	}

	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 2, "/root/b")
	state.startSelectionForTest(1, 3, keyID, 1, StartContext{Path: "/root/b/id", Line: 8, Column: 9})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/b", Line: 10, Column: 11}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(second child) error = %v", err)
	}
	if _, err := state.closeScopes(2, failIdentityReport(t)); err != nil {
		t.Fatalf("closeScopes(second child) error = %v", err)
	}

	state.startSelectionForTest(0, 3, refID, 1, StartContext{Path: "/root/ref", Line: 12, Column: 13})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/ref", Line: 14, Column: 15}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(ref) error = %v", err)
	}

	var got error
	if _, err := state.closeScopes(1, func(err error) error {
		got = err
		return nil
	}); err != nil {
		t.Fatalf("closeScopes(parent) error = %v", err)
	}
	expectXSDCode(t, got, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, got, "keyref does not resolve")
	expectXSDLocation(t, got, "/root/ref", 12, 13)
}

func TestIdentityStateMergedChildKeyConflictUsesSelectedNodeNotPath(t *testing.T) {
	t.Parallel()

	var state identityState
	const (
		keyID runtime.IdentityConstraintID = 1
		refID runtime.IdentityConstraintID = 2
	)
	info := identityInfo(map[runtime.IdentityConstraintID]identityConstraintInfoForTest{
		keyID: {Kind: runtime.IdentityKey},
		refID: {Kind: runtime.IdentityKeyRef, Refer: keyID},
	})
	startIdentityScope(t, &state, []runtime.IdentityConstraintID{refID}, 1, "/root")

	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 2, "/root/group")
	state.startSelectionForTest(1, 3, keyID, 1, StartContext{Path: "/root/group/id", Line: 4, Column: 5})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/group", Line: 6, Column: 7}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(first child) error = %v", err)
	}
	if _, err := state.closeScopes(2, failIdentityReport(t)); err != nil {
		t.Fatalf("closeScopes(first child) error = %v", err)
	}

	startIdentityScope(t, &state, []runtime.IdentityConstraintID{keyID}, 2, "/root/group")
	state.startSelectionForTest(1, 3, keyID, 1, StartContext{Path: "/root/group/id", Line: 8, Column: 9})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/group", Line: 10, Column: 11}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(second child) error = %v", err)
	}
	if _, err := state.closeScopes(2, failIdentityReport(t)); err != nil {
		t.Fatalf("closeScopes(second child) error = %v", err)
	}

	state.startSelectionForTest(0, 3, refID, 1, StartContext{Path: "/root/ref", Line: 12, Column: 13})
	captureIdentityField(t, &state, 0, "x")
	if err := finishSelectionsForTest(&state, info, 3, StartContext{Path: "/root/ref", Line: 14, Column: 15}, failIdentityReport(t)); err != nil {
		t.Fatalf("FinishSelections(ref) error = %v", err)
	}

	var got error
	if _, err := state.closeScopes(1, func(err error) error {
		got = err
		return nil
	}); err != nil {
		t.Fatalf("closeScopes(parent) error = %v", err)
	}
	expectXSDCode(t, got, xsderrors.CodeValidationIdentity)
	expectXSDMessage(t, got, "keyref does not resolve")
	expectXSDLocation(t, got, "/root/ref", 12, 13)
}

type identityConstraintInfoForTest struct {
	Kind  runtime.IdentityKind
	Refer runtime.IdentityConstraintID
}

type identityInfoRuntime map[runtime.IdentityConstraintID]identityConstraintInfoForTest

func identityInfo(in map[runtime.IdentityConstraintID]identityConstraintInfoForTest) identityInfoRuntime {
	return identityInfoRuntime(in)
}

func finishSelectionsForTest(
	s *identityState,
	infoByID identityInfoRuntime,
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
		info, ok := infoByID[sel.constraint]
		if !ok {
			return xsderrors.InternalInvariant("identity constraint metadata is invalid")
		}
		if err := s.finishSelectionWithConstraint(info.Kind, info.Refer, sel, identityLimits{}, ctx); err != nil {
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

func startIdentityScope(t *testing.T, state *identityState, constraints []runtime.IdentityConstraintID, depth int, path string) {
	t.Helper()
	const elem runtime.ElementID = 0
	constraintIDs, ok := runtime.ElementIdentityConstraintIDs([][]runtime.IdentityConstraintID{constraints}, elem)
	if !ok {
		t.Fatal("ElementIdentityConstraintIDs() rejected test fixture")
	}
	err := state.startScope(constraintIDs, depth, 0, StartContext{Path: path})
	if err != nil {
		t.Fatalf("startScope(depth=%d) error = %v", depth, err)
	}
}

func captureIdentityField(t *testing.T, state *identityState, selection int, value string) {
	t.Helper()
	err := state.captureFields([]identityFieldMatch{{Selection: selection, Field: 0}}, value, StartContext{Path: "/field", Line: 1, Column: 1})
	if err != nil {
		t.Fatalf("captureFields(selection=%d) error = %v", selection, err)
	}
}

func booleanRuntimeForTest(t *testing.T) (*runtime.Schema, runtime.SimpleTypeID) {
	t.Helper()
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="flag" type="xs:boolean"/></xs:schema>`)
	name, ok := rt.LookupQName("", "flag")
	if !ok {
		t.Fatal("LookupQName(flag) failed")
	}
	_, typ, ok := rt.RootElement(runtime.RuntimeName{Known: true, Name: name})
	if !ok {
		t.Fatal("RootElement(flag) failed")
	}
	id, ok := typ.Type.Simple()
	if !ok {
		t.Fatalf("RootElement(flag) type = %v, want simple type", typ)
	}
	return rt, id
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
