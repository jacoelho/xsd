package runtime

import (
	"strings"
	"testing"
)

func TestDerivationKindValidity(t *testing.T) {
	t.Parallel()

	for _, kind := range []DerivationKind{
		DerivationKindNone,
		DerivationKindRestriction,
		DerivationKindExtension,
	} {
		if !ValidDerivationKind(kind) {
			t.Fatalf("ValidDerivationKind(%d) = false", kind)
		}
	}
	if ValidDerivationKind(DerivationKind(99)) {
		t.Fatal("invalid derivation kind was accepted")
	}
}

func TestContentKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		kind   ContentKind
		mixed  bool
		simple bool
	}{
		{name: "element only", kind: ContentElementOnly},
		{name: "mixed", kind: ContentMixed, mixed: true},
		{name: "simple", kind: ContentSimple, simple: true},
		{name: "simple mixed", kind: ContentSimpleMixed, mixed: true, simple: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !ValidContentKind(tt.kind) {
				t.Fatalf("ValidContentKind(%d) = false", tt.kind)
			}
			if got := tt.kind.Mixed(); got != tt.mixed {
				t.Fatalf("Mixed() = %v, want %v", got, tt.mixed)
			}
			if got := tt.kind.Simple(); got != tt.simple {
				t.Fatalf("Simple() = %v, want %v", got, tt.simple)
			}
		})
	}
	if ValidContentKind(ContentKind(99)) {
		t.Fatal("invalid content kind was accepted")
	}
}

func TestContentKindConstructors(t *testing.T) {
	t.Parallel()

	if got := ElementContentKind(false); got != ContentElementOnly {
		t.Fatalf("ElementContentKind(false) = %d, want %d", got, ContentElementOnly)
	}
	if got := ElementContentKind(true); got != ContentMixed {
		t.Fatalf("ElementContentKind(true) = %d, want %d", got, ContentMixed)
	}
	if got := SimpleContentKind(false); got != ContentSimple {
		t.Fatalf("SimpleContentKind(false) = %d, want %d", got, ContentSimple)
	}
	if got := SimpleContentKind(true); got != ContentSimpleMixed {
		t.Fatalf("SimpleContentKind(true) = %d, want %d", got, ContentSimpleMixed)
	}
}

func TestComplexTypeContentHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		ct            ComplexType
		mixed         bool
		simpleContent bool
	}{
		{name: "element only", ct: ComplexType{ContentKind: ContentElementOnly}},
		{name: "mixed", ct: ComplexType{ContentKind: ContentMixed}, mixed: true},
		{name: "simple", ct: ComplexType{ContentKind: ContentSimple}, simpleContent: true},
		{name: "simple mixed", ct: ComplexType{ContentKind: ContentSimpleMixed}, mixed: true, simpleContent: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.ct.Mixed(); got != tt.mixed {
				t.Fatalf("Mixed() = %v, want %v", got, tt.mixed)
			}
			if got := tt.ct.SimpleContent(); got != tt.simpleContent {
				t.Fatalf("SimpleContent() = %v, want %v", got, tt.simpleContent)
			}
		})
	}
}

func TestComplexTypeByID(t *testing.T) {
	t.Parallel()

	types := []ComplexType{
		{Name: QName{Local: 1}},
		{Name: QName{Local: 2}},
	}
	got, ok := ComplexTypeByID(types, 1)
	if !ok || got.Name != types[1].Name {
		t.Fatalf("ComplexTypeByID(valid) = %+v, %v; want type 1, true", got, ok)
	}
	for _, id := range []ComplexTypeID{NoComplexType, 2} {
		got, ok := ComplexTypeByID(types, id)
		if ok || got != nil {
			t.Fatalf("ComplexTypeByID(%d) = %+v, %v; want nil, false", id, got, ok)
		}
	}
}

func TestEqualComplexAttributeUseSetIDProjection(t *testing.T) {
	t.Parallel()

	complexTypes := []ComplexType{
		{Attrs: 1},
		{Attrs: 2},
	}
	projection := NewComplexAttributeUseSetIDProjection(complexTypes)
	if !EqualComplexAttributeUseSetIDProjection(projection, complexTypes) {
		t.Fatal("NewComplexAttributeUseSetIDProjection() did not produce matching projection")
	}
	if !EqualComplexAttributeUseSetIDProjection([]AttributeUseSetID{1, 2}, complexTypes) {
		t.Fatal("EqualComplexAttributeUseSetIDProjection() rejected matching projection")
	}
	if EqualComplexAttributeUseSetIDProjection([]AttributeUseSetID{1}, complexTypes) {
		t.Fatal("EqualComplexAttributeUseSetIDProjection() accepted short projection")
	}
	if EqualComplexAttributeUseSetIDProjection([]AttributeUseSetID{1, 3}, complexTypes) {
		t.Fatal("EqualComplexAttributeUseSetIDProjection() accepted mismatched projection")
	}
	if err := ValidateComplexAttributeUseSetIDProjection(NewComplexAttributeUseSetIDProjection(complexTypes), complexTypes); err != nil {
		t.Fatalf("ValidateComplexAttributeUseSetIDProjection() error = %v", err)
	}
	if err := ValidateComplexAttributeUseSetIDProjection([]AttributeUseSetID{1}, complexTypes); err == nil || err.Error() != "complex attribute use-set projection count does not match types" {
		t.Fatalf("ValidateComplexAttributeUseSetIDProjection(short) error = %v, want count invariant", err)
	}
	if err := ValidateComplexAttributeUseSetIDProjection([]AttributeUseSetID{1, 3}, complexTypes); err == nil || err.Error() != "complex attribute use-set projection does not match type" {
		t.Fatalf("ValidateComplexAttributeUseSetIDProjection(changed) error = %v, want mismatch invariant", err)
	}
}

func TestEqualComplexContentModelIDProjection(t *testing.T) {
	t.Parallel()

	complexTypes := []ComplexType{
		{Content: 1},
		{Content: 2},
	}
	projection := NewComplexContentModelIDProjection(complexTypes)
	if !EqualComplexContentModelIDProjection(projection, complexTypes) {
		t.Fatal("NewComplexContentModelIDProjection() did not produce matching projection")
	}
	if !EqualComplexContentModelIDProjection([]ContentModelID{1, 2}, complexTypes) {
		t.Fatal("EqualComplexContentModelIDProjection() rejected matching projection")
	}
	if EqualComplexContentModelIDProjection([]ContentModelID{1}, complexTypes) {
		t.Fatal("EqualComplexContentModelIDProjection() accepted short projection")
	}
	if EqualComplexContentModelIDProjection([]ContentModelID{1, 3}, complexTypes) {
		t.Fatal("EqualComplexContentModelIDProjection() accepted mismatched projection")
	}
	if got := ContentModelForTypeByID(projection, ComplexRef(1)); got != 2 {
		t.Fatalf("ContentModelForTypeByID(complex) = %v, want 2", got)
	}
	if got := ContentModelForTypeByID(projection, SimpleRef(0)); got != NoContentModel {
		t.Fatalf("ContentModelForTypeByID(simple) = %v, want no content model", got)
	}
	if got := ContentModelForTypeByID(projection, ComplexRef(99)); got != NoContentModel {
		t.Fatalf("ContentModelForTypeByID(invalid) = %v, want no content model", got)
	}
	if err := ValidateComplexContentModelIDProjection(NewComplexContentModelIDProjection(complexTypes), complexTypes); err != nil {
		t.Fatalf("ValidateComplexContentModelIDProjection() error = %v", err)
	}
	if err := ValidateComplexContentModelIDProjection([]ContentModelID{1}, complexTypes); err == nil || err.Error() != "complex content-model projection count does not match types" {
		t.Fatalf("ValidateComplexContentModelIDProjection(short) error = %v, want count invariant", err)
	}
	if err := ValidateComplexContentModelIDProjection([]ContentModelID{1, 3}, complexTypes); err == nil || err.Error() != "complex content-model projection does not match type" {
		t.Fatalf("ValidateComplexContentModelIDProjection(changed) error = %v, want mismatch invariant", err)
	}
}

func TestValidateComplexTypeRuntime(t *testing.T) {
	t.Parallel()

	names, err := NewNameTable(8, []string{EmptyNamespaceURI}, []ExpandedName{{Local: "type"}})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	name, ok := names.LookupQName("", "type")
	if !ok {
		t.Fatal("missing type QName")
	}
	models := []ContentModel{
		{Kind: ModelEmpty},
		{Kind: ModelSequence, Occurs: Occurrence{Min: 1, Max: 1}},
	}
	limits := ComplexTypeRefLimits{
		SimpleTypeCount:      1,
		ComplexTypeCount:     2,
		AttributeUseSetCount: 1,
		AnyType:              0,
	}
	valid := ComplexType{
		Name:        name,
		Base:        ComplexRef(0),
		Content:     0,
		Attrs:       0,
		TextType:    NoSimpleType,
		ContentKind: ContentElementOnly,
		Derivation:  DerivationKindRestriction,
	}
	tests := []struct {
		name    string
		wantErr string
		ct      ComplexType
		id      ComplexTypeID
	}{
		{
			name: "valid",
			id:   1,
			ct:   valid,
		},
		{
			name: "anyType can omit base",
			id:   0,
			ct: func() ComplexType {
				ct := valid
				ct.Base = TypeID{}
				ct.Derivation = DerivationKindNone
				return ct
			}(),
		},
		{
			name: "invalid name",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.Name = QName{Namespace: 99, Local: 99}
				return ct
			}(),
			wantErr: "complex type references invalid name",
		},
		{
			name: "invalid content kind",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.ContentKind = ContentKind(99)
				return ct
			}(),
			wantErr: "complex type has invalid content kind",
		},
		{
			name: "invalid derivation kind",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.Derivation = DerivationKind(99)
				return ct
			}(),
			wantErr: "complex type has invalid derivation kind",
		},
		{
			name: "invalid block",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.Block = DerivationSubstitution
				return ct
			}(),
			wantErr: "complex type block mask contains invalid derivation",
		},
		{
			name: "invalid final",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.Final = DerivationList
				return ct
			}(),
			wantErr: "complex type final mask contains invalid derivation",
		},
		{
			name: "explicit derivation needs kind",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.Derivation = DerivationKindNone
				ct.ExplicitDerivation = true
				return ct
			}(),
			wantErr: "complex type marks explicit derivation without derivation kind",
		},
		{
			name: "non-anyType needs base",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.Base = TypeID{}
				return ct
			}(),
			wantErr: "complex type has no base type",
		},
		{
			name: "invalid base",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.Base = ComplexRef(2)
				return ct
			}(),
			wantErr: "complex type references invalid base",
		},
		{
			name: "invalid content model",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.Content = 2
				return ct
			}(),
			wantErr: "complex type references invalid content model",
		},
		{
			name: "invalid attribute use set",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.Attrs = 1
				return ct
			}(),
			wantErr: "complex type references invalid attribute use set",
		},
		{
			name: "text type without simple content",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.TextType = 0
				return ct
			}(),
			wantErr: "complex type stores text type without simple content",
		},
		{
			name: "simple content invalid text type",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.ContentKind = ContentSimple
				ct.TextType = 1
				return ct
			}(),
			wantErr: "complex type references invalid text type",
		},
		{
			name: "simple content requires empty model",
			id:   1,
			ct: func() ComplexType {
				ct := valid
				ct.ContentKind = ContentSimple
				ct.Content = 1
				ct.TextType = 0
				return ct
			}(),
			wantErr: "complex type simple content must have empty content model",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexTypeRuntime(&names, tt.id, tt.ct, models, limits)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateComplexTypeRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateComplexTypeRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateComplexTypeDerivationRuntime(t *testing.T) {
	t.Parallel()

	const (
		anyType ComplexTypeID = 0
		id      ComplexTypeID = 1
		baseID  ComplexTypeID = 2
	)
	tests := []struct {
		name    string
		wantErr string
		ct      ComplexType
		id      ComplexTypeID
	}{
		{
			name: "anyType has no derivation requirement",
			id:   anyType,
			ct:   ComplexType{},
		},
		{
			name: "simple base derivation is validated by simple-base rule",
			id:   id,
			ct: ComplexType{
				Base: SimpleRef(0),
			},
		},
		{
			name: "explicit complex extension",
			id:   id,
			ct: ComplexType{
				Base:               ComplexRef(baseID),
				Derivation:         DerivationKindExtension,
				ExplicitDerivation: true,
			},
		},
		{
			name: "complex extension must be explicit",
			id:   id,
			ct: ComplexType{
				Base:       ComplexRef(baseID),
				Derivation: DerivationKindExtension,
			},
			wantErr: "complex extension is not marked explicit",
		},
		{
			name: "implicit anyType restriction is allowed",
			id:   id,
			ct: ComplexType{
				Base:       ComplexRef(anyType),
				Derivation: DerivationKindRestriction,
			},
		},
		{
			name: "complex restriction must be explicit except anyType",
			id:   id,
			ct: ComplexType{
				Base:       ComplexRef(baseID),
				Derivation: DerivationKindRestriction,
			},
			wantErr: "complex restriction is not marked explicit",
		},
		{
			name: "non-anyType cannot omit derivation",
			id:   id,
			ct: ComplexType{
				Base: ComplexRef(baseID),
			},
			wantErr: "non-anyType complex type has no derivation",
		},
		{
			name: "invalid derivation kind",
			id:   id,
			ct: ComplexType{
				Base:       ComplexRef(baseID),
				Derivation: DerivationKind(99),
			},
			wantErr: "complex type has invalid derivation kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexTypeDerivationRuntime(anyType, tt.id, tt.ct)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateComplexTypeDerivationRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateComplexTypeDerivationRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestComplexTypeDerivationBaseID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		count   int
		base    TypeID
		want    ComplexTypeID
	}{
		{
			name:  "valid complex base",
			base:  ComplexRef(1),
			count: 2,
			want:  1,
		},
		{
			name:    "simple base",
			base:    SimpleRef(0),
			count:   2,
			want:    NoComplexType,
			wantErr: "complex type derivation references non-complex base",
		},
		{
			name:    "zero type",
			base:    TypeID{},
			count:   2,
			want:    NoComplexType,
			wantErr: "complex type derivation references non-complex base",
		},
		{
			name:    "complex base out of range",
			base:    ComplexRef(2),
			count:   2,
			want:    NoComplexType,
			wantErr: "complex type derivation references non-complex base",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ComplexTypeDerivationBaseID(tt.base, tt.count)
			if got != tt.want {
				t.Fatalf("ComplexTypeDerivationBaseID() id = %d, want %d", got, tt.want)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ComplexTypeDerivationBaseID() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ComplexTypeDerivationBaseID() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateComplexTypeFinalAllows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wantErr    string
		final      DerivationMask
		derivation DerivationMask
	}{
		{
			name:       "extension allowed",
			derivation: DerivationExtension,
		},
		{
			name:       "restriction allowed",
			derivation: DerivationRestriction,
		},
		{
			name:       "extension blocked",
			final:      DerivationExtension,
			derivation: DerivationExtension,
			wantErr:    "complex type extension is blocked by base final",
		},
		{
			name:       "restriction blocked",
			final:      DerivationRestriction,
			derivation: DerivationRestriction,
			wantErr:    "complex type restriction is blocked by base final",
		},
		{
			name:       "invalid final mask",
			final:      DerivationList,
			derivation: DerivationExtension,
			wantErr:    "complex type final mask contains invalid derivation",
		},
		{
			name:       "invalid derivation",
			derivation: DerivationList,
			wantErr:    "complex type final derivation is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexTypeFinalAllows(tt.final, tt.derivation)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateComplexTypeFinalAllows() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateComplexTypeFinalAllows() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSimpleBaseComplexExtensionFinalAllows(t *testing.T) {
	t.Parallel()

	if err := ValidateSimpleBaseComplexExtensionFinalAllows(0); err != nil {
		t.Fatalf("ValidateSimpleBaseComplexExtensionFinalAllows() error = %v", err)
	}
	err := ValidateSimpleBaseComplexExtensionFinalAllows(DerivationExtension)
	if err == nil || err.Error() != "complex type extension is blocked by simple base final" {
		t.Fatalf("ValidateSimpleBaseComplexExtensionFinalAllows() error = %v, want blocked extension", err)
	}
}

func TestValidateComplexTypeExtensionRuntime(t *testing.T) {
	t.Parallel()

	const (
		anyType         ComplexTypeID  = 0
		baseType        ComplexTypeID  = 1
		emptyID         ContentModelID = 0
		baseSeqID       ContentModelID = 1
		derivedSeqID    ContentModelID = 2
		badDerivedSeqID ContentModelID = 3
		elemA           ElementID      = 0
		elemB           ElementID      = 1
	)
	one := Occurrence{Min: 1, Max: 1}
	rt := testParticleRuntime{
		models: []ContentModel{
			{Kind: ModelEmpty},
			{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{ElementParticle(elemA, one)},
			},
			{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{ElementParticle(elemA, one), ElementParticle(elemB, one)},
			},
			{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{ElementParticle(elemB, one), ElementParticle(elemA, one)},
			},
		},
	}
	base := ComplexType{
		Content:     baseSeqID,
		TextType:    NoSimpleType,
		ContentKind: ContentElementOnly,
	}
	derived := ComplexType{
		Base:        ComplexRef(baseType),
		Content:     derivedSeqID,
		TextType:    NoSimpleType,
		ContentKind: ContentElementOnly,
		Derivation:  DerivationKindExtension,
	}
	tests := []struct {
		name    string
		wantErr string
		base    ComplexType
		derived ComplexType
	}{
		{
			name:    "valid complex content extension",
			base:    base,
			derived: derived,
		},
		{
			name: "base final blocks extension",
			base: func() ComplexType {
				ct := base
				ct.Final = DerivationExtension
				return ct
			}(),
			derived: derived,
			wantErr: "complex type extension is blocked by base final",
		},
		{
			name: "valid simple content extension",
			base: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
			},
			derived: ComplexType{
				Base:        ComplexRef(baseType),
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
				Derivation:  DerivationKindExtension,
			},
		},
		{
			name: "simple content base requires simple content derived",
			base: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
			},
			derived: derived,
			wantErr: "complex simple-content extension shape does not match base",
		},
		{
			name: "simple content text type must match",
			base: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
			},
			derived: ComplexType{
				Base:        ComplexRef(baseType),
				Content:     emptyID,
				TextType:    1,
				ContentKind: ContentSimple,
				Derivation:  DerivationKindExtension,
			},
			wantErr: "complex simple-content extension shape does not match base",
		},
		{
			name: "simple content must keep empty model",
			base: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
			},
			derived: ComplexType{
				Base:        ComplexRef(baseType),
				Content:     derivedSeqID,
				TextType:    0,
				ContentKind: ContentSimple,
				Derivation:  DerivationKindExtension,
			},
			wantErr: "complex simple-content extension shape does not match base",
		},
		{
			name: "complex base rejects simple content derived",
			base: base,
			derived: func() ComplexType {
				ct := derived
				ct.Content = emptyID
				ct.TextType = 0
				ct.ContentKind = ContentSimple
				return ct
			}(),
			wantErr: "complex extension changes element content to simple content",
		},
		{
			name: "extension cannot drop mixed content",
			base: func() ComplexType {
				ct := base
				ct.ContentKind = ContentMixed
				return ct
			}(),
			derived: derived,
			wantErr: "complex extension drops mixed base content",
		},
		{
			name: "anyType extension can be non-mixed",
			base: func() ComplexType {
				ct := base
				ct.ContentKind = ContentMixed
				return ct
			}(),
			derived: func() ComplexType {
				ct := derived
				ct.Base = ComplexRef(anyType)
				return ct
			}(),
		},
		{
			name: "derived content must preserve base",
			base: base,
			derived: func() ComplexType {
				ct := derived
				ct.Content = badDerivedSeqID
				return ct
			}(),
			wantErr: "complex extension content does not preserve base content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexTypeExtensionRuntime(rt, tt.base, tt.derived, anyType)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateComplexTypeExtensionRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateComplexTypeExtensionRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

type complexTypeValidationRuntime struct {
	testParticleRuntime
	simpleFinal []DerivationMask
	derivationRuntimeStub
}

func (rt complexTypeValidationRuntime) SimpleTypeFinal(id SimpleTypeID) (DerivationMask, bool) {
	if !ValidUint32Index(uint32(id), len(rt.simpleFinal)) {
		return 0, false
	}
	return rt.simpleFinal[id], true
}

func TestValidateComplexTypeSimpleBaseExtensionRuntime(t *testing.T) {
	t.Parallel()

	const (
		emptyID     ContentModelID = 0
		seqID       ContentModelID = 1
		baseSimple  SimpleTypeID   = 0
		otherSimple SimpleTypeID   = 1
	)
	one := Occurrence{Min: 1, Max: 1}
	rt := complexTypeValidationRuntime{
		testParticleRuntime: testParticleRuntime{
			models: []ContentModel{
				{Kind: ModelEmpty},
				{
					Kind:      ModelSequence,
					Occurs:    one,
					Particles: []Particle{ElementParticle(0, one)},
				},
			},
		},
		simpleFinal: []DerivationMask{0, DerivationExtension},
	}
	valid := ComplexType{
		Content:            emptyID,
		TextType:           baseSimple,
		ContentKind:        ContentSimple,
		Derivation:         DerivationKindExtension,
		ExplicitDerivation: true,
	}
	tests := []struct {
		name    string
		wantErr string
		derived ComplexType
		base    SimpleTypeID
	}{
		{
			name:    "valid simple-base extension",
			base:    baseSimple,
			derived: valid,
		},
		{
			name: "must be extension",
			base: baseSimple,
			derived: func() ComplexType {
				ct := valid
				ct.Derivation = DerivationKindRestriction
				return ct
			}(),
			wantErr: "complex type with simple base is not an extension",
		},
		{
			name: "must be explicit",
			base: baseSimple,
			derived: func() ComplexType {
				ct := valid
				ct.ExplicitDerivation = false
				return ct
			}(),
			wantErr: "complex simple-base extension is not marked explicit",
		},
		{
			name:    "invalid simple base",
			base:    99,
			derived: valid,
			wantErr: "complex type references invalid simple base",
		},
		{
			name: "simple base final blocks extension",
			base: otherSimple,
			derived: func() ComplexType {
				ct := valid
				ct.TextType = otherSimple
				return ct
			}(),
			wantErr: "complex type extension is blocked by simple base final",
		},
		{
			name: "must use simple content",
			base: baseSimple,
			derived: func() ComplexType {
				ct := valid
				ct.ContentKind = ContentElementOnly
				ct.TextType = NoSimpleType
				return ct
			}(),
			wantErr: "complex type simple-base extension shape is invalid",
		},
		{
			name: "text type must match base",
			base: baseSimple,
			derived: func() ComplexType {
				ct := valid
				ct.TextType = otherSimple
				return ct
			}(),
			wantErr: "complex type simple-base extension shape is invalid",
		},
		{
			name: "simple content must use empty content model",
			base: baseSimple,
			derived: func() ComplexType {
				ct := valid
				ct.Content = seqID
				return ct
			}(),
			wantErr: "complex type simple-base extension shape is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexTypeSimpleBaseExtensionRuntime(rt, tt.base, tt.derived)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateComplexTypeSimpleBaseExtensionRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateComplexTypeSimpleBaseExtensionRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateComplexTypeRestrictionRuntime(t *testing.T) {
	t.Parallel()

	const (
		emptyID      ContentModelID = 0
		baseSeqID    ContentModelID = 1
		derivedSeqID ContentModelID = 2
	)
	one := Occurrence{Min: 1, Max: 1}
	rt := complexTypeValidationRuntime{
		testParticleRuntime: testParticleRuntime{
			models: []ContentModel{
				{Kind: ModelEmpty},
				{
					Kind:      ModelSequence,
					Occurs:    one,
					Particles: []Particle{ElementParticle(0, one)},
				},
				{
					Kind:      ModelSequence,
					Occurs:    one,
					Particles: []Particle{ElementParticle(1, one)},
				},
			},
		},
		derivationRuntimeStub: derivationRuntimeStub{
			simple: []SimpleTypeDerivation{
				{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
				{Base: 0, Variety: SimpleVarietyAtomic},
			},
		},
	}
	base := ComplexType{
		Content:     baseSeqID,
		TextType:    NoSimpleType,
		ContentKind: ContentElementOnly,
	}
	derived := ComplexType{
		Content:     derivedSeqID,
		TextType:    NoSimpleType,
		ContentKind: ContentElementOnly,
		Derivation:  DerivationKindRestriction,
	}
	tests := []struct {
		name    string
		wantErr string
		base    ComplexType
		derived ComplexType
	}{
		{
			name:    "valid complex content restriction leaves particle traversal to caller",
			base:    base,
			derived: derived,
		},
		{
			name: "base final blocks restriction",
			base: func() ComplexType {
				ct := base
				ct.Final = DerivationRestriction
				return ct
			}(),
			derived: derived,
			wantErr: "complex type restriction is blocked by base final",
		},
		{
			name: "valid simple content restricts simple base",
			base: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
			},
			derived: ComplexType{
				Content:     emptyID,
				TextType:    1,
				ContentKind: ContentSimple,
				Derivation:  DerivationKindRestriction,
			},
		},
		{
			name: "simple content text type must derive from base",
			base: ComplexType{
				Content:     emptyID,
				TextType:    1,
				ContentKind: ContentSimple,
			},
			derived: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
				Derivation:  DerivationKindRestriction,
			},
			wantErr: "simpleContent restriction type is not derived from base",
		},
		{
			name: "simple content can restrict mixed emptiable complex base",
			base: ComplexType{
				Content:     emptyID,
				TextType:    NoSimpleType,
				ContentKind: ContentMixed,
			},
			derived: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
				Derivation:  DerivationKindRestriction,
			},
		},
		{
			name: "simple content requires simple or emptiable mixed base",
			base: ComplexType{
				Content:     emptyID,
				TextType:    NoSimpleType,
				ContentKind: ContentElementOnly,
			},
			derived: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
				Derivation:  DerivationKindRestriction,
			},
			wantErr: "complex simple-content restriction base is not simple or emptiable mixed",
		},
		{
			name: "complex content cannot drop simple base",
			base: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
			},
			derived: derived,
			wantErr: "complex content restriction drops simple base content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexTypeRestrictionRuntime(rt, tt.base, tt.derived)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateComplexTypeRestrictionRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateComplexTypeRestrictionRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSimpleContentRestrictionTextType(t *testing.T) {
	t.Parallel()

	rt := derivationRuntimeStub{
		simple: []SimpleTypeDerivation{
			{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
			{Base: 0, Variety: SimpleVarietyAtomic},
			{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
		},
	}
	tests := []struct {
		name    string
		wantErr string
		derived SimpleTypeID
		base    SimpleTypeID
	}{
		{name: "mixed base has no text type", derived: 2, base: NoSimpleType},
		{name: "derived text type restricts base", derived: 1, base: 0},
		{name: "same text type", derived: 0, base: 0},
		{name: "unrelated text type", derived: 2, base: 0, wantErr: "simpleContent restriction type is not derived from base"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleContentRestrictionTextType(rt, tt.derived, tt.base)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleContentRestrictionTextType() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSimpleContentRestrictionTextType() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestSimpleContentDerivationBaseAllowed(t *testing.T) {
	t.Parallel()

	const (
		emptyID ContentModelID = iota
		seqID
	)
	rt := testParticleRuntime{
		models: []ContentModel{
			{Kind: ModelEmpty},
			{
				Kind:      ModelSequence,
				Occurs:    Occurrence{Min: 1, Max: 1},
				Particles: []Particle{ElementParticle(0, Occurrence{Min: 1, Max: 1})},
			},
		},
	}
	tests := []struct {
		name        string
		base        ComplexType
		restriction bool
		want        bool
	}{
		{
			name: "simple base",
			base: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
			},
			want: true,
		},
		{
			name: "emptiable mixed restriction",
			base: ComplexType{
				Content:     emptyID,
				ContentKind: ContentMixed,
			},
			restriction: true,
			want:        true,
		},
		{
			name: "emptiable mixed extension",
			base: ComplexType{
				Content:     emptyID,
				ContentKind: ContentMixed,
			},
		},
		{
			name: "non-emptiable mixed restriction",
			base: ComplexType{
				Content:     seqID,
				ContentKind: ContentMixed,
			},
			restriction: true,
		},
		{
			name: "element-only restriction",
			base: ComplexType{
				Content:     emptyID,
				ContentKind: ContentElementOnly,
			},
			restriction: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := SimpleContentDerivationBaseAllowed(rt, tt.base, tt.restriction); got != tt.want {
				t.Fatalf("SimpleContentDerivationBaseAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComplexContentMixedDerivationBaseAllowed(t *testing.T) {
	t.Parallel()

	const (
		emptyID ContentModelID = iota
		seqID
	)
	rt := testParticleRuntime{
		models: []ContentModel{
			{Kind: ModelEmpty},
			{
				Kind:      ModelSequence,
				Occurs:    Occurrence{Min: 1, Max: 1},
				Particles: []Particle{ElementParticle(0, Occurrence{Min: 1, Max: 1})},
			},
		},
	}
	tests := []struct {
		name      string
		base      ComplexType
		extension bool
		want      bool
	}{
		{
			name: "mixed base",
			base: ComplexType{
				Content:     seqID,
				ContentKind: ContentMixed,
			},
			want: true,
		},
		{
			name: "simple mixed base",
			base: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimpleMixed,
			},
			want: true,
		},
		{
			name: "empty element-only extension",
			base: ComplexType{
				Content:     emptyID,
				ContentKind: ContentElementOnly,
			},
			extension: true,
			want:      true,
		},
		{
			name: "empty element-only restriction",
			base: ComplexType{
				Content:     emptyID,
				ContentKind: ContentElementOnly,
			},
		},
		{
			name: "non-empty element-only extension",
			base: ComplexType{
				Content:     seqID,
				ContentKind: ContentElementOnly,
			},
			extension: true,
		},
		{
			name: "simple base without mixed",
			base: ComplexType{
				Content:     emptyID,
				TextType:    0,
				ContentKind: ContentSimple,
			},
			extension: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ComplexContentMixedDerivationBaseAllowed(rt, tt.base, tt.extension); got != tt.want {
				t.Fatalf("ComplexContentMixedDerivationBaseAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateComplexContentMixedDerivationBase(t *testing.T) {
	t.Parallel()

	const (
		emptyID ContentModelID = iota
		seqID
	)
	rt := testParticleRuntime{
		models: []ContentModel{
			{Kind: ModelEmpty},
			{
				Kind:      ModelSequence,
				Occurs:    Occurrence{Min: 1, Max: 1},
				Particles: []Particle{ElementParticle(0, Occurrence{Min: 1, Max: 1})},
			},
		},
	}
	tests := []struct {
		name      string
		wantErr   string
		base      ComplexType
		extension bool
		mixed     bool
	}{
		{
			name: "not mixed",
			base: ComplexType{
				Content:     seqID,
				ContentKind: ContentElementOnly,
			},
		},
		{
			name: "mixed base",
			base: ComplexType{
				Content:     seqID,
				ContentKind: ContentMixed,
			},
			mixed: true,
		},
		{
			name: "empty element-only extension",
			base: ComplexType{
				Content:     emptyID,
				ContentKind: ContentElementOnly,
			},
			extension: true,
			mixed:     true,
		},
		{
			name: "non-empty element-only extension",
			base: ComplexType{
				Content:     seqID,
				ContentKind: ContentElementOnly,
			},
			extension: true,
			mixed:     true,
			wantErr:   "complexContent mixed derivation requires mixed base",
		},
		{
			name: "empty element-only restriction",
			base: ComplexType{
				Content:     emptyID,
				ContentKind: ContentElementOnly,
			},
			mixed:   true,
			wantErr: "complexContent mixed derivation requires mixed base",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexContentMixedDerivationBase(rt, tt.base, tt.extension, tt.mixed)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateComplexContentMixedDerivationBase() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateComplexContentMixedDerivationBase() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
