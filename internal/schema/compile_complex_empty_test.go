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
	id, err := c.addModelAt(contentModelNode{Kind: modelEmpty}, source)
	if err != nil {
		t.Fatal(err)
	}
	before := len(c.rt.Models)

	got, err := c.canonicalComplexContentModel(id, contentEmpty)
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
		model contentModelNode
	}{
		{
			name: "mixed empty model",
			model: contentModelNode{
				Kind:  modelEmpty,
				Mixed: true,
			},
		},
		{
			name: "empty model with inactive slices",
			model: contentModelNode{
				Kind:      modelEmpty,
				Particles: []contentParticle{elementParticle(1, occurrence{Min: 1, Max: 1})},
			},
		},
		{
			name: "empty model with occurrence",
			model: contentModelNode{
				Kind:   modelEmpty,
				Occurs: occurrence{Min: 1, Max: 1},
			},
		},
		{
			name: "nonempty sequence",
			model: contentModelNode{
				Kind:      modelSequence,
				Occurs:    occurrence{Min: 1, Max: 1},
				Particles: []contentParticle{{Kind: particleElement, Element: 0, Occurs: occurrence{Min: 1, Max: 1}}},
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
			got, err := c.canonicalComplexContentModel(id, contentEmpty)
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
			if err := validateContentModelShape(model); err != nil {
				t.Fatalf("canonical empty model shape invalid: %v", err)
			}
			if model.Kind != modelEmpty || model.Mixed {
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
	got, err := c.canonicalComplexContentModel(noContentModel, contentEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if got == noContentModel {
		t.Fatal("canonical empty model kept absent model ID")
	}
	if len(c.rt.Models) != before+1 {
		t.Fatalf("canonical empty model count = %d, want %d", len(c.rt.Models), before+1)
	}
	model := c.rt.Models[got]
	if err := validateContentModelShape(model); err != nil {
		t.Fatalf("canonical empty model shape invalid: %v", err)
	}
	if model.Kind != modelEmpty || model.Mixed {
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
	invalid := noContentModel - 1

	if _, err := c.canonicalComplexContentModel(invalid, contentEmpty); err == nil {
		t.Fatal("canonical empty model accepted invalid model ID")
	}
	if len(c.rt.Models) != before {
		t.Fatalf("invalid model ID changed model count from %d to %d", before, len(c.rt.Models))
	}
}
