package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/validator/attrs"
)

func TestValidateSimpleTypeAttrsRejectsNonXsi(t *testing.T) {
	schema, ids := buildAttrFixture(t)
	sess := NewSession(schema)

	startAttrs := []attrs.Start{{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")}}
	classified, err := sess.classifyAttrs(startAttrs, false)
	if err != nil {
		t.Fatalf("classifyAttrs: %v", err)
	}
	_, err = sess.validateSimpleTypeAttrsClassified(startAttrs, classified.Classes, false)
	if err == nil {
		t.Fatalf("expected non-xsi attribute error")
	}
}

func TestValidateComplexAttrsMarksPresent(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	ct := &schema.ComplexTypes[1]
	uses := attrs.Uses(sess.rt.AttrIndex.Uses, ct.Attrs)
	present := sess.attrState.PreparePresent(len(uses))

	startAttrs := []attrs.Start{{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")}}
	classified, err := sess.classifyAttrs(startAttrs, false)
	if err != nil {
		t.Fatalf("classifyAttrs: %v", err)
	}
	validated, seenID, err := sess.validateComplexAttrsClassified(
		ct,
		present,
		startAttrs,
		classified.Classes,
		nil,
		true,
		sess.attrState.PrepareValidated(true, len(startAttrs)),
	)
	if err != nil {
		t.Fatalf("validateComplexAttrs: %v", err)
	}
	if seenID {
		t.Fatalf("expected seenID false")
	}
	if len(validated) != 1 {
		t.Fatalf("validated attrs = %d, want 1", len(validated))
	}
	if len(present) == 0 || !present[0] {
		t.Fatalf("present[0] = %v, want true", present)
	}
}

func TestApplyDefaultAttrsAddsDefault(t *testing.T) {
	schema, _ := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	ct := &schema.ComplexTypes[1]
	uses := attrs.Uses(sess.rt.AttrIndex.Uses, ct.Attrs)
	present := sess.attrState.PreparePresent(len(uses))

	applied, err := sess.applyDefaultAttrs(uses, present, false, false)
	if err != nil {
		t.Fatalf("applyDefaultAttrs: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("applied defaults = %d, want 1", len(applied))
	}
	if applied[0].Name != uses[0].Name {
		t.Fatalf("applied name = %d, want %d", applied[0].Name, uses[0].Name)
	}
}
