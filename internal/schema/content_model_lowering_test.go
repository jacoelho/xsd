package schema

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestAppendParticle(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	zero := occurrence{}
	repeated := occurrence{Min: 1, Max: 2}
	unbounded := occurrence{Min: 0, Unbounded: true}

	tests := []struct {
		name      string
		modelKind modelKind
		occurs    occurrence
		particle  func(occurrence) contentParticle
		wantLen   int
		wantCode  xsderrors.Code
	}{
		{name: "omits zero count", modelKind: modelSequence, occurs: zero},
		{name: "sequence accepts repeating particle", modelKind: modelSequence, occurs: repeated, wantLen: 1},
		{name: "all accepts non-repeating particle", modelKind: modelAll, occurs: one, wantLen: 1},
		{name: "all rejects finite repeated particle", modelKind: modelAll, occurs: repeated, wantCode: xsderrors.CodeSchemaOccurrence},
		{name: "all rejects unbounded particle", modelKind: modelAll, occurs: unbounded, wantCode: xsderrors.CodeSchemaOccurrence},
		{
			name:      "all rejects wildcard particle",
			modelKind: modelAll,
			occurs:    one,
			particle:  func(occurs occurrence) contentParticle { return wildcardParticle(1, occurs) },
			wantCode:  xsderrors.CodeSchemaContentModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := contentModelNode{Kind: tt.modelKind, Occurs: one}
			particle := func(occurs occurrence) contentParticle {
				return elementParticle(1, occurs)
			}
			if tt.particle != nil {
				particle = tt.particle
			}
			err := appendParticle(&model, particle(tt.occurs))
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
		parent   modelKind
		child    modelChildAdmission
		wantCode xsderrors.Code
	}{
		{
			name:   "sequence accepts element child",
			parent: modelSequence,
			child:  modelChildAdmission{Kind: modelChildElement},
		},
		{
			name:   "all accepts element child",
			parent: modelAll,
			child:  modelChildAdmission{Kind: modelChildElement},
		},
		{
			name:     "all rejects wildcard child",
			parent:   modelAll,
			child:    modelChildAdmission{Kind: modelChildWildcard},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name:     "all rejects nested model child",
			parent:   modelAll,
			child:    modelChildAdmission{Kind: modelChildModel, ModelKind: modelSequence},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name:     "sequence rejects all model child",
			parent:   modelSequence,
			child:    modelChildAdmission{Kind: modelChildModel, ModelKind: modelAll},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name:   "sequence accepts sequence model child",
			parent: modelSequence,
			child:  modelChildAdmission{Kind: modelChildModel, ModelKind: modelSequence},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateModelGroupChildAdmission(tt.parent, tt.child)
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
		want        modelKind
		wantMessage string
	}{
		{local: sequenceChild, want: modelSequence},
		{local: choiceChild, want: modelChoice},
		{local: allChild, want: modelAll},
		{local: elementChild, wantMessage: "unsupported model \"element\""},
	}
	for _, tt := range tests {
		t.Run(tt.local, func(t *testing.T) {
			t.Parallel()

			got, err := modelKindForLocal(tt.local)
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
		want        modelChildAdmission
		wantMessage string
	}{
		{local: "element", source: &schemaElementSource{}, want: modelChildAdmission{Kind: modelChildElement}},
		{local: "any", source: &schemaWildcardSource{}, want: modelChildAdmission{Kind: modelChildWildcard}},
		{local: "group", source: &schemaGroupSource{}, want: modelChildAdmission{Kind: modelChildModel}},
		{local: "sequence", source: &schemaModelSource{Kind: modelSequence}, want: modelChildAdmission{Kind: modelChildModel, ModelKind: modelSequence}},
		{local: "choice", source: &schemaModelSource{Kind: modelChoice}, want: modelChildAdmission{Kind: modelChildModel, ModelKind: modelChoice}},
		{local: "all", source: &schemaModelSource{Kind: modelAll}, want: modelChildAdmission{Kind: modelChildModel, ModelKind: modelAll}},
		{local: "attribute", source: &schemaAttributeSource{}, wantMessage: "invalid model group child \"attribute\""},
	}
	for _, tt := range tests {
		t.Run(tt.local, func(t *testing.T) {
			t.Parallel()

			doc := &schemaDocument{name: "model-child.xsd"}
			child := &schemaNode{doc: doc, id: 1, local: tt.local, semantic: schemaSemanticSource{kind: tt.source}, line: 4, column: 5}
			if tt.wantMessage != "" {
				parent := &schemaNode{doc: doc, local: "sequence", semantic: schemaSemanticSource{kind: &schemaModelSource{Kind: modelSequence}}, children: []schemaNodeID{1}, line: 2, column: 3}
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

	one := occurrence{Min: 1, Max: 1}
	optional := occurrence{Min: 0, Max: 1}
	modelRepeat := occurrence{Min: 2, Max: 3}
	modelOptionalRepeat := occurrence{Min: 0, Max: 2}
	particleRepeat := occurrence{Min: 2, Max: 3}
	a := elementParticle(1, one)
	b := elementParticle(2, one)

	tests := []struct {
		name       string
		parentKind modelKind
		child      contentModelNode
		want       bool
		wantLen    int
		wantOccurs occurrence
	}{
		{
			name:       "choice flattens exactly-one choice child",
			parentKind: modelChoice,
			child:      contentModelNode{Kind: modelChoice, Occurs: one, Particles: []contentParticle{a, b}},
			want:       true,
			wantLen:    2,
			wantOccurs: one,
		},
		{
			name:       "choice keeps repeated choice child nested",
			parentKind: modelChoice,
			child:      contentModelNode{Kind: modelChoice, Occurs: modelRepeat, Particles: []contentParticle{a}},
		},
		{
			name:       "sequence keeps repeated singleton model nested",
			parentKind: modelSequence,
			child:      contentModelNode{Kind: modelSequence, Occurs: modelRepeat, Particles: []contentParticle{a}},
		},
		{
			name:       "sequence keeps repeated model around optional particle nested",
			parentKind: modelSequence,
			child: contentModelNode{
				Kind:      modelChoice,
				Occurs:    modelRepeat,
				Particles: []contentParticle{elementParticle(1, optional)},
			},
		},
		{
			name:       "sequence flattens unit singleton preserving repeated child",
			parentKind: modelSequence,
			child:      contentModelNode{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(1, particleRepeat)}},
			want:       true,
			wantLen:    1,
			wantOccurs: particleRepeat,
		},
		{
			name:       "sequence keeps unsafe optional repeated single particle nested",
			parentKind: modelSequence,
			child: contentModelNode{
				Kind:      modelSequence,
				Occurs:    modelOptionalRepeat,
				Particles: []contentParticle{elementParticle(1, particleRepeat)},
			},
		},
		{
			name:       "sequence flattens exactly-one multi-particle sequence",
			parentKind: modelSequence,
			child:      contentModelNode{Kind: modelSequence, Occurs: one, Particles: []contentParticle{a, b}},
			want:       true,
			wantLen:    2,
			wantOccurs: one,
		},
		{
			name:       "all keeps sequence child nested",
			parentKind: modelAll,
			child:      contentModelNode{Kind: modelSequence, Occurs: one, Particles: []contentParticle{a}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := contentModelNode{Kind: tt.parentKind, Occurs: one}
			before := cloneContentModel(model)
			got := appendFlattenedModelChild(&model, tt.child)
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

	repeated := occurrence{Min: 2, Max: 3}
	rt := newContentModelLoweringRuntime([]contentModelNode{{
		Kind:   modelSequence,
		Occurs: repeated,
		Particles: []contentParticle{
			elementParticle(1, occurrence{Min: 1, Max: 1}),
		},
	}})
	p, ok, err := compileModelParticle(rt.build(), rt.addModel, 0)
	if err != nil {
		t.Fatalf("ModelParticle() error = %v", err)
	}
	if !ok || p.Kind != particleModel || p.Model != 1 || p.Occurs != repeated {
		t.Fatalf("ModelParticle() = %+v, %v; want normalized model particle", p, ok)
	}
	if len(rt.models) != 2 || !rt.models[1].Occurs.IsExactlyOne() {
		t.Fatalf("normalized model = %#v, want appended exactly-one model", rt.models)
	}

	zero := contentModelNode{Kind: modelSequence, Occurs: occurrence{}}
	rt = newContentModelLoweringRuntime([]contentModelNode{zero})
	p, ok, err = compileModelParticle(rt.build(), rt.addModel, 0)
	if err != nil || ok || p != (contentParticle{}) {
		t.Fatalf("ModelParticle(zero) = %+v, %v, %v; want no particle", p, ok, err)
	}

	rt = newContentModelLoweringRuntime(nil)
	err = appendModelParticle(rt.build(), rt.addModel, &contentModelNode{}, 0)
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestExtendSequenceModel(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	baseParticle := elementParticle(1, one)
	extParticle := elementParticle(2, one)

	rt := newContentModelLoweringRuntime([]contentModelNode{
		{Kind: modelSequence, Occurs: one, Particles: []contentParticle{baseParticle}, Mixed: true},
		{Kind: modelChoice, Occurs: one, Particles: []contentParticle{extParticle}},
	})
	id, err := extendSequenceModel(rt.build(), rt.addModel, 0, 1)
	if err != nil {
		t.Fatalf("ExtendSequenceModel() error = %v", err)
	}
	if id != 2 {
		t.Fatalf("ExtendSequenceModel() id = %d, want appended model 2", id)
	}
	got := rt.models[id]
	if got.Kind != modelSequence || !got.Mixed || len(got.Particles) != 2 {
		t.Fatalf("extended model = %#v, want mixed sequence with two particles", got)
	}
	if got.Particles[0] != baseParticle || got.Particles[1].Kind != particleModel {
		t.Fatalf("extended particles = %#v, want base particle then extension model particle", got.Particles)
	}

	rt = newContentModelLoweringRuntime([]contentModelNode{
		{Kind: modelEmpty, Occurs: one, Mixed: true},
		{Kind: modelSequence, Occurs: one, Particles: []contentParticle{extParticle}},
	})
	id, err = extendSequenceModel(rt.build(), rt.addModel, 0, 1)
	if err != nil {
		t.Fatalf("ExtendSequenceModel(empty base) error = %v", err)
	}
	if id != 2 || !rt.models[id].Mixed {
		t.Fatalf("ExtendSequenceModel(empty base) id/model = %d/%#v, want mixed copy", id, rt.models[id])
	}

	rt = newContentModelLoweringRuntime(nil)
	_, err = extendSequenceModel(rt.build(), rt.addModel, 0, 1)
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

func TestValidateComplexExtensionModelAdmission(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	empty := contentModelNode{Kind: modelEmpty, Occurs: one}
	sequence := contentModelNode{
		Kind:      modelSequence,
		Occurs:    one,
		Particles: []contentParticle{elementParticle(1, one)},
	}
	all := contentModelNode{
		Kind:      modelAll,
		Occurs:    one,
		Particles: []contentParticle{elementParticle(1, one)},
	}
	rt := complexExtensionModelFixture{models: []contentModelNode{empty, sequence, all}}
	tests := []struct {
		name      string
		admission complexExtensionModelAdmission
		wantCat   xsderrors.Category
		wantCode  xsderrors.Code
	}{
		{
			name: "valid sequence extension",
			admission: complexExtensionModelAdmission{
				BaseContent: contentModelID(0),
				Extension:   contentModelID(1),
			},
		},
		{
			name: "anyType mixed base can become element-only",
			admission: complexExtensionModelAdmission{
				BaseContent:   contentModelID(1),
				Extension:     contentModelID(1),
				BaseIsAnyType: true,
				BaseMixed:     true,
			},
		},
		{
			name: "mixed base drop",
			admission: complexExtensionModelAdmission{
				BaseContent: contentModelID(1),
				Extension:   contentModelID(1),
				BaseMixed:   true,
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "all extension with non-empty base",
			admission: complexExtensionModelAdmission{
				BaseContent: contentModelID(1),
				Extension:   contentModelID(2),
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "all base with extension particles",
			admission: complexExtensionModelAdmission{
				BaseContent: contentModelID(2),
				Extension:   contentModelID(1),
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
		{
			name: "missing extension model",
			admission: complexExtensionModelAdmission{
				BaseContent: contentModelID(0),
				Extension:   contentModelID(9),
			},
			wantCat:  xsderrors.CategoryInternal,
			wantCode: xsderrors.CodeInternalInvariant,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateComplexExtensionModelAdmission(rt.build(), tt.admission)
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
		admission complexExtensionContentAdmission
		wantCode  xsderrors.Code
	}{
		{name: "complex base with model child", admission: complexExtensionContentAdmission{HasModelChild: true}},
		{name: "simple content base without model child", admission: complexExtensionContentAdmission{BaseSimpleContent: true}},
		{
			name: "simple content base with model child",
			admission: complexExtensionContentAdmission{
				BaseSimpleContent: true,
				HasModelChild:     true,
			},
			wantCode: xsderrors.CodeSchemaContentModel,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateComplexExtensionContentAdmission(tt.admission)
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
	models []contentModelNode
}

type contentModelLoweringFixture struct {
	models []contentModelNode
}

func newContentModelLoweringRuntime(models []contentModelNode) *contentModelLoweringFixture {
	return &contentModelLoweringFixture{models: cloneContentModels(models)}
}

func cloneContentModels(models []contentModelNode) []contentModelNode {
	out := make([]contentModelNode, len(models))
	for i, model := range models {
		out[i] = cloneContentModel(model)
	}
	return out
}

func (s *contentModelLoweringFixture) addModel(model contentModelNode) (contentModelID, error) {
	id, err := nextContentModelID(len(s.models))
	if err != nil {
		return noContentModel, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model limit exceeded")
	}
	s.models = append(s.models, cloneContentModel(model))
	return id, nil
}

func (f complexExtensionModelFixture) build() *schemaBuild {
	return &schemaBuild{Models: cloneContentModels(f.models)}
}

func (s *contentModelLoweringFixture) build() *schemaBuild { return &schemaBuild{Models: s.models} }
