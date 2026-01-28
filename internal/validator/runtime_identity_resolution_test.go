package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestIdentityDuplicateUnique(t *testing.T) {
	fx := buildIdentityFixture()
	schema := fx.schema

	schema.ICs = make([]runtime.IdentityConstraint, 2)
	schema.ICs[1] = runtime.IdentityConstraint{
		Category:    runtime.ICUnique,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	schema.ICSelectors = []runtime.PathID{fx.pathChild}
	schema.ICFields = []runtime.PathID{fx.pathAttrID}
	schema.ElemICs = []runtime.ICID{1}
	schema.Elements[fx.elemRoot].ICOff = 0
	schema.Elements[fx.elemRoot].ICLen = 1

	sess := NewSession(schema)

	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemRoot, Type: fx.typeComplex, Sym: fx.symRoot, NS: fx.nsID,
	}); err != nil {
		t.Fatalf("identityStart root: %v", err)
	}
	attrs := []StartAttr{{
		Sym:   fx.symID,
		NS:    fx.empty,
		Local: []byte("id"),
		Value: []byte("dup"),
	}}
	for range 2 {
		if err := sess.identityStart(identityStartInput{
			Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID, Attrs: attrs,
		}); err != nil {
			t.Fatalf("identityStart item: %v", err)
		}
		if err := sess.identityEnd(identityEndInput{}); err != nil {
			t.Fatalf("identityEnd item: %v", err)
		}
	}
	if err := sess.identityEnd(identityEndInput{}); err != nil {
		t.Fatalf("identityEnd root: %v", err)
	}

	if len(sess.icState.violations) != 1 {
		t.Fatalf("violations = %d, want 1", len(sess.icState.violations))
	}
	if !strings.Contains(sess.icState.violations[0].Error(), "duplicate unique") {
		t.Fatalf("violation = %q, want duplicate unique", sess.icState.violations[0].Error())
	}
}

func TestIdentityKeyrefMissing(t *testing.T) {
	fx := buildIdentityFixture()
	schema := fx.schema

	schema.ICs = make([]runtime.IdentityConstraint, 3)
	schema.ICs[1] = runtime.IdentityConstraint{
		Category:    runtime.ICKey,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	schema.ICs[2] = runtime.IdentityConstraint{
		Category:    runtime.ICKeyRef,
		SelectorOff: 1,
		SelectorLen: 1,
		FieldOff:    1,
		FieldLen:    1,
		Referenced:  1,
	}
	schema.ICSelectors = []runtime.PathID{fx.pathGroupItem, fx.pathChild}
	schema.ICFields = []runtime.PathID{fx.pathAttrID, fx.pathAttrID}
	schema.ElemICs = []runtime.ICID{1, 2}
	schema.Elements[fx.elemRoot].ICOff = 0
	schema.Elements[fx.elemRoot].ICLen = 2

	runCase := func(keyValue, refValue string) int {
		sess := NewSession(schema)

		if err := sess.identityStart(identityStartInput{
			Elem: fx.elemRoot, Type: fx.typeComplex, Sym: fx.symRoot, NS: fx.nsID,
		}); err != nil {
			t.Fatalf("identityStart root: %v", err)
		}
		if err := sess.identityStart(identityStartInput{
			Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID,
			Attrs: []StartAttr{{
				Sym:   fx.symID,
				NS:    fx.empty,
				Local: []byte("id"),
				Value: []byte(refValue),
			}},
		}); err != nil {
			t.Fatalf("identityStart item: %v", err)
		}
		if err := sess.identityEnd(identityEndInput{}); err != nil {
			t.Fatalf("identityEnd item: %v", err)
		}
		if err := sess.identityStart(identityStartInput{
			Elem: fx.elemGroup, Type: fx.typeComplex, Sym: fx.symGroup, NS: fx.nsID,
		}); err != nil {
			t.Fatalf("identityStart group: %v", err)
		}
		if err := sess.identityStart(identityStartInput{
			Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID,
			Attrs: []StartAttr{{
				Sym:   fx.symID,
				NS:    fx.empty,
				Local: []byte("id"),
				Value: []byte(keyValue),
			}},
		}); err != nil {
			t.Fatalf("identityStart item: %v", err)
		}
		if err := sess.identityEnd(identityEndInput{}); err != nil {
			t.Fatalf("identityEnd item: %v", err)
		}
		if err := sess.identityEnd(identityEndInput{}); err != nil {
			t.Fatalf("identityEnd group: %v", err)
		}
		if err := sess.identityEnd(identityEndInput{}); err != nil {
			t.Fatalf("identityEnd root: %v", err)
		}
		return len(sess.icState.violations)
	}

	if got := runCase("two", "one"); got != 1 {
		t.Fatalf("missing keyref violations = %d, want 1", got)
	}
	if got := runCase("two", "two"); got != 0 {
		t.Fatalf("resolved keyref violations = %d, want 0", got)
	}
}
