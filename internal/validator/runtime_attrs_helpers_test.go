package validator

import "testing"

func TestValidateSimpleTypeAttrsRejectsNonXsi(t *testing.T) {
	schema, ids := buildAttrFixture()
	sess := NewSession(schema)

	attrs := []StartAttr{{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")}}
	_, err := sess.validateSimpleTypeAttrs(attrs, false)
	if err == nil {
		t.Fatalf("expected non-xsi attribute error")
	}
}

func TestValidateComplexAttrsMarksPresent(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired()
	sess := NewSession(schema)

	ct := &schema.ComplexTypes[1]
	uses := sess.attrUses(ct.Attrs)
	present := sess.prepareAttrPresent(len(uses))

	attrs := []StartAttr{{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")}}
	validated, seenID, err := sess.validateComplexAttrs(ct, present, attrs, nil, true)
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
	schema, _ := buildAttrFixtureNoRequired()
	sess := NewSession(schema)

	ct := &schema.ComplexTypes[1]
	uses := sess.attrUses(ct.Attrs)
	present := sess.prepareAttrPresent(len(uses))

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
