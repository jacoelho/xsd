package schema

import "testing"

func TestCanonicalComplexContentModelReusesValidEmptyRow(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	source := &schemaNode{}
	id, err := c.addModelAt(ContentModel{Kind: ModelEmpty}, source)
	if err != nil {
		t.Fatal(err)
	}
	before := len(c.rt.Models)

	got, err := c.canonicalComplexContentModel(id, ContentEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("canonical empty model ID = %d, want existing %d", got, id)
	}
	if len(c.rt.Models) != before {
		t.Fatalf("canonical empty model count = %d, want %d", len(c.rt.Models), before)
	}
}

func TestCanonicalComplexContentModelAllocatesEmptyRowForNonCanonicalInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		model ContentModel
	}{
		{
			name: "mixed empty model",
			model: ContentModel{
				Kind:  ModelEmpty,
				Mixed: true,
			},
		},
		{
			name: "empty model with inactive slices",
			model: ContentModel{
				Kind:         ModelEmpty,
				ChoiceLimits: []uint32{1},
			},
		},
		{
			name: "empty model with occurrence",
			model: ContentModel{
				Kind:   ModelEmpty,
				Occurs: Occurrence{Min: 1, Max: 1},
			},
		},
		{
			name: "nonempty sequence",
			model: ContentModel{
				Kind:      ModelSequence,
				Occurs:    Occurrence{Min: 1, Max: 1},
				Particles: []Particle{{Kind: ParticleElement, Element: 0, Occurs: Occurrence{Min: 1, Max: 1}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			limits, err := NormalizeOptions(Options{})
			if err != nil {
				t.Fatal(err)
			}
			c, err := newCompiler(limits)
			if err != nil {
				t.Fatal(err)
			}
			baseSource := &schemaNode{}

			id, err := c.addModelAt(tt.model, baseSource)
			if err != nil {
				t.Fatal(err)
			}
			before := len(c.rt.Models)
			got, err := c.canonicalComplexContentModel(id, ContentEmpty)
			if err != nil {
				t.Fatal(err)
			}
			if got == id {
				t.Fatalf("canonical empty model reused non-canonical row %d", id)
			}
			if len(c.rt.Models) != before+1 {
				t.Fatalf("canonical empty model count = %d, want %d", len(c.rt.Models), before+1)
			}
			model := c.rt.Models[got]
			if err := ValidateContentModelShape(model); err != nil {
				t.Fatalf("canonical empty model shape invalid: %v", err)
			}
			if model.Kind != ModelEmpty || model.Mixed {
				t.Fatalf("canonical empty model = %#v, want inactive non-mixed ModelEmpty", model)
			}
			if c.modelSources[got] != baseSource {
				t.Fatalf("canonical empty model source = %p, want %p", c.modelSources[got], baseSource)
			}
		})
	}
}

func TestCanonicalComplexContentModelAllocatesEmptyRowForNoModel(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	before := len(c.rt.Models)
	got, err := c.canonicalComplexContentModel(NoContentModel, ContentEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if got == NoContentModel {
		t.Fatal("canonical empty model kept absent model ID")
	}
	if len(c.rt.Models) != before+1 {
		t.Fatalf("canonical empty model count = %d, want %d", len(c.rt.Models), before+1)
	}
	model := c.rt.Models[got]
	if err := ValidateContentModelShape(model); err != nil {
		t.Fatalf("canonical empty model shape invalid: %v", err)
	}
	if model.Kind != ModelEmpty || model.Mixed {
		t.Fatalf("canonical empty model = %#v, want inactive non-mixed ModelEmpty", model)
	}
}

func TestCanonicalComplexContentModelRejectsInvalidModelID(t *testing.T) {
	t.Parallel()

	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	before := len(c.rt.Models)
	invalid := NoContentModel - 1

	if _, err := c.canonicalComplexContentModel(invalid, ContentEmpty); err == nil {
		t.Fatal("canonical empty model accepted invalid model ID")
	}
	if len(c.rt.Models) != before {
		t.Fatalf("invalid model ID changed model count from %d to %d", before, len(c.rt.Models))
	}
}
