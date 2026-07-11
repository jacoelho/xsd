package runtime

import "testing"

func TestElementStartInfoProjection(t *testing.T) {
	t.Parallel()

	info := NewElementStartInfo(ElementStartInfoShape{
		Type:     SimpleRef(1),
		Block:    DerivationExtension,
		Abstract: true,
		Nillable: true,
		Fixed:    true,
	})
	if info.Type != SimpleRef(1) ||
		info.Block != DerivationExtension ||
		!info.Abstract ||
		!info.Nillable ||
		!info.Fixed {
		t.Fatalf("NewElementStartInfo() = %+v, want projected facts", info)
	}

	same := NewElementStartInfo(ElementStartInfoShape{
		Type:     SimpleRef(1),
		Block:    DerivationExtension,
		Abstract: true,
		Nillable: true,
		Fixed:    true,
	})
	if !EqualElementStartInfo(info, same) {
		t.Fatal("EqualElementStartInfo() = false, want true")
	}
	changed := NewElementStartInfo(ElementStartInfoShape{
		Type:     SimpleRef(1),
		Block:    DerivationExtension,
		Abstract: true,
		Nillable: true,
	})
	if EqualElementStartInfo(info, changed) {
		t.Fatal("EqualElementStartInfo() = true for different fixed flag")
	}
}

func TestElementStartInfoForDecl(t *testing.T) {
	t.Parallel()

	decl := ElementDecl{
		Type:     ComplexRef(2),
		Block:    DerivationRestriction,
		Abstract: true,
		Nillable: true,
		Fixed:    &ValueConstraint{},
		Default:  &ValueConstraint{},
	}
	info := NewElementStartInfoForElementDecl(decl)
	if info.Type != decl.Type ||
		info.Block != decl.Block ||
		info.Abstract != decl.Abstract ||
		info.Nillable != decl.Nillable ||
		!info.Fixed ||
		!info.Default {
		t.Fatalf("NewElementStartInfoForElementDecl() = %+v, want projected declaration facts", info)
	}
	if !EqualElementStartInfoForElementDecl(info, decl) {
		t.Fatal("EqualElementStartInfoForElementDecl() = false, want true")
	}
	infos := NewElementStartInfosForElementDecls([]ElementDecl{decl})
	if len(infos) != 1 || infos[0] != info {
		t.Fatalf("NewElementStartInfosForElementDecls() = %+v, want single projected declaration fact", infos)
	}
	if got, ok := ElementStartInfoByID(infos, 0); !ok || got != info {
		t.Fatalf("ElementStartInfoByID() = %+v, %v; want %+v, true", got, ok, info)
	}
	if got, ok := ElementStartInfoByID(infos, ElementID(99)); ok || got != (ElementStartInfo{}) {
		t.Fatalf("ElementStartInfoByID(invalid) = %+v, %v; want zero, false", got, ok)
	}
	if !EqualElementStartInfosForElementDecls(infos, []ElementDecl{decl}) {
		t.Fatal("EqualElementStartInfosForElementDecls() = false, want true")
	}
	if EqualElementStartInfosForElementDecls(infos, nil) {
		t.Fatal("EqualElementStartInfosForElementDecls() accepted different length")
	}
	if err := ValidateElementStartInfosForElementDecls(infos, []ElementDecl{decl}); err != nil {
		t.Fatalf("ValidateElementStartInfosForElementDecls() error = %v", err)
	}
	if err := ValidateElementStartInfosForElementDecls(infos[:0], []ElementDecl{decl}); err == nil || err.Error() != "element start projection count does not match declarations" {
		t.Fatalf("ValidateElementStartInfosForElementDecls(short) error = %v, want count invariant", err)
	}

	tests := []struct {
		name   string
		mutate func(*ElementDecl)
	}{
		{"type differs", func(decl *ElementDecl) { decl.Type = SimpleRef(1) }},
		{"block differs", func(decl *ElementDecl) { decl.Block = DerivationExtension }},
		{"abstract differs", func(decl *ElementDecl) { decl.Abstract = false }},
		{"nillable differs", func(decl *ElementDecl) { decl.Nillable = false }},
		{"fixed differs", func(decl *ElementDecl) { decl.Fixed = nil }},
		{"default differs", func(decl *ElementDecl) { decl.Default = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			changedDecl := decl
			tt.mutate(&changedDecl)
			if EqualElementStartInfoForElementDecl(info, changedDecl) {
				t.Fatal("EqualElementStartInfoForElementDecl() = true, want false")
			}
			if EqualElementStartInfosForElementDecls(infos, []ElementDecl{changedDecl}) {
				t.Fatal("EqualElementStartInfosForElementDecls() = true, want false")
			}
			if err := ValidateElementStartInfosForElementDecls(infos, []ElementDecl{changedDecl}); err == nil || err.Error() != "element start projection does not match declaration" {
				t.Fatalf("ValidateElementStartInfosForElementDecls(changed) error = %v, want mismatch invariant", err)
			}
		})
	}
}

func TestTypeInfoProjection(t *testing.T) {
	t.Parallel()

	info := NewTypeInfo(TypeInfoShape{
		Block:    DerivationRestriction,
		Abstract: true,
	})
	if info.Block != DerivationRestriction || !info.Abstract {
		t.Fatalf("NewTypeInfo() = %+v, want projected facts", info)
	}

	same := NewTypeInfo(TypeInfoShape{
		Block:    DerivationRestriction,
		Abstract: true,
	})
	if info != same {
		t.Fatal("equivalent type info values differ")
	}
	changed := NewTypeInfo(TypeInfoShape{Block: DerivationRestriction})
	if info == changed {
		t.Fatal("type info values match despite different abstract flag")
	}
}

func TestTypeInfoForComplexType(t *testing.T) {
	t.Parallel()

	ct := ComplexType{
		Block:    DerivationExtension,
		Abstract: true,
	}
	info := NewTypeInfo(TypeInfoShape{Block: ct.Block, Abstract: ct.Abstract})
	if info.Block != ct.Block || info.Abstract != ct.Abstract {
		t.Fatalf("NewTypeInfo() = %+v, want projected complex type facts", info)
	}
}
