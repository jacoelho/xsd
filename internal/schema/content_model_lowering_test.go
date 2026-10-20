package schema

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestAppendParticle(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	zero := Occurrence{}
	repeated := Occurrence{Min: 1, Max: 2}
	unbounded := Occurrence{Min: 0, Unbounded: true}

	tests := []struct {
		name      string
		modelKind ModelKind
		occurs    Occurrence
		particle  func(Occurrence) Particle
		wantLen   int
		wantCode  xsderrors.Code
	}{
		{name: "omits zero count", modelKind: ModelSequence, occurs: zero},
		{name: "sequence accepts repeating particle", modelKind: ModelSequence, occurs: repeated, wantLen: 1},
		{name: "all accepts non-repeating particle", modelKind: ModelAll, occurs: one, wantLen: 1},
		{name: "all rejects finite repeated particle", modelKind: ModelAll, occurs: repeated, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "all rejects unbounded particle", modelKind: ModelAll, occurs: unbounded, wantCode: xsderrors.CodeSchemaOccurrence},
		{
			name:      "all rejects wildcard particle",
			modelKind: ModelAll,
			occurs:    one,
			particle:  func(occurs Occurrence) Particle { return WildcardParticle(1, occurs) },
			wantCode:  xsderrors.CodeSchemaContentModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := ContentModel{Kind: tt.modelKind, Occurs: one}
			particle := func(occurs Occurrence) Particle {
				return ElementParticle(1, occurs)
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
		parent   ModelKind
		child    ModelChildAdmission
		wantCode xsderrors.Code
	}{
		{
			name:   "sequence accepts element child",
			parent: ModelSequence,
			child:  ModelChildAdmission{Kind: ModelChildElement},
		},
		{
			name:   "all accepts element child",
			parent: ModelAll,
			child:  ModelChildAdmission{Kind: ModelChildElement},
		},
		{
			name:     "all rejects wildcard child",
			parent:   ModelAll,
			child:    ModelChildAdmission{Kind: ModelChildWildcard},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name:     "all rejects nested model child",
			parent:   ModelAll,
			child:    ModelChildAdmission{Kind: ModelChildModel, ModelKind: ModelSequence},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name:     "sequence rejects all model child",
			parent:   ModelSequence,
			child:    ModelChildAdmission{Kind: ModelChildModel, ModelKind: ModelAll},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name:   "sequence accepts sequence model child",
			parent: ModelSequence,
			child:  ModelChildAdmission{Kind: ModelChildModel, ModelKind: ModelSequence},
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
		want        ModelKind
		wantMessage string
	}{
		{local: sequenceChild, want: ModelSequence},
		{local: choiceChild, want: ModelChoice},
		{local: allChild, want: ModelAll},
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

func TestModelChildAdmissionForNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		local       string
		source      schemaSemanticKind
		want        ModelChildAdmission
		wantMessage string
	}{
		{local: "element", source: &schemaElementSource{}, want: ModelChildAdmission{Kind: ModelChildElement}},
		{local: "any", source: &schemaWildcardSource{}, want: ModelChildAdmission{Kind: ModelChildWildcard}},
		{local: "group", source: &schemaGroupSource{}, want: ModelChildAdmission{Kind: ModelChildModel}},
		{local: "sequence", source: &schemaModelSource{Kind: ModelSequence}, want: ModelChildAdmission{Kind: ModelChildModel, ModelKind: ModelSequence}},
		{local: "choice", source: &schemaModelSource{Kind: ModelChoice}, want: ModelChildAdmission{Kind: ModelChildModel, ModelKind: ModelChoice}},
		{local: "all", source: &schemaModelSource{Kind: ModelAll}, want: ModelChildAdmission{Kind: ModelChildModel, ModelKind: ModelAll}},
		{local: "attribute", source: &schemaAttributeSource{}, wantMessage: "invalid model group child attribute"},
	}
	for _, tt := range tests {
		t.Run(tt.local, func(t *testing.T) {
			t.Parallel()

			doc := &schemaDocument{name: "model-child.xsd"}
			child := &schemaNode{doc: doc, id: 1, local: tt.local, semantic: schemaSemanticSource{kind: tt.source}, line: 4, column: 5}
			if tt.wantMessage != "" {
				parent := &schemaNode{doc: doc, local: "sequence", semantic: schemaSemanticSource{kind: &schemaModelSource{Kind: ModelSequence}}, children: []schemaNodeID{1}, line: 2, column: 3}
				doc.arena = []*schemaNode{parent, child}
				err := validateModelGroupSyntax(parent, Limits{})
				expectSchemaContentModelMessage(t, err, tt.wantMessage)
				diagnostic, ok := errors.AsType[*xsderrors.Error](err)
				if !ok {
					t.Fatalf("model grammar error = %T %[1]v, want xsderrors.Error", err)
				}
				if diagnostic.Path() != "model-child.xsd" || diagnostic.Line() != 4 || diagnostic.Column() != 5 {
					t.Fatalf("location = {%q %d %d}, want {model-child.xsd 4 5}", diagnostic.Path(), diagnostic.Line(), diagnostic.Column())
				}
				return
			}
			got, err := modelChildAdmissionForNode(child)
			if err != nil {
				t.Fatalf("modelChildAdmissionForNode() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("modelChildAdmissionForNode() = %#v, want %#v", got, tt.want)
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
	if diag.Category() != xsderrors.CategorySchemaCompile || diag.Code() != xsderrors.CodeSchemaContentModel || diag.Message() != message {
		t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category(), diag.Code(), diag.Message(), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel, message)
	}
}

func TestAppendFlattenedModelChild(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	optional := Occurrence{Min: 0, Max: 1}
	modelRepeat := Occurrence{Min: 2, Max: 3}
	modelOptionalRepeat := Occurrence{Min: 0, Max: 2}
	particleRepeat := Occurrence{Min: 2, Max: 3}
	a := ElementParticle(1, one)
	b := ElementParticle(2, one)

	tests := []struct {
		name       string
		parentKind ModelKind
		child      ContentModel
		want       bool
		wantLen    int
		wantOccurs Occurrence
	}{
		{
			name:       "choice flattens exactly-one choice child",
			parentKind: ModelChoice,
			child:      ContentModel{Kind: ModelChoice, Occurs: one, Particles: []Particle{a, b}},
			want:       true,
			wantLen:    2,
			wantOccurs: one,
		},
		{
			name:       "choice keeps repeated choice child nested",
			parentKind: ModelChoice,
			child:      ContentModel{Kind: ModelChoice, Occurs: modelRepeat, Particles: []Particle{a}},
		},
		{
			name:       "sequence keeps repeated singleton model nested",
			parentKind: ModelSequence,
			child:      ContentModel{Kind: ModelSequence, Occurs: modelRepeat, Particles: []Particle{a}},
		},
		{
			name:       "sequence keeps repeated model around optional particle nested",
			parentKind: ModelSequence,
			child: ContentModel{
				Kind:      ModelChoice,
				Occurs:    modelRepeat,
				Particles: []Particle{ElementParticle(1, optional)},
			},
		},
		{
			name:       "sequence flattens unit singleton preserving repeated child",
			parentKind: ModelSequence,
			child:      ContentModel{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(1, particleRepeat)}},
			want:       true,
			wantLen:    1,
			wantOccurs: particleRepeat,
		},
		{
			name:       "sequence keeps unsafe optional repeated single particle nested",
			parentKind: ModelSequence,
			child: ContentModel{
				Kind:      ModelSequence,
				Occurs:    modelOptionalRepeat,
				Particles: []Particle{ElementParticle(1, particleRepeat)},
			},
		},
		{
			name:       "sequence flattens exactly-one multi-particle sequence",
			parentKind: ModelSequence,
			child:      ContentModel{Kind: ModelSequence, Occurs: one, Particles: []Particle{a, b}},
			want:       true,
			wantLen:    2,
			wantOccurs: one,
		},
		{
			name:       "all keeps sequence child nested",
			parentKind: ModelAll,
			child:      ContentModel{Kind: ModelSequence, Occurs: one, Particles: []Particle{a}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := ContentModel{Kind: tt.parentKind, Occurs: one}
			before := CloneContentModel(model)
			got := AppendFlattenedModelChild(&model, tt.child)
			if got != tt.want {
				t.Fatalf("AppendFlattenedModelChild() = %v, want %v", got, tt.want)
			}
			if !got && !reflect.DeepEqual(model, before) {
				t.Fatalf("refused flatten changed parent: got %#v, want %#v", model, before)
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

	repeated := Occurrence{Min: 2, Max: 3}
	rt := newContentModelLoweringRuntime([]ContentModel{{
		Kind:   ModelSequence,
		Occurs: repeated,
		Particles: []Particle{
			ElementParticle(1, Occurrence{Min: 1, Max: 1}),
		},
	}})
	p, ok, err := CompileModelParticle(rt.build(), rt.addModel, 0)
	if err != nil {
		t.Fatalf("ModelParticle() error = %v", err)
	}
	if !ok || p.Kind != ParticleModel || p.Model != 1 || p.Occurs != repeated {
		t.Fatalf("ModelParticle() = %+v, %v; want normalized model particle", p, ok)
	}
	if len(rt.models) != 2 || !rt.models[1].Occurs.IsExactlyOne() {
		t.Fatalf("normalized model = %#v, want appended exactly-one model", rt.models)
	}

	zero := ContentModel{Kind: ModelSequence, Occurs: Occurrence{}}
	rt = newContentModelLoweringRuntime([]ContentModel{zero})
	p, ok, err = CompileModelParticle(rt.build(), rt.addModel, 0)
	if err != nil || ok || p != (Particle{}) {
		t.Fatalf("ModelParticle(zero) = %+v, %v, %v; want no particle", p, ok, err)
	}

	rt = newContentModelLoweringRuntime(nil)
	err = AppendModelParticle(rt.build(), rt.addModel, &ContentModel{}, 0)
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestExtendSequenceModel(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	baseParticle := ElementParticle(1, one)
	extParticle := ElementParticle(2, one)

	rt := newContentModelLoweringRuntime([]ContentModel{
		{Kind: ModelSequence, Occurs: one, Particles: []Particle{baseParticle}, Mixed: true},
		{Kind: ModelChoice, Occurs: one, Particles: []Particle{extParticle}},
	})
	id, err := ExtendSequenceModel(rt.build(), rt.addModel, 0, 1)
	if err != nil {
		t.Fatalf("ExtendSequenceModel() error = %v", err)
	}
	if id != 2 {
		t.Fatalf("ExtendSequenceModel() id = %d, want appended model 2", id)
	}
	got := rt.models[id]
	if got.Kind != ModelSequence || !got.Mixed || len(got.Particles) != 2 {
		t.Fatalf("extended model = %#v, want mixed sequence with two particles", got)
	}
	if got.Particles[0] != baseParticle || got.Particles[1].Kind != ParticleModel {
		t.Fatalf("extended particles = %#v, want base particle then extension model particle", got.Particles)
	}

	rt = newContentModelLoweringRuntime([]ContentModel{
		{Kind: ModelEmpty, Occurs: one, Mixed: true},
		{Kind: ModelSequence, Occurs: one, Particles: []Particle{extParticle}},
	})
	id, err = ExtendSequenceModel(rt.build(), rt.addModel, 0, 1)
	if err != nil {
		t.Fatalf("ExtendSequenceModel(empty base) error = %v", err)
	}
	if id != 2 || !rt.models[id].Mixed {
		t.Fatalf("ExtendSequenceModel(empty base) id/model = %d/%#v, want mixed copy", id, rt.models[id])
	}

	rt = newContentModelLoweringRuntime(nil)
	_, err = ExtendSequenceModel(rt.build(), rt.addModel, 0, 1)
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestValidateComplexExtensionModelAdmission(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	empty := ContentModel{Kind: ModelEmpty, Occurs: one}
	sequence := ContentModel{
		Kind:      ModelSequence,
		Occurs:    one,
		Particles: []Particle{ElementParticle(1, one)},
	}
	all := ContentModel{
		Kind:      ModelAll,
		Occurs:    one,
		Particles: []Particle{ElementParticle(1, one)},
	}
	rt := complexExtensionModelFixture{models: []ContentModel{empty, sequence, all}}
	tests := []struct {
		name      string
		admission ComplexExtensionModelAdmission
		wantCat   xsderrors.Category
		wantCode  xsderrors.Code
	}{
		{
			name: "valid sequence extension",
			admission: ComplexExtensionModelAdmission{
				BaseContent: ContentModelID(0),
				Extension:   ContentModelID(1),
			},
		},
		{
			name: "anyType mixed base can become element-only",
			admission: ComplexExtensionModelAdmission{
				BaseContent:   ContentModelID(1),
				Extension:     ContentModelID(1),
				BaseIsAnyType: true,
				BaseMixed:     true,
			},
		},
		{
			name: "mixed base drop",
			admission: ComplexExtensionModelAdmission{
				BaseContent: ContentModelID(1),
				Extension:   ContentModelID(1),
				BaseMixed:   true,
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "all extension with non-empty base",
			admission: ComplexExtensionModelAdmission{
				BaseContent: ContentModelID(1),
				Extension:   ContentModelID(2),
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "all base with extension particles",
			admission: ComplexExtensionModelAdmission{
				BaseContent: ContentModelID(2),
				Extension:   ContentModelID(1),
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "missing extension model",
			admission: ComplexExtensionModelAdmission{
				BaseContent: ContentModelID(0),
				Extension:   ContentModelID(9),
			},
			wantCat:  xsderrors.CategoryInternal,
			wantCode: xsderrors.CodeInternalInvariant,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexExtensionModelAdmission(rt.build(), tt.admission)
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

type complexExtensionModelFixture struct {
	models []ContentModel
}

type contentModelLoweringFixture struct {
	models []ContentModel
}

func newContentModelLoweringRuntime(models []ContentModel) *contentModelLoweringFixture {
	return &contentModelLoweringFixture{models: cloneContentModels(models)}
}

func cloneContentModels(models []ContentModel) []ContentModel {
	out := make([]ContentModel, len(models))
	for i, model := range models {
		out[i] = CloneContentModel(model)
	}
	return out
}

func (s *contentModelLoweringFixture) addModel(model ContentModel) (ContentModelID, error) {
	id, err := NextContentModelID(len(s.models))
	if err != nil {
		return NoContentModel, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model limit exceeded")
	}
	s.models = append(s.models, CloneContentModel(model))
	return id, nil
}

func (f complexExtensionModelFixture) build() *schemaBuild {
	return &schemaBuild{Models: cloneContentModels(f.models)}
}

func (s *contentModelLoweringFixture) build() *schemaBuild { return &schemaBuild{Models: s.models} }
