package model

import "testing"

type normalizeTestType struct {
	name      string
	builtin   bool
	ws        WhiteSpace
	primitive Type
}

func (t *normalizeTestType) Name() QName {
	if t == nil {
		return QName{}
	}
	return QName{Local: t.name}
}

func (t *normalizeTestType) IsBuiltin() bool {
	if t == nil {
		return false
	}
	return t.builtin
}

func (t *normalizeTestType) BaseType() Type {
	return nil
}

func (t *normalizeTestType) PrimitiveType() Type {
	if t == nil {
		return nil
	}
	return t.primitive
}

func (t *normalizeTestType) FundamentalFacets() *FundamentalFacets {
	return nil
}

func (t *normalizeTestType) WhiteSpace() WhiteSpace {
	if t == nil {
		return WhiteSpacePreserve
	}
	return t.ws
}

func TestNormalizeTypeValueRejectsNilType(t *testing.T) {
	t.Parallel()

	_, err := NormalizeTypeValue("x", nil)
	if err == nil {
		t.Fatalf("expected error for nil type")
	}
}

func TestNormalizeTypeValueTemporalTrimsOuterWhitespace(t *testing.T) {
	t.Parallel()

	typ := &normalizeTestType{
		name:    "dateTime",
		builtin: true,
		ws:      WhiteSpacePreserve,
	}

	got, err := NormalizeTypeValue("  2001-01-01T00:00:00  ", typ)
	if err != nil {
		t.Fatalf("NormalizeTypeValue() error = %v", err)
	}
	if got != "2001-01-01T00:00:00" {
		t.Fatalf("NormalizeTypeValue() = %q, want %q", got, "2001-01-01T00:00:00")
	}
}

func TestNormalizeTypeValueNonTemporalKeepsOuterWhitespace(t *testing.T) {
	t.Parallel()

	typ := &normalizeTestType{
		name:    "string",
		builtin: true,
		ws:      WhiteSpacePreserve,
	}

	got, err := NormalizeTypeValue("  a  ", typ)
	if err != nil {
		t.Fatalf("NormalizeTypeValue() error = %v", err)
	}
	if got != "  a  " {
		t.Fatalf("NormalizeTypeValue() = %q, want %q", got, "  a  ")
	}
}

func TestNormalizeTypeValueUsesPrimitiveForDerivedTypes(t *testing.T) {
	t.Parallel()

	primitive := &normalizeTestType{
		name:    "dateTime",
		builtin: true,
		ws:      WhiteSpacePreserve,
	}
	typ := &normalizeTestType{
		name:      "myDateTime",
		builtin:   false,
		ws:        WhiteSpacePreserve,
		primitive: primitive,
	}

	got, err := NormalizeTypeValue("  2001-01-01T00:00:00  ", typ)
	if err != nil {
		t.Fatalf("NormalizeTypeValue() error = %v", err)
	}
	if got != "2001-01-01T00:00:00" {
		t.Fatalf("NormalizeTypeValue() = %q, want %q", got, "2001-01-01T00:00:00")
	}
}
