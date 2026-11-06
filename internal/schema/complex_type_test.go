package schema

import (
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func TestDerivationKindValidity(t *testing.T) {
	t.Parallel()

	for _, kind := range []derivationKind{
		derivationKindNone,
		derivationKindRestriction,
		derivationKindExtension,
	} {
		if !validDerivationKind(kind) {
			t.Fatalf("ValidDerivationKind(%d) = false", kind)
		}
	}
	if validDerivationKind(derivationKind(99)) {
		t.Fatal("invalid derivation kind was accepted")
	}
}

func TestContentKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		kind   complexContentVariety
		mixed  bool
		simple bool
	}{
		{name: "element only", kind: contentElementOnly},
		{name: "mixed", kind: contentMixed, mixed: true},
		{name: "simple", kind: contentSimple, simple: true},
		{name: "empty", kind: contentEmpty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !validContentKind(tt.kind) {
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
	if validContentKind(complexContentVariety(99)) {
		t.Fatal("invalid content kind was accepted")
	}
}

func TestComplexTypeContentHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		ct            complexTypeMetadata
		mixed         bool
		simpleContent bool
	}{
		{name: "element only", ct: complexTypeMetadata{ContentKind: contentElementOnly}},
		{name: "mixed", ct: complexTypeMetadata{ContentKind: contentMixed}, mixed: true},
		{name: "simple", ct: complexTypeMetadata{ContentKind: contentSimple}, simpleContent: true},
		{name: "empty", ct: complexTypeMetadata{ContentKind: contentEmpty}},
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

	types := []complexTypeMetadata{
		{Name: QName{Local: 1}},
		{Name: QName{Local: 2}},
	}
	got, ok := complexTypeByID(types, 1)
	if !ok || got.Name != types[1].Name {
		t.Fatalf("ComplexTypeByID(valid) = %+v, %v; want type 1, true", got, ok)
	}
	for _, id := range []ComplexTypeID{noComplexType, 2} {
		got, ok := complexTypeByID(types, id)
		if ok || got != nil {
			t.Fatalf("ComplexTypeByID(%d) = %+v, %v; want nil, false", id, got, ok)
		}
	}
}

func TestValidateComplexTypeRuntime(t *testing.T) {
	t.Parallel()

	names, err := newNameTable(8, []string{EmptyNamespaceURI}, []ExpandedName{{Local: "type"}})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	name, ok := names.LookupQName("", "type")
	if !ok {
		t.Fatal("missing type QName")
	}
	models := []contentModelNode{
		{Kind: modelEmpty},
		{Kind: modelSequence, Occurs: occurrence{Min: 1, Max: 1}},
		{Kind: modelSequence, Occurs: occurrence{Min: 1, Max: 1}, Particles: []contentParticle{elementParticle(0, occurrence{Min: 1, Max: 1})}},
		{Kind: modelEmpty, Mixed: true},
		{Kind: modelChoice, Occurs: occurrence{Min: 1, Max: 1}},
		{Kind: modelAll, Occurs: occurrence{Min: 0, Max: 1}},
	}
	limits := complexTypeRefLimits{
		SimpleTypeCount:      1,
		ComplexTypeCount:     2,
		AttributeUseSetCount: 1,
		AnyType:              0,
	}
	valid := complexTypeMetadata{
		Name:        name,
		Base:        ComplexRef(0),
		Content:     0,
		Attrs:       0,
		TextType:    NoSimpleType,
		ContentKind: contentElementOnly,
		Derivation:  derivationKindRestriction,
	}
	tests := []struct {
		name    string
		wantErr string
		ct      complexTypeMetadata
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
			ct: func() complexTypeMetadata {
				ct := valid
				ct.Base = TypeID{}
				ct.Derivation = derivationKindNone
				return ct
			}(),
		},
		{
			name: "invalid name",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.Name = QName{Namespace: 99, Local: 99}
				return ct
			}(),
			wantErr: "complex type references invalid name",
		},
		{
			name: "invalid content kind",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.ContentKind = complexContentVariety(99)
				return ct
			}(),
			wantErr: "complex type has invalid content kind",
		},
		{
			name: "invalid derivation kind",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.Derivation = derivationKind(99)
				return ct
			}(),
			wantErr: "complex type has invalid derivation kind",
		},
		{
			name: "invalid block",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.Block = derivationSubstitution
				return ct
			}(),
			wantErr: "complex type block mask contains invalid derivation",
		},
		{
			name: "invalid final",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.Final = derivationList
				return ct
			}(),
			wantErr: "complex type final mask contains invalid derivation",
		},
		{
			name: "explicit derivation needs kind",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.Derivation = derivationKindNone
				ct.ExplicitDerivation = true
				return ct
			}(),
			wantErr: "complex type marks explicit derivation without derivation kind",
		},
		{
			name: "non-anyType needs base",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.Base = TypeID{}
				return ct
			}(),
			wantErr: "complex type has no base type",
		},
		{
			name: "invalid base",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.Base = ComplexRef(2)
				return ct
			}(),
			wantErr: "complex type references invalid base",
		},
		{
			name: "invalid content model",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.Content = 6
				return ct
			}(),
			wantErr: "complex type references invalid content model",
		},
		{
			name: "invalid attribute use set",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.Attrs = 1
				return ct
			}(),
			wantErr: "complex type references invalid attribute use set",
		},
		{
			name: "text type without simple content",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.TextType = 0
				return ct
			}(),
			wantErr: "complex type stores text type without simple content",
		},
		{
			name: "empty content uses empty model",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.ContentKind = contentEmpty
				ct.Content = 0
				return ct
			}(),
		},
		{
			name: "empty content rejects element-only sequence",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.ContentKind = contentEmpty
				ct.Content = 1
				return ct
			}(),
			wantErr: "empty complex type must reference a non-mixed empty content model",
		},
		{
			name: "empty content rejects sequence with particles",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.ContentKind = contentEmpty
				ct.Content = 2
				return ct
			}(),
			wantErr: "empty complex type must reference a non-mixed empty content model",
		},
		{
			name: "empty content rejects mixed empty model",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.ContentKind = contentEmpty
				ct.Content = 3
				return ct
			}(),
			wantErr: "empty complex type must reference a non-mixed empty content model",
		},
		{
			name: "empty content rejects choice model",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.ContentKind = contentEmpty
				ct.Content = 4
				return ct
			}(),
			wantErr: "empty complex type must reference a non-mixed empty content model",
		},
		{
			name: "empty content rejects all model",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.ContentKind = contentEmpty
				ct.Content = 5
				return ct
			}(),
			wantErr: "empty complex type must reference a non-mixed empty content model",
		},
		{
			name: "simple content invalid text type",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.ContentKind = contentSimple
				ct.TextType = 1
				return ct
			}(),
			wantErr: "complex type references invalid text type",
		},
		{
			name: "simple content requires empty model",
			id:   1,
			ct: func() complexTypeMetadata {
				ct := valid
				ct.ContentKind = contentSimple
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

			err := validateComplexTypeRuntime(&names, tt.id, tt.ct, models, limits)
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

func TestValidateComplexTypeGraph(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr string
		types   []complexTypeMetadata
	}{
		{
			name: "valid forward backward and shared bases",
			types: []complexTypeMetadata{
				{Base: ComplexRef(2)},
				{Base: SimpleRef(0)},
				{},
				{Base: ComplexRef(2)},
				{Base: ComplexRef(0)},
			},
		},
		{
			name:    "invalid base",
			types:   []complexTypeMetadata{{Base: ComplexRef(1)}},
			wantErr: "complex type graph references invalid base",
		},
		{
			name:    "self cycle",
			types:   []complexTypeMetadata{{Base: ComplexRef(0)}},
			wantErr: "complex type graph contains cycle",
		},
		{
			name: "multi-node cycle",
			types: []complexTypeMetadata{
				{Base: ComplexRef(1)},
				{Base: ComplexRef(0)},
			},
			wantErr: "complex type graph contains cycle",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateComplexTypeGraph(tt.types)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateComplexTypeGraph() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("validateComplexTypeGraph() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateComplexTypeGraphHandlesDeepForwardChain(t *testing.T) {
	t.Parallel()

	const depth = 10_000
	types := make([]complexTypeMetadata, depth)
	for i := range depth - 1 {
		types[i].Base = ComplexRef(ComplexTypeID(i + 1))
	}
	types[depth-1].Base = SimpleRef(0)
	if err := validateComplexTypeGraph(types); err != nil {
		t.Fatalf("validateComplexTypeGraph() error = %v", err)
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
		ct      complexTypeMetadata
		id      ComplexTypeID
	}{
		{
			name: "anyType has no derivation requirement",
			id:   anyType,
			ct:   complexTypeMetadata{},
		},
		{
			name: "simple base derivation is validated by simple-base rule",
			id:   id,
			ct: complexTypeMetadata{
				Base: SimpleRef(0),
			},
		},
		{
			name: "explicit complex extension",
			id:   id,
			ct: complexTypeMetadata{
				Base:               ComplexRef(baseID),
				Derivation:         derivationKindExtension,
				ExplicitDerivation: true,
			},
		},
		{
			name: "complex extension must be explicit",
			id:   id,
			ct: complexTypeMetadata{
				Base:       ComplexRef(baseID),
				Derivation: derivationKindExtension,
			},
			wantErr: "complex extension is not marked explicit",
		},
		{
			name: "implicit anyType restriction is allowed",
			id:   id,
			ct: complexTypeMetadata{
				Base:       ComplexRef(anyType),
				Derivation: derivationKindRestriction,
			},
		},
		{
			name: "complex restriction must be explicit except anyType",
			id:   id,
			ct: complexTypeMetadata{
				Base:       ComplexRef(baseID),
				Derivation: derivationKindRestriction,
			},
			wantErr: "complex restriction is not marked explicit",
		},
		{
			name: "non-anyType cannot omit derivation",
			id:   id,
			ct: complexTypeMetadata{
				Base: ComplexRef(baseID),
			},
			wantErr: "non-anyType complex type has no derivation",
		},
		{
			name: "invalid derivation kind",
			id:   id,
			ct: complexTypeMetadata{
				Base:       ComplexRef(baseID),
				Derivation: derivationKind(99),
			},
			wantErr: "complex type has invalid derivation kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateComplexTypeDerivationRuntime(anyType, tt.id, tt.ct)
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
			want:    noComplexType,
			wantErr: "complex type derivation references non-complex base",
		},
		{
			name:    "zero type",
			base:    TypeID{},
			count:   2,
			want:    noComplexType,
			wantErr: "complex type derivation references non-complex base",
		},
		{
			name:    "complex base out of range",
			base:    ComplexRef(2),
			count:   2,
			want:    noComplexType,
			wantErr: "complex type derivation references non-complex base",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := complexTypeDerivationBaseID(tt.base, tt.count)
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
			final:      derivationList,
			derivation: DerivationExtension,
			wantErr:    "complex type final mask contains invalid derivation",
		},
		{
			name:       "invalid derivation",
			derivation: derivationList,
			wantErr:    "complex type final derivation is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateComplexTypeFinalAllows(tt.final, tt.derivation)
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

	if err := validateSimpleBaseComplexExtensionFinalAllows(0); err != nil {
		t.Fatalf("ValidateSimpleBaseComplexExtensionFinalAllows() error = %v", err)
	}
	err := validateSimpleBaseComplexExtensionFinalAllows(DerivationExtension)
	if err == nil || err.Error() != "complex type extension is blocked by simple base final" {
		t.Fatalf("ValidateSimpleBaseComplexExtensionFinalAllows() error = %v, want blocked extension", err)
	}
}

func TestValidateComplexTypeExtensionRuntime(t *testing.T) {
	t.Parallel()

	const (
		anyType         ComplexTypeID  = 0
		baseType        ComplexTypeID  = 1
		emptyID         contentModelID = 0
		baseSeqID       contentModelID = 1
		derivedSeqID    contentModelID = 2
		badDerivedSeqID contentModelID = 3
		elemA           ElementID      = 0
		elemB           ElementID      = 1
	)
	one := occurrence{Min: 1, Max: 1}
	rt := particleFixture{
		models: []contentModelNode{
			{Kind: modelEmpty},
			{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{elementParticle(elemA, one)},
			},
			{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{elementParticle(elemA, one), elementParticle(elemB, one)},
			},
			{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{elementParticle(elemB, one), elementParticle(elemA, one)},
			},
		},
	}
	base := complexTypeMetadata{
		Content:     baseSeqID,
		TextType:    NoSimpleType,
		ContentKind: contentElementOnly,
	}
	derived := complexTypeMetadata{
		Base:        ComplexRef(baseType),
		Content:     derivedSeqID,
		TextType:    NoSimpleType,
		ContentKind: contentElementOnly,
		Derivation:  derivationKindExtension,
	}
	tests := []struct {
		name    string
		wantErr string
		base    complexTypeMetadata
		derived complexTypeMetadata
	}{
		{
			name:    "valid complex content extension",
			base:    base,
			derived: derived,
		},
		{
			name: "base final blocks extension",
			base: func() complexTypeMetadata {
				ct := base
				ct.Final = DerivationExtension
				return ct
			}(),
			derived: derived,
			wantErr: "complex type extension is blocked by base final",
		},
		{
			name: "valid simple content extension",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
			},
			derived: complexTypeMetadata{
				Base:        ComplexRef(baseType),
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
				Derivation:  derivationKindExtension,
			},
		},
		{
			name: "simple content base requires simple content derived",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
			},
			derived: derived,
			wantErr: "complex simple-content extension shape does not match base",
		},
		{
			name: "simple content text type must match",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
			},
			derived: complexTypeMetadata{
				Base:        ComplexRef(baseType),
				Content:     emptyID,
				TextType:    1,
				ContentKind: contentSimple,
				Derivation:  derivationKindExtension,
			},
			wantErr: "complex simple-content extension shape does not match base",
		},
		{
			name: "simple content must keep empty model",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
			},
			derived: complexTypeMetadata{
				Base:        ComplexRef(baseType),
				Content:     derivedSeqID,
				TextType:    0,
				ContentKind: contentSimple,
				Derivation:  derivationKindExtension,
			},
			wantErr: "complex simple-content extension shape does not match base",
		},
		{
			name: "complex base rejects simple content derived",
			base: base,
			derived: func() complexTypeMetadata {
				ct := derived
				ct.Content = emptyID
				ct.TextType = 0
				ct.ContentKind = contentSimple
				return ct
			}(),
			wantErr: "complex extension changes element content to simple content",
		},
		{
			name: "extension cannot drop mixed content",
			base: func() complexTypeMetadata {
				ct := base
				ct.ContentKind = contentMixed
				return ct
			}(),
			derived: derived,
			wantErr: "complex extension drops mixed base content",
		},
		{
			name: "anyType extension can be non-mixed",
			base: func() complexTypeMetadata {
				ct := base
				ct.ContentKind = contentMixed
				return ct
			}(),
			derived: func() complexTypeMetadata {
				ct := derived
				ct.Base = ComplexRef(anyType)
				return ct
			}(),
		},
		{
			name: "derived content must preserve base",
			base: base,
			derived: func() complexTypeMetadata {
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

			err := validateComplexTypeExtensionRuntime(rt.build(), tt.base, tt.derived, anyType)
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

type complexTypeValidationFixture struct {
	particleFixture
	derivationFixture

	simpleFinal []DerivationMask
}

func TestValidateComplexTypeSimpleBaseExtensionRuntime(t *testing.T) {
	t.Parallel()

	const (
		emptyID     contentModelID = 0
		seqID       contentModelID = 1
		baseSimple  SimpleTypeID   = 0
		otherSimple SimpleTypeID   = 1
	)
	one := occurrence{Min: 1, Max: 1}
	rt := complexTypeValidationFixture{
		models: []contentModelNode{
			{Kind: modelEmpty},
			{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{elementParticle(0, one)},
			},
		},
		simpleFinal: []DerivationMask{0, DerivationExtension},
	}
	valid := complexTypeMetadata{
		Content:            emptyID,
		TextType:           baseSimple,
		ContentKind:        contentSimple,
		Derivation:         derivationKindExtension,
		ExplicitDerivation: true,
	}
	tests := []struct {
		name    string
		wantErr string
		derived complexTypeMetadata
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
			derived: func() complexTypeMetadata {
				ct := valid
				ct.Derivation = derivationKindRestriction
				return ct
			}(),
			wantErr: "complex type with simple base is not an extension",
		},
		{
			name: "must be explicit",
			base: baseSimple,
			derived: func() complexTypeMetadata {
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
			derived: func() complexTypeMetadata {
				ct := valid
				ct.TextType = otherSimple
				return ct
			}(),
			wantErr: "complex type extension is blocked by simple base final",
		},
		{
			name: "must use simple content",
			base: baseSimple,
			derived: func() complexTypeMetadata {
				ct := valid
				ct.ContentKind = contentElementOnly
				ct.TextType = NoSimpleType
				return ct
			}(),
			wantErr: "complex type simple-base extension shape is invalid",
		},
		{
			name: "text type must match base",
			base: baseSimple,
			derived: func() complexTypeMetadata {
				ct := valid
				ct.TextType = otherSimple
				return ct
			}(),
			wantErr: "complex type simple-base extension shape is invalid",
		},
		{
			name: "simple content must use empty content model",
			base: baseSimple,
			derived: func() complexTypeMetadata {
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

			err := validateComplexTypeSimpleBaseExtensionRuntime(rt.build(), tt.base, tt.derived)
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
		emptyID      contentModelID = 0
		baseSeqID    contentModelID = 1
		derivedSeqID contentModelID = 2
	)
	one := occurrence{Min: 1, Max: 1}
	rt := complexTypeValidationFixture{
		models: []contentModelNode{
			{Kind: modelEmpty},
			{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{elementParticle(0, one)},
			},
			{
				Kind:      modelSequence,
				Occurs:    one,
				Particles: []contentParticle{elementParticle(1, one)},
			},
		},
		simple: []simpleTypeDerivation{
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
			{Base: 0, Variety: valuepkg.Atomic},
		},
	}
	base := complexTypeMetadata{
		Content:     baseSeqID,
		TextType:    NoSimpleType,
		ContentKind: contentElementOnly,
	}
	derived := complexTypeMetadata{
		Content:     derivedSeqID,
		TextType:    NoSimpleType,
		ContentKind: contentElementOnly,
		Derivation:  derivationKindRestriction,
	}
	tests := []struct {
		name    string
		wantErr string
		base    complexTypeMetadata
		derived complexTypeMetadata
	}{
		{
			name:    "valid complex content restriction leaves particle traversal to caller",
			base:    base,
			derived: derived,
		},
		{
			name: "base final blocks restriction",
			base: func() complexTypeMetadata {
				ct := base
				ct.Final = DerivationRestriction
				return ct
			}(),
			derived: derived,
			wantErr: "complex type restriction is blocked by base final",
		},
		{
			name: "valid simple content restricts simple base",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
			},
			derived: complexTypeMetadata{
				Content:     emptyID,
				TextType:    1,
				ContentKind: contentSimple,
				Derivation:  derivationKindRestriction,
			},
		},
		{
			name: "simple content text type must derive from base",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    1,
				ContentKind: contentSimple,
			},
			derived: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
				Derivation:  derivationKindRestriction,
			},
			wantErr: "simpleContent restriction type is not derived from base",
		},
		{
			name: "simple content can restrict mixed emptiable complex base",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    NoSimpleType,
				ContentKind: contentMixed,
			},
			derived: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
				Derivation:  derivationKindRestriction,
			},
		},
		{
			name: "simple content requires simple or emptiable mixed base",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    NoSimpleType,
				ContentKind: contentElementOnly,
			},
			derived: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
				Derivation:  derivationKindRestriction,
			},
			wantErr: "complex simple-content restriction base is not simple or emptiable mixed",
		},
		{
			name: "complex content cannot drop simple base",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
			},
			derived: derived,
			wantErr: "complex content restriction drops simple base content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			analysis := unlimitedContentModelAnalysis(rt.build())
			err := validateComplexTypeRestrictionRuntime(rt.build(), analysis, tt.base, tt.derived, unboundedTypeDerivationWork)
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

	rt := derivationFixture{
		simple: []simpleTypeDerivation{
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
			{Base: 0, Variety: valuepkg.Atomic},
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
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

			err := validateSimpleContentRestrictionTextType(rt.build(), tt.derived, tt.base, unboundedTypeDerivationWork)
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
		emptyID contentModelID = iota
		seqID
	)
	rt := particleFixture{
		models: []contentModelNode{
			{Kind: modelEmpty},
			{
				Kind:      modelSequence,
				Occurs:    occurrence{Min: 1, Max: 1},
				Particles: []contentParticle{elementParticle(0, occurrence{Min: 1, Max: 1})},
			},
		},
	}
	tests := []struct {
		name       string
		base       complexTypeMetadata
		derivation derivationKind
		want       bool
	}{
		{
			name: "simple base",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
			},
			derivation: derivationKindExtension,
			want:       true,
		},
		{
			name: "emptiable mixed restriction",
			base: complexTypeMetadata{
				Content:     emptyID,
				ContentKind: contentMixed,
			},
			derivation: derivationKindRestriction,
			want:       true,
		},
		{
			name: "emptiable mixed extension",
			base: complexTypeMetadata{
				Content:     emptyID,
				ContentKind: contentMixed,
			},
			derivation: derivationKindExtension,
		},
		{
			name: "non-emptiable mixed restriction",
			base: complexTypeMetadata{
				Content:     seqID,
				ContentKind: contentMixed,
			},
			derivation: derivationKindRestriction,
		},
		{
			name: "element-only restriction",
			base: complexTypeMetadata{
				Content:     emptyID,
				ContentKind: contentElementOnly,
			},
			derivation: derivationKindRestriction,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			analysis := unlimitedContentModelAnalysis(rt.build())
			got, err := simpleContentDerivationBaseAllowed(analysis, tt.base, tt.derivation)
			if err != nil {
				t.Fatalf("SimpleContentDerivationBaseAllowed() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("SimpleContentDerivationBaseAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComplexContentMixedDerivationBaseAllowed(t *testing.T) {
	t.Parallel()

	const (
		emptyID contentModelID = iota
		seqID
	)
	tests := []struct {
		name       string
		base       complexTypeMetadata
		derivation derivationKind
		want       bool
	}{
		{
			name: "mixed base",
			base: complexTypeMetadata{
				Content:     seqID,
				ContentKind: contentMixed,
			},
			derivation: derivationKindRestriction,
			want:       true,
		},
		{
			name: "simple base restriction",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
			},
			derivation: derivationKindRestriction,
			want:       false,
		},
		{
			name: "true empty extension",
			base: complexTypeMetadata{
				Content:     emptyID,
				ContentKind: contentEmpty,
			},
			derivation: derivationKindExtension,
			want:       true,
		},
		{
			name: "normalized element-only extension",
			base: complexTypeMetadata{
				Content:     emptyID,
				ContentKind: contentElementOnly,
			},
			derivation: derivationKindExtension,
		},
		{
			name: "empty element-only restriction",
			base: complexTypeMetadata{
				Content:     emptyID,
				ContentKind: contentElementOnly,
			},
			derivation: derivationKindRestriction,
		},
		{
			name: "non-empty element-only extension",
			base: complexTypeMetadata{
				Content:     seqID,
				ContentKind: contentElementOnly,
			},
			derivation: derivationKindExtension,
		},
		{
			name: "simple base without mixed",
			base: complexTypeMetadata{
				Content:     emptyID,
				TextType:    0,
				ContentKind: contentSimple,
			},
			derivation: derivationKindExtension,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := complexContentMixedDerivationBaseAllowed(tt.base, tt.derivation); got != tt.want {
				t.Fatalf("ComplexContentMixedDerivationBaseAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateComplexContentMixedDerivationBase(t *testing.T) {
	t.Parallel()

	const (
		emptyID contentModelID = iota
		seqID
	)
	tests := []struct {
		name       string
		wantErr    string
		base       complexTypeMetadata
		derivation derivationKind
		content    complexContentVariety
	}{
		{
			name: "not mixed",
			base: complexTypeMetadata{
				Content:     seqID,
				ContentKind: contentElementOnly,
			},
			derivation: derivationKindRestriction,
			content:    contentElementOnly,
		},
		{
			name: "mixed base",
			base: complexTypeMetadata{
				Content:     seqID,
				ContentKind: contentMixed,
			},
			derivation: derivationKindRestriction,
			content:    contentMixed,
		},
		{
			name: "true empty extension",
			base: complexTypeMetadata{
				Content:     emptyID,
				ContentKind: contentEmpty,
			},
			derivation: derivationKindExtension,
			content:    contentMixed,
		},
		{
			name: "non-empty element-only extension",
			base: complexTypeMetadata{
				Content:     seqID,
				ContentKind: contentElementOnly,
			},
			derivation: derivationKindExtension,
			content:    contentMixed,
			wantErr:    "complexContent mixed derivation requires mixed base",
		},
		{
			name: "empty element-only restriction",
			base: complexTypeMetadata{
				Content:     emptyID,
				ContentKind: contentElementOnly,
			},
			derivation: derivationKindRestriction,
			content:    contentMixed,
			wantErr:    "complexContent mixed derivation requires mixed base",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateComplexContentMixedDerivationBase(tt.base, tt.derivation, tt.content)
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

func (f complexTypeValidationFixture) build() *schemaBuild {
	b := f.derivationFixture.build()
	p := f.particleFixture.build()
	b.Models = p.Models
	b.Elements = p.Elements
	b.Wildcards = p.Wildcards
	b.Substitutions = p.Substitutions
	if len(f.simpleFinal) > len(b.SimpleTypes) {
		b.SimpleTypes = append(b.SimpleTypes, make([]simpleType, len(f.simpleFinal)-len(b.SimpleTypes))...)
	}
	for i, final := range f.simpleFinal {
		b.SimpleTypes[i].Final = final
	}
	return b
}

func (emptyTypeDerivationFixture) build() *schemaBuild { return &schemaBuild{} }
