package schema

import "testing"

func TestContentModelAnalysisBoundsAndMemoizesFacts(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	rt := testParticleRuntime{
		models: []ContentModel{
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ModelParticle(1, one)}},
			{Kind: ModelEmpty},
		},
		elements: []QName{{Local: 1}},
	}
	spent := 0
	work := func(steps int) error {
		spent += steps
		return nil
	}
	analysis, err := NewContentModelAnalysis(rt, work)
	if err != nil {
		t.Fatal(err)
	}
	if emptiable, err := analysis.ModelEmptiable(0); err != nil || !emptiable {
		t.Fatalf("ModelEmptiable() = %v, %v; want true, nil", emptiable, err)
	}
	afterEmpty := spent
	if _, err := analysis.ModelEmptiable(0); err != nil {
		t.Fatal(err)
	}
	if spent != afterEmpty {
		t.Fatalf("memoized emptiability spent %d additional steps", spent-afterEmpty)
	}
	if _, err := analysis.ModelCountRange(0); err != nil {
		t.Fatal(err)
	}
	afterRange := spent
	if _, err := analysis.ModelCountRange(0); err != nil {
		t.Fatal(err)
	}
	if spent != afterRange {
		t.Fatalf("memoized count range spent %d additional steps", spent-afterRange)
	}

	cyclic := testParticleRuntime{models: []ContentModel{{
		Kind:      ModelSequence,
		Occurs:    one,
		Particles: []Particle{ModelParticle(0, one)},
	}}}
	cycleAnalysis := unlimitedContentModelAnalysis(cyclic)
	if _, err := cycleAnalysis.ModelEmptiable(0); err == nil {
		t.Fatal("ModelEmptiable() accepted a model cycle")
	}
	if _, err := cycleAnalysis.ModelCountRange(0); err == nil {
		t.Fatal("ModelCountRange() accepted a model cycle")
	}
	if _, err := analysis.ModelEmptiable(99); err == nil {
		t.Fatal("ModelEmptiable() accepted a missing model")
	}
	if _, err := NewContentModelAnalysis(nil, work); err == nil {
		t.Fatal("NewContentModelAnalysis() accepted nil runtime")
	}
	if _, err := NewContentModelAnalysis(rt, nil); err == nil {
		t.Fatal("NewContentModelAnalysis() accepted nil work budget")
	}
}

func TestParticleModelEmptiability(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	optional := Occurrence{Min: 0, Max: 1}
	rt := testParticleRuntime{
		models: []ContentModel{
			{Kind: ModelEmpty},
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(0, one)}},
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ModelParticle(0, one)}},
			{Kind: ModelChoice, Occurs: one, Particles: []Particle{ElementParticle(0, one), ElementParticle(1, optional)}},
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(0, optional), ElementParticle(1, one)}},
			{Kind: ModelChoice, Occurs: one, Particles: []Particle{ElementParticle(0, one)}},
		},
	}
	tests := []struct {
		name    string
		model   ContentModelID
		want    bool
		wantErr bool
	}{
		{name: "absent", model: NoContentModel, want: true},
		{name: "empty", model: 0, want: true},
		{name: "required element sequence", model: 1, want: false},
		{name: "nested empty sequence", model: 2, want: true},
		{name: "choice with empty branch", model: 3, want: true},
		{name: "sequence with required tail", model: 4, want: false},
		{name: "choice without empty branch", model: 5, want: false},
		{name: "invalid", model: 99, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := unlimitedContentModelAnalysis(rt).ModelEmptiable(tt.model)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ModelEmptiable() succeeded, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ModelEmptiable() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ModelEmptiable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParticleModelHasNoParticles(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	rt := testParticleRuntime{
		models: []ContentModel{
			{Kind: ModelEmpty},
			{Kind: ModelSequence, Occurs: one},
			{Kind: ModelAny, Mixed: true},
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(0, one)}},
		},
	}
	tests := []struct {
		name  string
		model ContentModelID
		want  bool
	}{
		{name: "absent", model: NoContentModel, want: true},
		{name: "empty", model: 0, want: true},
		{name: "empty sequence", model: 1, want: true},
		{name: "any", model: 2, want: false},
		{name: "non-empty sequence", model: 3, want: false},
		{name: "invalid", model: 99, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ModelHasNoParticles(rt, tt.model); got != tt.want {
				t.Fatalf("ModelHasNoParticles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParticleCountRange(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	optional := Occurrence{Min: 0, Max: 1}
	repeat := Occurrence{Min: 2, Max: 3}
	rt := testParticleRuntime{
		models: []ContentModel{
			{
				Kind:   ModelSequence,
				Occurs: one,
				Particles: []Particle{
					ElementParticle(0, repeat),
					ElementParticle(1, optional),
				},
			},
			{
				Kind:      ModelChoice,
				Occurs:    Occurrence{Min: 2, Max: 2},
				Particles: []Particle{ElementParticle(0, one), ElementParticle(1, repeat)},
			},
		},
	}
	tests := []struct {
		name  string
		model ContentModelID
		want  Occurrence
	}{
		{name: "sequence sums particles", model: 0, want: Occurrence{Min: 2, Max: 4}},
		{name: "choice unions then multiplies", model: 1, want: Occurrence{Min: 2, Max: 6}},
		{name: "absent", model: NoContentModel, want: Occurrence{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := unlimitedContentModelAnalysis(rt).ModelCountRange(tt.model)
			if err != nil {
				t.Fatalf("ModelCountRange() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ModelCountRange() = %+v, want %+v", got, tt.want)
			}
		})
	}
	got, err := unlimitedContentModelAnalysis(rt).ParticleCountRange(ModelParticle(0, Occurrence{Min: 2, Max: 2}))
	if err != nil {
		t.Fatalf("ParticleCountRange() error = %v", err)
	}
	if want := (Occurrence{Min: 4, Max: 8}); got != want {
		t.Fatalf("ParticleCountRange() = %+v, want %+v", got, want)
	}
}

func TestOccurrenceRangeSubset(t *testing.T) {
	t.Parallel()

	if !OccurrenceRangeSubset(Occurrence{Min: 2, Max: 3}, Occurrence{Min: 1, Max: 5}) {
		t.Fatal("narrow finite range should be subset")
	}
	if OccurrenceRangeSubset(Occurrence{Min: 0, Max: 3}, Occurrence{Min: 1, Max: 5}) {
		t.Fatal("lower minimum should not be subset")
	}
	if OccurrenceRangeSubset(Occurrence{Min: 2, Unbounded: true}, Occurrence{Min: 1, Max: 5}) {
		t.Fatal("unbounded derived range should not be subset of finite base")
	}
}

func TestParticlesOverlap(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	optional := Occurrence{Min: 0, Max: 1}
	nameA := QName{Namespace: EmptyNamespaceID, Local: 1}
	nameB := QName{Namespace: EmptyNamespaceID, Local: 2}
	nameC := QName{Namespace: EmptyNamespaceID, Local: 3}
	rt := testParticleRuntime{
		models: []ContentModel{
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(1, optional), ElementParticle(2, one)}},
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(1, one), ElementParticle(2, one)}},
		},
		elements: []QName{nameA, nameB, nameC},
		wildcards: []Wildcard{
			{Mode: WildcardLocal, Process: ProcessStrict},
			{Mode: WildcardTargetNamespace, Namespaces: []NamespaceID{1}, Process: ProcessStrict},
			{Mode: WildcardAny, Process: ProcessStrict},
		},
		substitutions: map[ElementID][]ElementID{
			0: {2},
		},
		substitutionLookup: map[ElementID]map[QName]ElementID{
			0: {nameC: 2},
		},
	}
	tests := []struct {
		name     string
		a        Particle
		b        Particle
		wantName QName
		want     bool
		wantErr  bool
	}{
		{
			name:     "same element",
			a:        ElementParticle(0, one),
			b:        ElementParticle(0, one),
			wantName: nameA,
			want:     true,
		},
		{
			name:     "substitution member",
			a:        ElementParticle(0, one),
			b:        ElementParticle(2, one),
			wantName: nameC,
			want:     true,
		},
		{
			name:     "element wildcard",
			a:        ElementParticle(0, one),
			b:        WildcardParticle(0, one),
			wantName: nameA,
			want:     true,
		},
		{
			name: "disjoint wildcards",
			a:    WildcardParticle(0, one),
			b:    WildcardParticle(1, one),
		},
		{
			name: "overlapping wildcards",
			a:    WildcardParticle(1, one),
			b:    WildcardParticle(2, one),
			want: true,
		},
		{
			name:     "sequence start skips optional",
			a:        ModelParticle(0, one),
			b:        ElementParticle(2, one),
			wantName: nameC,
			want:     true,
		},
		{
			name: "sequence start stops at required",
			a:    ModelParticle(1, one),
			b:    ElementParticle(2, one),
		},
		{
			name:    "invalid model",
			a:       ModelParticle(99, one),
			b:       ElementParticle(0, one),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotName, got, err := unlimitedContentModelAnalysis(rt).Overlap(tt.a, tt.b)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Overlap() succeeded, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Overlap() error = %v", err)
			}
			if got != tt.want || gotName != tt.wantName {
				t.Fatalf("Overlap() = (%+v, %v), want (%+v, %v)", gotName, got, tt.wantName, tt.want)
			}
		})
	}
}

type testParticleRuntime struct {
	substitutionLookup map[ElementID]map[QName]ElementID
	substitutions      map[ElementID][]ElementID
	models             []ContentModel
	elements           []QName
	wildcards          []Wildcard
}

func (rt testParticleRuntime) ContentModel(id ContentModelID) (ContentModel, bool) {
	if !ValidUint32Index(uint32(id), len(rt.models)) {
		return ContentModel{}, false
	}
	return rt.models[id], true
}

func (rt testParticleRuntime) ElementName(id ElementID) (QName, bool) {
	if !ValidUint32Index(uint32(id), len(rt.elements)) {
		return QName{}, false
	}
	return rt.elements[id], true
}

func (rt testParticleRuntime) Wildcard(id WildcardID) (Wildcard, bool) {
	if !ValidUint32Index(uint32(id), len(rt.wildcards)) {
		return Wildcard{}, false
	}
	return rt.wildcards[id], true
}

func (rt testParticleRuntime) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	for _, member := range rt.substitutions[id] {
		if !fn(member) {
			return
		}
	}
}

func (rt testParticleRuntime) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	members := rt.substitutionLookup[id]
	if members == nil {
		return NoElement, false
	}
	member, ok := members[name]
	return member, ok
}
