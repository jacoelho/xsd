package schema

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestAppendParticle(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	zero := occurrence{}
	repeated := occurrence{Min: 1, Max: 2}

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
			err := appendParticle(testDependencyWork(), &model, particle(tt.occurs))
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

func (s *contentModelLoweringFixture) build() *schemaBuild {
	return &schemaBuild{Models: s.models, typeWork: testDependencyWork()}
}

// testDependencyWork returns an unlimited dependency budget for fixtures that
// reach the lowering functions directly.
func testDependencyWork() *workBudget {
	b := newWorkBudget(dependencyWorkBudget, math.MaxInt)
	return &b
}

// cloneContentModel deep-clones content-model metadata for test fixtures.
func cloneContentModel(in contentModelNode) contentModelNode {
	in.Particles = slices.Clone(in.Particles)
	return in
}
