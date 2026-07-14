package runtime

import (
	"reflect"
	"strconv"
	"testing"
)

func TestRuntimeNameLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		in   RuntimeName
	}{
		{
			name: "known names use local label",
			in:   RuntimeName{Known: true, NS: "urn:test", Local: "item"},
			want: "item",
		},
		{
			name: "unknown names use expanded label",
			in:   RuntimeName{NS: "urn:test", Local: "item"},
			want: "{urn:test}item",
		},
		{
			name: "empty namespace uses local label",
			in:   RuntimeName{Local: "item"},
			want: "item",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.in.Label(); got != tt.want {
				t.Fatalf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidUint32Index(t *testing.T) {
	t.Parallel()

	if !ValidUint32Index(2, 3) {
		t.Fatal("2 should be a valid index into length 3")
	}
	if ValidUint32Index(3, 3) {
		t.Fatal("3 should not be a valid index into length 3")
	}
	if ValidUint32Index(0, -1) {
		t.Fatal("negative length should reject all indexes")
	}
}

func TestTypedIDValidators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		valid  func(uint32, int) bool
		absent func(int) bool
		name   string
	}{
		{
			name:   "simple type",
			valid:  func(id uint32, n int) bool { return ValidSimpleTypeID(SimpleTypeID(id), n) },
			absent: func(n int) bool { return ValidSimpleTypeID(NoSimpleType, n) },
		},
		{
			name:   "complex type",
			valid:  func(id uint32, n int) bool { return ValidComplexTypeID(ComplexTypeID(id), n) },
			absent: func(n int) bool { return ValidComplexTypeID(NoComplexType, n) },
		},
		{
			name:   "element",
			valid:  func(id uint32, n int) bool { return ValidElementID(ElementID(id), n) },
			absent: func(n int) bool { return ValidElementID(NoElement, n) },
		},
		{
			name:   "attribute",
			valid:  func(id uint32, n int) bool { return ValidAttributeID(AttributeID(id), n) },
			absent: func(n int) bool { return ValidAttributeID(AttributeID(invalidID), n) },
		},
		{
			name:   "content model",
			valid:  func(id uint32, n int) bool { return ValidContentModelID(ContentModelID(id), n) },
			absent: func(n int) bool { return ValidContentModelID(NoContentModel, n) },
		},
		{
			name:   "attribute use set",
			valid:  func(id uint32, n int) bool { return ValidAttributeUseSetID(AttributeUseSetID(id), n) },
			absent: func(n int) bool { return ValidAttributeUseSetID(NoAttributeUseSet, n) },
		},
		{
			name:   "wildcard",
			valid:  func(id uint32, n int) bool { return ValidWildcardID(WildcardID(id), n) },
			absent: func(n int) bool { return ValidWildcardID(NoWildcard, n) },
		},
		{
			name:   "identity constraint",
			valid:  func(id uint32, n int) bool { return ValidIdentityConstraintID(IdentityConstraintID(id), n) },
			absent: func(n int) bool { return ValidIdentityConstraintID(NoIdentityConstraint, n) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !tt.valid(2, 3) {
				t.Fatal("2 should be a valid ID into length 3")
			}
			if tt.valid(3, 3) {
				t.Fatal("3 should not be a valid ID into length 3")
			}
			if tt.valid(0, -1) {
				t.Fatal("negative length should reject all IDs")
			}
			if tt.absent(3) {
				t.Fatal("absent sentinel should not be a valid ID")
			}
			if strconv.IntSize > 32 {
				maxID := uint64(invalidID)
				if tt.absent(int(maxID + 1)) {
					t.Fatal("absent sentinel should not be valid for an oversized table")
				}
			}
		})
	}
}

func TestRuntimeIDTableLength(t *testing.T) {
	t.Parallel()

	if strconv.IntSize <= 32 {
		t.Skip("runtime ID table boundary requires a 64-bit int")
	}
	maxID := uint64(invalidID)
	if !validRuntimeIDTableLength(int(maxID)) {
		t.Fatal("maximum sentinel-excluding table length should be valid")
	}
	if validRuntimeIDTableLength(int(maxID + 1)) {
		t.Fatal("table length containing the absent sentinel should be invalid")
	}
}

func TestElementIndexIDRejectsAbsentSentinel(t *testing.T) {
	t.Parallel()

	if strconv.IntSize <= 32 {
		t.Skip("absent sentinel cannot be represented as a non-negative int")
	}
	maxID := uint64(invalidID)
	if id, ok := elementIndexID(int(maxID)); ok || id != NoElement {
		t.Fatalf("elementIndexID(invalidID) = %d, %v, want NoElement, false", id, ok)
	}
}

func TestNextRuntimeIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		next func(int) (uint32, bool)
	}{
		{name: "simple type", next: func(n int) (uint32, bool) {
			id, ok := NextSimpleTypeID(n)
			return uint32(id), ok
		}},
		{name: "complex type", next: func(n int) (uint32, bool) {
			id, ok := NextComplexTypeID(n)
			return uint32(id), ok
		}},
		{name: "element", next: func(n int) (uint32, bool) {
			id, ok := NextElementID(n)
			return uint32(id), ok
		}},
		{name: "attribute", next: func(n int) (uint32, bool) {
			id, ok := NextAttributeID(n)
			return uint32(id), ok
		}},
		{name: "content model", next: func(n int) (uint32, bool) {
			id, ok := NextContentModelID(n)
			return uint32(id), ok
		}},
		{name: "attribute use set", next: func(n int) (uint32, bool) {
			id, ok := NextAttributeUseSetID(n)
			return uint32(id), ok
		}},
		{name: "wildcard", next: func(n int) (uint32, bool) {
			id, ok := NextWildcardID(n)
			return uint32(id), ok
		}},
		{name: "identity constraint", next: func(n int) (uint32, bool) {
			id, ok := NextIdentityConstraintID(n)
			return uint32(id), ok
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id, ok := tt.next(2)
			if !ok || id != 2 {
				t.Fatalf("next(2) = %d, %v, want 2, true", id, ok)
			}
			if _, ok := tt.next(-1); ok {
				t.Fatal("negative length should not allocate an ID")
			}
			if strconv.IntSize > 32 {
				if _, ok := tt.next(int(invalidID)); ok {
					t.Fatal("absent sentinel should not be allocated as an ID")
				}
			}
		})
	}
}

func TestNewUint32Index(t *testing.T) {
	t.Parallel()

	id, ok := NewUint32Index(int(^uint32(0)))
	if !ok || id != ^uint32(0) {
		t.Fatalf("NewUint32Index(max uint32) = %d, %v, want max uint32, true", id, ok)
	}
	if _, ok := NewUint32Index(-1); ok {
		t.Fatal("negative index should fail")
	}
}

func TestTypeID(t *testing.T) {
	t.Parallel()

	simple := SimpleTypeID(7)
	if !SimpleRef(simple).IsSimple() || SimpleRef(simple).IsComplex() {
		t.Fatalf("SimpleRef(%d) has wrong classification", simple)
	}
	if got, ok := SimpleRef(simple).Simple(); !ok || got != simple {
		t.Fatalf("SimpleRef(%d).Simple() = %d, %v; want %d, true", simple, got, ok, simple)
	}
	if got, ok := SimpleRef(simple).Complex(); ok || got != NoComplexType {
		t.Fatalf("SimpleRef(%d).Complex() = %d, %v; want NoComplexType, false", simple, got, ok)
	}

	complexID := ComplexTypeID(11)
	if !ComplexRef(complexID).IsComplex() || ComplexRef(complexID).IsSimple() {
		t.Fatalf("ComplexRef(%d) has wrong classification", complexID)
	}
	if got, ok := ComplexRef(complexID).Complex(); !ok || got != complexID {
		t.Fatalf("ComplexRef(%d).Complex() = %d, %v; want %d, true", complexID, got, ok, complexID)
	}
	if got, ok := ComplexRef(complexID).Simple(); ok || got != NoSimpleType {
		t.Fatalf("ComplexRef(%d).Simple() = %d, %v; want NoSimpleType, false", complexID, got, ok)
	}
	if (TypeID{}).IsSimple() || (TypeID{}).IsComplex() {
		t.Fatal("zero TypeID has a concrete classification")
	}
	invalid := TypeID{kind: typeKind(255), id: 1}
	if invalid.IsSimple() || invalid.IsComplex() {
		t.Fatal("invalid TypeID tag has a concrete classification")
	}
}

func TestTypeIDRepresentationIsOpaque(t *testing.T) {
	typ := reflect.TypeFor[TypeID]()
	for field := range typ.Fields() {
		if field.IsExported() {
			t.Fatalf("TypeID field %s is exported", field.Name)
		}
	}
}

func TestValidTypeID(t *testing.T) {
	t.Parallel()

	if !validTypeID(SimpleRef(2), 3, 0) {
		t.Fatal("SimpleRef(2) should be valid into simple length 3")
	}
	if validTypeID(SimpleRef(3), 3, 0) {
		t.Fatal("SimpleRef(3) should be invalid into simple length 3")
	}
	if !validTypeID(ComplexRef(2), 0, 3) {
		t.Fatal("ComplexRef(2) should be valid into complex length 3")
	}
	if validTypeID(ComplexRef(3), 0, 3) {
		t.Fatal("ComplexRef(3) should be invalid into complex length 3")
	}
	if validTypeID(TypeID{}, 3, 3) {
		t.Fatal("zero TypeID should be invalid")
	}
	if validTypeID(SimpleRef(0), -1, 3) || validTypeID(ComplexRef(0), 3, -1) {
		t.Fatal("negative table length should reject type IDs")
	}
}

func TestNoQNameReturnsIndependentValue(t *testing.T) {
	first := NoQName()
	first.Namespace = EmptyNamespaceID
	first.Local = 0
	if first == NoQName() {
		t.Fatal("mutating returned QName did not change the caller-owned value")
	}
	if second := NoQName(); second.Namespace != NamespaceID(invalidID) || second.Local != LocalNameID(invalidID) {
		t.Fatalf("NoQName() after caller mutation = %+v", second)
	}
}

func TestDerivationMaskBitsAreDistinct(t *testing.T) {
	t.Parallel()

	bits := []DerivationMask{
		DerivationExtension,
		DerivationRestriction,
		DerivationSubstitution,
		DerivationList,
		DerivationUnion,
	}
	var seen DerivationMask
	for _, bit := range bits {
		if bit == 0 || bit&(bit-1) != 0 {
			t.Fatalf("derivation bit %08b is not a single non-zero bit", bit)
		}
		if seen&bit != 0 {
			t.Fatalf("derivation bit %08b is duplicated in %08b", bit, seen)
		}
		seen |= bit
	}
}

func TestDerivationMaskClasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		check func(DerivationMask) bool
		name  string
		mask  DerivationMask
		valid bool
	}{
		{name: "element block allows substitution", mask: DerivationSubstitution, valid: true, check: ValidElementBlockMask},
		{name: "element block rejects list", mask: DerivationList, check: ValidElementBlockMask},
		{name: "element final allows extension restriction", mask: DerivationComplexMask, valid: true, check: ValidElementFinalMask},
		{name: "element final rejects union", mask: DerivationUnion, check: ValidElementFinalMask},
		{name: "complex block allows complex mask", mask: DerivationComplexMask, valid: true, check: ValidComplexBlockMask},
		{name: "complex block rejects substitution", mask: DerivationSubstitution, check: ValidComplexBlockMask},
		{name: "complex final allows complex mask", mask: DerivationComplexMask, valid: true, check: ValidComplexFinalMask},
		{name: "complex final rejects list", mask: DerivationList, check: ValidComplexFinalMask},
		{name: "simple final allows simple mask", mask: DerivationSimpleFinalMask, valid: true, check: ValidSimpleFinalMask},
		{name: "simple final rejects extension", mask: DerivationExtension, check: ValidSimpleFinalMask},
		{name: "final default allows simple and complex derivations", mask: DerivationFinalDefaultMask, valid: true, check: func(mask DerivationMask) bool {
			return mask&^DerivationFinalDefaultMask == 0
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.check(tt.mask); got != tt.valid {
				t.Fatalf("check(%08b) = %v, want %v", tt.mask, got, tt.valid)
			}
		})
	}
}

func TestParseDerivationSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		wantToken string
		allowed   DerivationMask
		want      DerivationMask
		wantKind  DerivationSetIssueKind
	}{
		{
			name:    "empty",
			allowed: DerivationFinalDefaultMask,
		},
		{
			name:    "multiple tokens",
			value:   derivationSetExtensionToken + " " + derivationSetRestrictionToken,
			allowed: DerivationComplexMask,
			want:    DerivationExtension | DerivationRestriction,
		},
		{
			name:    "xml whitespace",
			value:   derivationSetExtensionToken + "\t\n" + derivationSetRestrictionToken,
			allowed: DerivationComplexMask,
			want:    DerivationExtension | DerivationRestriction,
		},
		{
			name:      "non xml whitespace is not a separator",
			value:     "extension\u00a0restriction",
			allowed:   DerivationComplexMask,
			wantKind:  DerivationSetInvalidToken,
			wantToken: "extension\u00a0restriction",
		},
		{
			name:    "all",
			value:   derivationSetAllToken,
			allowed: DerivationFinalDefaultMask,
			want:    DerivationFinalDefaultMask,
		},
		{
			name:      "all with token",
			value:     derivationSetAllToken + " " + derivationSetRestrictionToken,
			allowed:   DerivationFinalDefaultMask,
			wantKind:  DerivationSetAllCombination,
			wantToken: derivationSetRestrictionToken,
		},
		{
			name:      "repeated all",
			value:     derivationSetAllToken + " " + derivationSetAllToken,
			allowed:   DerivationFinalDefaultMask,
			wantKind:  DerivationSetAllCombination,
			wantToken: derivationSetAllToken,
		},
		{
			name:      "disallowed token",
			value:     derivationSetSubstitutionToken,
			allowed:   DerivationComplexMask,
			wantKind:  DerivationSetDisallowedToken,
			wantToken: derivationSetSubstitutionToken,
		},
		{
			name:      "invalid token",
			value:     "foo",
			allowed:   DerivationComplexMask,
			wantKind:  DerivationSetInvalidToken,
			wantToken: "foo",
		},
		{
			name:    "duplicate token is idempotent",
			value:   derivationSetExtensionToken + " " + derivationSetExtensionToken,
			allowed: DerivationComplexMask,
			want:    DerivationExtension,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, issue := ParseDerivationSet(tt.value, tt.allowed)
			if got != tt.want || issue.Kind != tt.wantKind || issue.Token != tt.wantToken {
				t.Fatalf("ParseDerivationSet(%q, %08b) = %08b, %+v; want %08b, kind %v token %q", tt.value, tt.allowed, got, issue, tt.want, tt.wantKind, tt.wantToken)
			}
		})
	}
}
