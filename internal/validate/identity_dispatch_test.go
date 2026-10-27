package validate

import (
	"errors"
	"slices"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentityFieldDispatchRebuildsSelectionsAndClearsMatches(t *testing.T) {
	t.Parallel()
	evaluation := identityEvaluation{selections: []identitySelection{
		{constraint: 3, fieldStart: 2, fieldLen: 1},
		{constraint: 7, fieldStart: 63, fieldLen: 1},
		{constraint: 3, fieldStart: 64, fieldLen: 2},
	}, fieldValues: make([]identityFieldValue, 130)}
	evaluation.fieldDispatch.prepare(evaluation.selections)
	var matches []identityFieldMatch
	for range 2 {
		for _, candidate := range evaluation.fieldDispatch.forConstraint(3) {
			matches = evaluation.appendIdentityFieldMatch(matches, candidate.selection, 0)
		}
	}
	if want := []identityFieldMatch{{Selection: 0, Field: 0}, {Selection: 2, Field: 0}}; !slices.Equal(matches, want) {
		t.Fatalf("constraint join matches = %v, want %v", matches, want)
	}
	matches = evaluation.appendIdentityFieldMatch(matches, 1, 0)
	matches = evaluation.appendIdentityFieldMatch(matches, 2, 1)
	if len(matches) != 4 {
		t.Fatalf("adjacent bitset words lost distinct fields: %v", matches)
	}

	evaluation.clearIdentityFieldMatches(matches)
	// Compaction changes ordinals; every new collection derives them again.
	evaluation.selections = []identitySelection{{constraint: 7, fieldStart: 0, fieldLen: 1}}
	evaluation.fieldValues = evaluation.fieldValues[:1]
	evaluation.fieldDispatch.prepare(evaluation.selections)
	matches = evaluation.appendIdentityFieldMatch(matches[:0], 0, 0)
	if len(matches) != 1 || len(evaluation.fieldDispatch.forConstraint(3)) != 0 {
		t.Fatalf("compacted selection did not replace prior matches: %v", matches)
	}
	evaluation.clearIdentityFieldMatches(matches)
	evaluation.selections = []identitySelection{{constraint: 3, fieldStart: 63, fieldLen: 3}}
	evaluation.fieldValues = evaluation.fieldValues[:130]
	evaluation.fieldDispatch.prepare(evaluation.selections)
	matches = matches[:0]
	for _, field := range []int{0, 1, 2, 0, 1, 2} {
		matches = evaluation.appendIdentityFieldMatch(matches, 0, field)
	}
	if want := []identityFieldMatch{{Selection: 0, Field: 0}, {Selection: 0, Field: 1}, {Selection: 0, Field: 2}}; !slices.Equal(matches, want) {
		t.Fatalf("grow after shrink left stale match bits: %v, want %v", matches, want)
	}
	evaluation.clearIdentityFieldMatches(matches)
	// Attribute and element collections share scratch, but never match bits.
	evaluation.fieldDispatch.prepareMatches(130)
	if got := evaluation.appendIdentityFieldMatch(nil, 0, 2); len(got) != 1 {
		t.Fatalf("next collection retained an earlier field match: %v", got)
	}
}

func TestIdentityFieldDispatchRetentionBounds(t *testing.T) {
	t.Parallel()
	for _, extra := range []int{0, 1} {
		evaluation := identityEvaluation{fieldDispatch: identityFieldDispatchScratch{
			selections: make([]identitySelectionCandidate, 128+extra),
			matched:    make([]uint64, 2+extra),
		}}
		evaluation.fieldDispatch.matched[0] = 1
		retainedWords := evaluation.fieldDispatch.matched
		evaluation.fieldDispatch.reset(128)
		for _, word := range retainedWords {
			if word != 0 {
				t.Fatal("reset retained dirty bits after interrupted collection")
			}
		}
		if extra == 0 {
			if cap(evaluation.fieldDispatch.selections) != 128 || cap(evaluation.fieldDispatch.matched) != 2 {
				t.Fatal("reset discarded scratch at its retention bound")
			}
		} else if cap(evaluation.fieldDispatch.selections) != 0 || cap(evaluation.fieldDispatch.matched) != 0 {
			t.Fatal("reset retained scratch above its bound")
		}
		evaluation.selections = []identitySelection{{constraint: 1, fieldLen: 1}}
		evaluation.fieldValues = make([]identityFieldValue, 1)
		evaluation.fieldDispatch.prepare(evaluation.selections)
		if got := evaluation.appendIdentityFieldMatch(nil, 0, 0); len(got) != 1 {
			t.Fatalf("reuse after reset: %v", got)
		}
		evaluation.discard()
		if cap(evaluation.fieldDispatch.selections) != 0 || cap(evaluation.fieldDispatch.matched) != 0 {
			t.Fatal("discard retained field dispatch scratch")
		}
	}
}

func TestIdentityFieldDispatchCleansPartialAttributeFailure(t *testing.T) {
	t.Parallel()
	fixture := compiledIdentityRuntimeForTest(t)
	evaluation := newIdentityEvaluation(fixture.rt, identityLimits{}, 0)
	if err := evaluation.startElement(identityElementStart{
		Name:    xsdSchema.RuntimeName{Known: true, Name: fixture.elemName},
		Element: fixture.elemID,
		Mode:    elementAssessed,
	}); err != nil {
		t.Fatal(err)
	}
	evaluation.selections = append(evaluation.selections, identitySelection{
		constraint: xsdSchema.NoIdentityConstraint,
		fieldStart: len(evaluation.fieldValues),
		fieldLen:   1,
	})
	evaluation.fieldValues = append(evaluation.fieldValues, identityFieldValue{})
	attribute := xsdSchema.RuntimeName{Known: true, Name: fixture.attrName}
	_, err := evaluation.dispatchAttributeFieldMatches(attribute)
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Category() != xsderrors.CategoryInternal {
		t.Fatalf("invalid second program: %v", err)
	}
	evaluation.selections = evaluation.selections[:1]
	evaluation.fieldValues = evaluation.fieldValues[:1]
	matches, err := evaluation.dispatchAttributeFieldMatches(attribute)
	if err != nil || !slices.Equal(matches, []identityFieldMatch{{Selection: 0, Field: 0}}) {
		t.Fatalf("reuse after partial attribute failure = %v, %v", matches, err)
	}
}

func TestIdentityFieldDispatchDoesNotAdmitBitmapWithoutMatches(t *testing.T) {
	t.Parallel()
	fixture := compiledIdentityRuntimeForTest(t)
	evaluation := newIdentityEvaluation(fixture.rt, identityLimits{}, 0)
	if err := evaluation.startElement(identityElementStart{
		Name:    xsdSchema.RuntimeName{Known: true, Name: fixture.elemName},
		Element: fixture.elemID,
		Mode:    elementAssessed,
	}); err != nil {
		t.Fatal(err)
	}
	evaluation.fieldDispatch.prepare(evaluation.selections)
	if cap(evaluation.fieldDispatch.matched) != 0 {
		t.Fatal("selection index construction admitted a field bitmap")
	}
	matches, err := evaluation.dispatchAttributeFieldMatches(xsdSchema.RuntimeName{NS: "urn:other", Local: "absent"})
	if err != nil || len(matches) != 0 || cap(evaluation.fieldDispatch.matched) != 0 {
		t.Fatalf("unmatched attribute admitted a field bitmap: %v, %v", matches, err)
	}
	evaluation.fieldValues = make([]identityFieldValue, 65)
	evaluation.selections[0].fieldStart = 64
	for range 2 {
		matches = evaluation.dispatchElementFieldMatches()
		if !slices.Equal(matches, []identityFieldMatch{{Selection: 0, Field: 0}}) {
			t.Fatalf("first match at slot64 after no-hit collection: %v", matches)
		}
	}
}
