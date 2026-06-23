package compile

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestAppendParticle(t *testing.T) {
	t.Parallel()

	one := runtime.Occurrence{Min: 1, Max: 1}
	zero := runtime.Occurrence{}
	repeated := runtime.Occurrence{Min: 1, Max: 2}
	unbounded := runtime.Occurrence{Min: 0, Unbounded: true}

	tests := []struct {
		name      string
		modelKind runtime.ModelKind
		occurs    runtime.Occurrence
		particle  func(runtime.Occurrence) runtime.Particle
		wantLen   int
		wantCode  xsderrors.Code
	}{
		{name: "omits zero count", modelKind: runtime.ModelSequence, occurs: zero},
		{name: "sequence accepts repeating particle", modelKind: runtime.ModelSequence, occurs: repeated, wantLen: 1},
		{name: "all accepts non-repeating particle", modelKind: runtime.ModelAll, occurs: one, wantLen: 1},
		{name: "all rejects finite repeated particle", modelKind: runtime.ModelAll, occurs: repeated, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "all rejects unbounded particle", modelKind: runtime.ModelAll, occurs: unbounded, wantCode: xsderrors.CodeSchemaOccurrence},
		{
			name:      "all rejects wildcard particle",
			modelKind: runtime.ModelAll,
			occurs:    one,
			particle:  func(occurs runtime.Occurrence) runtime.Particle { return runtime.WildcardParticle(1, occurs) },
			wantCode:  xsderrors.CodeSchemaContentModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := runtime.ContentModel{Kind: tt.modelKind, Occurs: one}
			particle := func(occurs runtime.Occurrence) runtime.Particle {
				return runtime.ElementParticle(1, occurs)
			}
			if tt.particle != nil {
				particle = tt.particle
			}
			err := AppendParticle(&model, particle(tt.occurs))
			if tt.wantCode != "" {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("AppendParticle() error = %v", err)
			}
			if len(model.Particles) != tt.wantLen {
				t.Fatalf("particle count = %d, want %d", len(model.Particles), tt.wantLen)
			}
		})
	}
}

func TestValidateModelGroupChildAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		parent   runtime.ModelKind
		child    ModelChildAdmission
		wantCode xsderrors.Code
	}{
		{
			name:   "sequence accepts element child",
			parent: runtime.ModelSequence,
			child:  ModelChildAdmission{Kind: ModelChildElement},
		},
		{
			name:   "all accepts element child",
			parent: runtime.ModelAll,
			child:  ModelChildAdmission{Kind: ModelChildElement},
		},
		{
			name:     "all rejects wildcard child",
			parent:   runtime.ModelAll,
			child:    ModelChildAdmission{Kind: ModelChildWildcard},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name:     "all rejects nested model child",
			parent:   runtime.ModelAll,
			child:    ModelChildAdmission{Kind: ModelChildModel, ModelKind: runtime.ModelSequence},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name:     "sequence rejects all model child",
			parent:   runtime.ModelSequence,
			child:    ModelChildAdmission{Kind: ModelChildModel, ModelKind: runtime.ModelAll},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name:   "sequence accepts sequence model child",
			parent: runtime.ModelSequence,
			child:  ModelChildAdmission{Kind: ModelChildModel, ModelKind: runtime.ModelSequence},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateModelGroupChildAdmission(tt.parent, tt.child)
			if tt.wantCode != "" {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("ValidateModelGroupChildAdmission() error = %v", err)
			}
		})
	}
}

func TestModelKindForLocal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		local       string
		want        runtime.ModelKind
		wantMessage string
	}{
		{local: sequenceChild, want: runtime.ModelSequence},
		{local: choiceChild, want: runtime.ModelChoice},
		{local: allChild, want: runtime.ModelAll},
		{local: elementChild, wantMessage: "unsupported model element"},
	}
	for _, tt := range tests {
		t.Run(tt.local, func(t *testing.T) {
			t.Parallel()

			got, err := ModelKindForLocal(tt.local)
			if tt.wantMessage != "" {
				expectSchemaContentModelMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("ModelKindForLocal() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ModelKindForLocal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModelChildAdmissionForLocal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		local       string
		want        ModelChildAdmission
		wantMessage string
	}{
		{local: elementChild, want: ModelChildAdmission{Kind: ModelChildElement}},
		{local: anyChild, want: ModelChildAdmission{Kind: ModelChildWildcard}},
		{local: groupChild, want: ModelChildAdmission{Kind: ModelChildModel}},
		{local: sequenceChild, want: ModelChildAdmission{Kind: ModelChildModel, ModelKind: runtime.ModelSequence}},
		{local: choiceChild, want: ModelChildAdmission{Kind: ModelChildModel, ModelKind: runtime.ModelChoice}},
		{local: allChild, want: ModelChildAdmission{Kind: ModelChildModel, ModelKind: runtime.ModelAll}},
		{local: attributeChild, wantMessage: "invalid model group child attribute"},
	}
	for _, tt := range tests {
		t.Run(tt.local, func(t *testing.T) {
			t.Parallel()

			got, err := ModelChildAdmissionForLocal(tt.local)
			if tt.wantMessage != "" {
				expectSchemaContentModelMessage(t, err, tt.wantMessage)
				return
			}
			if err != nil {
				t.Fatalf("ModelChildAdmissionForLocal() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ModelChildAdmissionForLocal() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func expectSchemaContentModelMessage(t *testing.T, err error, message string) {
	t.Helper()
	diag, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error = %T %[1]v, want xsderrors.Error", err)
	}
	if diag.Category != xsderrors.CategorySchemaCompile || diag.Code != xsderrors.CodeSchemaContentModel || diag.Message != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category, diag.Code, diag.Message, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel, message)
	}
}

func TestAppendFlattenedModelChild(t *testing.T) {
	t.Parallel()

	one := runtime.Occurrence{Min: 1, Max: 1}
	optional := runtime.Occurrence{Min: 0, Max: 1}
	modelRepeat := runtime.Occurrence{Min: 2, Max: 3}
	modelOptionalRepeat := runtime.Occurrence{Min: 0, Max: 2}
	particleRepeat := runtime.Occurrence{Min: 2, Max: 3}
	a := runtime.ElementParticle(1, one)
	b := runtime.ElementParticle(2, one)

	tests := []struct {
		name       string
		parentKind runtime.ModelKind
		child      runtime.ContentModel
		want       bool
		wantLen    int
		wantOccurs runtime.Occurrence
	}{
		{
			name:       "choice flattens exactly-one choice child",
			parentKind: runtime.ModelChoice,
			child:      runtime.ContentModel{Kind: runtime.ModelChoice, Occurs: one, Particles: []runtime.Particle{a, b}},
			want:       true,
			wantLen:    2,
			wantOccurs: one,
		},
		{
			name:       "choice keeps repeated choice child nested",
			parentKind: runtime.ModelChoice,
			child:      runtime.ContentModel{Kind: runtime.ModelChoice, Occurs: modelRepeat, Particles: []runtime.Particle{a}},
		},
		{
			name:       "sequence multiplies single exactly-one particle by model occurrence",
			parentKind: runtime.ModelSequence,
			child:      runtime.ContentModel{Kind: runtime.ModelSequence, Occurs: modelRepeat, Particles: []runtime.Particle{a}},
			want:       true,
			wantLen:    1,
			wantOccurs: modelRepeat,
		},
		{
			name:       "sequence flattens optional single particle through repeated model",
			parentKind: runtime.ModelSequence,
			child: runtime.ContentModel{
				Kind:      runtime.ModelChoice,
				Occurs:    modelRepeat,
				Particles: []runtime.Particle{runtime.ElementParticle(1, optional)},
			},
			want:       true,
			wantLen:    1,
			wantOccurs: runtime.Occurrence{Min: 0, Max: 3},
		},
		{
			name:       "sequence keeps unsafe optional repeated single particle nested",
			parentKind: runtime.ModelSequence,
			child: runtime.ContentModel{
				Kind:      runtime.ModelSequence,
				Occurs:    modelOptionalRepeat,
				Particles: []runtime.Particle{runtime.ElementParticle(1, particleRepeat)},
			},
		},
		{
			name:       "sequence flattens exactly-one multi-particle sequence",
			parentKind: runtime.ModelSequence,
			child:      runtime.ContentModel{Kind: runtime.ModelSequence, Occurs: one, Particles: []runtime.Particle{a, b}},
			want:       true,
			wantLen:    2,
			wantOccurs: one,
		},
		{
			name:       "all keeps sequence child nested",
			parentKind: runtime.ModelAll,
			child:      runtime.ContentModel{Kind: runtime.ModelSequence, Occurs: one, Particles: []runtime.Particle{a}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := runtime.ContentModel{Kind: tt.parentKind, Occurs: one}
			got := AppendFlattenedModelChild(&model, tt.child)
			if got != tt.want {
				t.Fatalf("AppendFlattenedModelChild() = %v, want %v", got, tt.want)
			}
			if len(model.Particles) != tt.wantLen {
				t.Fatalf("particle count = %d, want %d", len(model.Particles), tt.wantLen)
			}
			if tt.wantLen > 0 && model.Particles[0].Occurs != tt.wantOccurs {
				t.Fatalf("first particle occurrence = %#v, want %#v", model.Particles[0].Occurs, tt.wantOccurs)
			}
		})
	}
}

func TestModelParticleNormalizesRepeatedModel(t *testing.T) {
	t.Parallel()

	repeated := runtime.Occurrence{Min: 2, Max: 3}
	rt := newContentModelLoweringRuntime([]runtime.ContentModel{{
		Kind:   runtime.ModelSequence,
		Occurs: repeated,
		Particles: []runtime.Particle{
			runtime.ElementParticle(1, runtime.Occurrence{Min: 1, Max: 1}),
		},
	}})
	p, ok, err := ModelParticle(rt, rt.addModel, 0)
	if err != nil {
		t.Fatalf("ModelParticle() error = %v", err)
	}
	if !ok || p.Kind != runtime.ParticleModel || p.Model != 1 || p.Occurs != repeated {
		t.Fatalf("ModelParticle() = %+v, %v; want normalized model particle", p, ok)
	}
	if len(rt.models) != 2 || !rt.models[1].Occurs.IsExactlyOne() {
		t.Fatalf("normalized model = %#v, want appended exactly-one model", rt.models)
	}

	zero := runtime.ContentModel{Kind: runtime.ModelSequence, Occurs: runtime.Occurrence{}}
	rt = newContentModelLoweringRuntime([]runtime.ContentModel{zero})
	p, ok, err = ModelParticle(rt, rt.addModel, 0)
	if err != nil || ok || p != (runtime.Particle{}) {
		t.Fatalf("ModelParticle(zero) = %+v, %v, %v; want no particle", p, ok, err)
	}

	rt = newContentModelLoweringRuntime(nil)
	err = AppendModelParticle(rt, rt.addModel, &runtime.ContentModel{}, 0)
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestExtendSequenceModel(t *testing.T) {
	t.Parallel()

	one := runtime.Occurrence{Min: 1, Max: 1}
	baseParticle := runtime.ElementParticle(1, one)
	extParticle := runtime.ElementParticle(2, one)

	rt := newContentModelLoweringRuntime([]runtime.ContentModel{
		{Kind: runtime.ModelSequence, Occurs: one, Particles: []runtime.Particle{baseParticle}, Mixed: true},
		{Kind: runtime.ModelChoice, Occurs: one, Particles: []runtime.Particle{extParticle}},
	})
	id, err := ExtendSequenceModel(rt, rt.addModel, 0, 1)
	if err != nil {
		t.Fatalf("ExtendSequenceModel() error = %v", err)
	}
	if id != 2 {
		t.Fatalf("ExtendSequenceModel() id = %d, want appended model 2", id)
	}
	got := rt.models[id]
	if got.Kind != runtime.ModelSequence || !got.Mixed || len(got.Particles) != 2 {
		t.Fatalf("extended model = %#v, want mixed sequence with two particles", got)
	}
	if got.Particles[0] != baseParticle || got.Particles[1].Kind != runtime.ParticleModel {
		t.Fatalf("extended particles = %#v, want base particle then extension model particle", got.Particles)
	}

	rt = newContentModelLoweringRuntime([]runtime.ContentModel{
		{Kind: runtime.ModelEmpty, Occurs: one, Mixed: true},
		{Kind: runtime.ModelSequence, Occurs: one, Particles: []runtime.Particle{extParticle}},
	})
	id, err = ExtendSequenceModel(rt, rt.addModel, 0, 1)
	if err != nil {
		t.Fatalf("ExtendSequenceModel(empty base) error = %v", err)
	}
	if id != 2 || !rt.models[id].Mixed {
		t.Fatalf("ExtendSequenceModel(empty base) id/model = %d/%#v, want mixed copy", id, rt.models[id])
	}

	rt = newContentModelLoweringRuntime(nil)
	_, err = ExtendSequenceModel(rt, rt.addModel, 0, 1)
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestValidateComplexExtensionModelAdmission(t *testing.T) {
	t.Parallel()

	one := runtime.Occurrence{Min: 1, Max: 1}
	empty := runtime.ContentModel{Kind: runtime.ModelEmpty, Occurs: one}
	sequence := runtime.ContentModel{
		Kind:      runtime.ModelSequence,
		Occurs:    one,
		Particles: []runtime.Particle{runtime.ElementParticle(1, one)},
	}
	all := runtime.ContentModel{
		Kind:      runtime.ModelAll,
		Occurs:    one,
		Particles: []runtime.Particle{runtime.ElementParticle(1, one)},
	}
	rt := complexExtensionModelRuntimeStub{models: []runtime.ContentModel{empty, sequence, all}}
	tests := []struct {
		name      string
		admission ComplexExtensionModelAdmission
		wantCat   xsderrors.Category
		wantCode  xsderrors.Code
	}{
		{
			name: "valid sequence extension",
			admission: ComplexExtensionModelAdmission{
				BaseContent: runtime.ContentModelID(0),
				Extension:   runtime.ContentModelID(1),
			},
		},
		{
			name: "anyType mixed base can become element-only",
			admission: ComplexExtensionModelAdmission{
				BaseContent:   runtime.ContentModelID(1),
				Extension:     runtime.ContentModelID(1),
				BaseIsAnyType: true,
				BaseMixed:     true,
			},
		},
		{
			name: "mixed base drop",
			admission: ComplexExtensionModelAdmission{
				BaseContent: runtime.ContentModelID(1),
				Extension:   runtime.ContentModelID(1),
				BaseMixed:   true,
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "all extension with non-empty base",
			admission: ComplexExtensionModelAdmission{
				BaseContent: runtime.ContentModelID(1),
				Extension:   runtime.ContentModelID(2),
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "all base with extension particles",
			admission: ComplexExtensionModelAdmission{
				BaseContent: runtime.ContentModelID(2),
				Extension:   runtime.ContentModelID(1),
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "missing extension model",
			admission: ComplexExtensionModelAdmission{
				BaseContent: runtime.ContentModelID(0),
				Extension:   runtime.ContentModelID(9),
			},
			wantCat:  xsderrors.CategoryInternal,
			wantCode: xsderrors.CodeInternalInvariant,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexExtensionModelAdmission(rt, tt.admission)
			if tt.wantCode != "" {
				wantCat := tt.wantCat
				if wantCat == "" {
					wantCat = xsderrors.CategorySchemaCompile
				}
				expectDiagnostic(t, err, wantCat, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("ValidateComplexExtensionModelAdmission() error = %v", err)
			}
		})
	}
}

func TestValidateComplexExtensionContentAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		admission ComplexExtensionContentAdmission
		wantCode  xsderrors.Code
	}{
		{name: "complex base with model child", admission: ComplexExtensionContentAdmission{HasModelChild: true}},
		{name: "simple content base without model child", admission: ComplexExtensionContentAdmission{BaseSimpleContent: true}},
		{
			name: "simple content base with model child",
			admission: ComplexExtensionContentAdmission{
				BaseSimpleContent: true,
				HasModelChild:     true,
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexExtensionContentAdmission(tt.admission)
			if tt.wantCode != "" {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("ValidateComplexExtensionContentAdmission() error = %v", err)
			}
		})
	}
}

type complexExtensionModelRuntimeStub struct {
	models []runtime.ContentModel
}

func (s complexExtensionModelRuntimeStub) ContentModel(id runtime.ContentModelID) (runtime.ContentModel, bool) {
	if !runtime.ValidUint32Index(uint32(id), len(s.models)) {
		return runtime.ContentModel{}, false
	}
	return s.models[id], true
}

func (s complexExtensionModelRuntimeStub) ElementName(runtime.ElementID) (runtime.QName, bool) {
	return runtime.QName{}, false
}

func (s complexExtensionModelRuntimeStub) Wildcard(runtime.WildcardID) (runtime.Wildcard, bool) {
	return runtime.Wildcard{}, false
}

func (s complexExtensionModelRuntimeStub) ForEachSubstitutionMember(runtime.ElementID, func(runtime.ElementID) bool) {
}

func (s complexExtensionModelRuntimeStub) SubstitutionMemberByName(runtime.ElementID, runtime.QName) (runtime.ElementID, bool) {
	return 0, false
}

type contentModelLoweringRuntime struct {
	models []runtime.ContentModel
}

func newContentModelLoweringRuntime(models []runtime.ContentModel) *contentModelLoweringRuntime {
	return &contentModelLoweringRuntime{models: runtime.CloneContentModels(models)}
}

func (s *contentModelLoweringRuntime) addModel(model runtime.ContentModel) (runtime.ContentModelID, error) {
	id, ok := runtime.NextContentModelID(len(s.models))
	if !ok {
		return runtime.NoContentModel, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model limit exceeded")
	}
	s.models = append(s.models, runtime.CloneContentModel(model))
	return id, nil
}

func (s *contentModelLoweringRuntime) ContentModel(id runtime.ContentModelID) (runtime.ContentModel, bool) {
	if !runtime.ValidContentModelID(id, len(s.models)) {
		return runtime.ContentModel{}, false
	}
	return runtime.CloneContentModel(s.models[id]), true
}

func (s *contentModelLoweringRuntime) ElementName(runtime.ElementID) (runtime.QName, bool) {
	return runtime.QName{}, false
}

func (s *contentModelLoweringRuntime) Wildcard(runtime.WildcardID) (runtime.Wildcard, bool) {
	return runtime.Wildcard{}, false
}

func (s *contentModelLoweringRuntime) ForEachSubstitutionMember(runtime.ElementID, func(runtime.ElementID) bool) {
}

func (s *contentModelLoweringRuntime) SubstitutionMemberByName(runtime.ElementID, runtime.QName) (runtime.ElementID, bool) {
	return runtime.NoElement, false
}
