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

	shape := ElementStartDeclShape{
		Type:     ComplexRef(2),
		Block:    DerivationRestriction,
		Abstract: true,
		Nillable: true,
		Fixed:    true,
	}
	info := NewElementStartInfoForDecl(shape)
	if info.Type != shape.Type ||
		info.Block != shape.Block ||
		info.Abstract != shape.Abstract ||
		info.Nillable != shape.Nillable ||
		info.Fixed != shape.Fixed {
		t.Fatalf("NewElementStartInfoForDecl() = %+v, want projected declaration facts", info)
	}
	if !EqualElementStartInfoForDecl(info, shape) {
		t.Fatal("EqualElementStartInfoForDecl() = false, want true")
	}
	infos := NewElementStartInfosForDecls([]ElementStartDeclShape{shape})
	if len(infos) != 1 || infos[0] != info {
		t.Fatalf("NewElementStartInfosForDecls() = %+v, want single projected declaration fact", infos)
	}
	if !EqualElementStartInfosForDecls(infos, []ElementStartDeclShape{shape}) {
		t.Fatal("EqualElementStartInfosForDecls() = false, want true")
	}
	if EqualElementStartInfosForDecls(infos, nil) {
		t.Fatal("EqualElementStartInfosForDecls() accepted different length")
	}

	decl := ElementDecl{
		Type:     shape.Type,
		Block:    shape.Block,
		Abstract: shape.Abstract,
		Nillable: shape.Nillable,
		Fixed:    &ValueConstraint{},
	}
	declInfo := NewElementStartInfoForElementDecl(decl)
	if declInfo != info {
		t.Fatalf("NewElementStartInfoForElementDecl() = %+v, want %+v", declInfo, info)
	}
	if !EqualElementStartInfoForElementDecl(declInfo, decl) {
		t.Fatal("EqualElementStartInfoForElementDecl() = false, want true")
	}
	declInfos := NewElementStartInfosForElementDecls([]ElementDecl{decl})
	if len(declInfos) != 1 || declInfos[0] != info {
		t.Fatalf("NewElementStartInfosForElementDecls() = %+v, want single projected declaration fact", declInfos)
	}
	if got, ok := DeclaredElementTypeByID(declInfos, 0); !ok || got != shape.Type {
		t.Fatalf("DeclaredElementTypeByID() = %v, %v; want %v, true", got, ok, shape.Type)
	}
	if got, ok := DeclaredElementTypeByID(declInfos, ElementID(99)); ok || got != (TypeID{}) {
		t.Fatalf("DeclaredElementTypeByID(invalid) = %v, %v; want zero, false", got, ok)
	}
	if got, ok := ElementStartInfoByID(declInfos, 0); !ok || got != info {
		t.Fatalf("ElementStartInfoByID() = %+v, %v; want %+v, true", got, ok, info)
	}
	if got, ok := ElementStartInfoByID(declInfos, ElementID(99)); ok || got != (ElementStartInfo{}) {
		t.Fatalf("ElementStartInfoByID(invalid) = %+v, %v; want zero, false", got, ok)
	}
	if !EqualElementStartInfosForElementDecls(declInfos, []ElementDecl{decl}) {
		t.Fatal("EqualElementStartInfosForElementDecls() = false, want true")
	}
	if EqualElementStartInfosForElementDecls(declInfos, nil) {
		t.Fatal("EqualElementStartInfosForElementDecls() accepted different length")
	}
	if err := ValidateElementStartInfosForElementDecls(declInfos, []ElementDecl{decl}); err != nil {
		t.Fatalf("ValidateElementStartInfosForElementDecls() error = %v", err)
	}
	if err := ValidateElementStartInfosForElementDecls(declInfos[:0], []ElementDecl{decl}); err == nil || err.Error() != "element start projection count does not match declarations" {
		t.Fatalf("ValidateElementStartInfosForElementDecls(short) error = %v, want count invariant", err)
	}

	tests := []struct {
		name  string
		shape ElementStartDeclShape
	}{
		{
			name: "type differs",
			shape: ElementStartDeclShape{
				Type:     SimpleRef(1),
				Block:    shape.Block,
				Abstract: shape.Abstract,
				Nillable: shape.Nillable,
				Fixed:    shape.Fixed,
			},
		},
		{
			name: "block differs",
			shape: ElementStartDeclShape{
				Type:     shape.Type,
				Block:    DerivationExtension,
				Abstract: shape.Abstract,
				Nillable: shape.Nillable,
				Fixed:    shape.Fixed,
			},
		},
		{
			name: "abstract differs",
			shape: ElementStartDeclShape{
				Type:     shape.Type,
				Block:    shape.Block,
				Nillable: shape.Nillable,
				Fixed:    shape.Fixed,
			},
		},
		{
			name: "nillable differs",
			shape: ElementStartDeclShape{
				Type:     shape.Type,
				Block:    shape.Block,
				Abstract: shape.Abstract,
				Fixed:    shape.Fixed,
			},
		},
		{
			name: "fixed differs",
			shape: ElementStartDeclShape{
				Type:     shape.Type,
				Block:    shape.Block,
				Abstract: shape.Abstract,
				Nillable: shape.Nillable,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if EqualElementStartInfoForDecl(info, tt.shape) {
				t.Fatal("EqualElementStartInfoForDecl() = true, want false")
			}
			if EqualElementStartInfosForDecls(infos, []ElementStartDeclShape{tt.shape}) {
				t.Fatal("EqualElementStartInfosForDecls() = true, want false")
			}
			changedDecl := decl
			changedDecl.Type = tt.shape.Type
			changedDecl.Block = tt.shape.Block
			changedDecl.Abstract = tt.shape.Abstract
			changedDecl.Nillable = tt.shape.Nillable
			if !tt.shape.Fixed {
				changedDecl.Fixed = nil
			}
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
	if !EqualTypeInfo(info, same) {
		t.Fatal("EqualTypeInfo() = false, want true")
	}
	changed := NewTypeInfo(TypeInfoShape{Block: DerivationRestriction})
	if EqualTypeInfo(info, changed) {
		t.Fatal("EqualTypeInfo() = true for different abstract flag")
	}
}

func TestTypeInfoForComplexType(t *testing.T) {
	t.Parallel()

	ct := ComplexType{
		Block:    DerivationExtension,
		Abstract: true,
	}
	info := NewTypeInfoForComplexType(ct)
	if info.Block != ct.Block || info.Abstract != ct.Abstract {
		t.Fatalf("NewTypeInfoForComplexType() = %+v, want projected complex type facts", info)
	}
	if !EqualTypeInfoForComplexType(info, ct) {
		t.Fatal("EqualTypeInfoForComplexType() = false, want true")
	}
	infos := NewTypeInfosForComplexTypes([]ComplexType{ct})
	if len(infos) != 1 || infos[0] != info {
		t.Fatalf("NewTypeInfosForComplexTypes() = %+v, want single projected complex type fact", infos)
	}
	if !EqualTypeInfosForComplexTypes(infos, []ComplexType{ct}) {
		t.Fatal("EqualTypeInfosForComplexTypes() = false, want true")
	}
	if EqualTypeInfosForComplexTypes(infos, nil) {
		t.Fatal("EqualTypeInfosForComplexTypes() accepted different length")
	}
	if err := ValidateTypeInfosForComplexTypes(infos, []ComplexType{ct}); err != nil {
		t.Fatalf("ValidateTypeInfosForComplexTypes() error = %v", err)
	}
	if err := ValidateTypeInfosForComplexTypes(infos[:0], []ComplexType{ct}); err == nil || err.Error() != "complex type info projection count does not match types" {
		t.Fatalf("ValidateTypeInfosForComplexTypes(short) error = %v, want count invariant", err)
	}
	if got, ok := TypeInfoByID(1, infos, ComplexRef(0)); !ok || got != info {
		t.Fatalf("TypeInfoByID(complex) = %+v, %v; want %+v, true", got, ok, info)
	}
	if got, ok := TypeInfoByID(1, infos, SimpleRef(0)); !ok || got != (TypeInfo{}) {
		t.Fatalf("TypeInfoByID(simple) = %+v, %v; want zero, true", got, ok)
	}
	if got, ok := TypeInfoByID(1, infos, ComplexRef(1)); ok || got != (TypeInfo{}) {
		t.Fatalf("TypeInfoByID(invalid complex) = %+v, %v; want zero, false", got, ok)
	}
	if got, ok := TypeInfoByID(1, infos, SimpleRef(1)); ok || got != (TypeInfo{}) {
		t.Fatalf("TypeInfoByID(invalid simple) = %+v, %v; want zero, false", got, ok)
	}

	if EqualTypeInfoForComplexType(info, ComplexType{Block: DerivationRestriction, Abstract: true}) {
		t.Fatal("EqualTypeInfoForComplexType() accepted wrong block")
	}
	if EqualTypeInfosForComplexTypes(infos, []ComplexType{{Block: DerivationRestriction, Abstract: true}}) {
		t.Fatal("EqualTypeInfosForComplexTypes() accepted wrong block")
	}
	if err := ValidateTypeInfosForComplexTypes(infos, []ComplexType{{Block: DerivationRestriction, Abstract: true}}); err == nil || err.Error() != "complex type info projection does not match complex type" {
		t.Fatalf("ValidateTypeInfosForComplexTypes(changed block) error = %v, want mismatch invariant", err)
	}
	if EqualTypeInfoForComplexType(info, ComplexType{Block: ct.Block}) {
		t.Fatal("EqualTypeInfoForComplexType() accepted wrong abstract flag")
	}
	if EqualTypeInfosForComplexTypes(infos, []ComplexType{{Block: ct.Block}}) {
		t.Fatal("EqualTypeInfosForComplexTypes() accepted wrong abstract flag")
	}
}
