package semanticcheck

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
)

func TestValidateParticleOccursUsesSharedBoundsPolicy(t *testing.T) {
	overflow := &model.ElementDecl{
		MinOccurs: occurs.OccursFromInt(0),
		MaxOccurs: occurs.OccursFromUint64(uint64(^uint32(0)) + 1),
	}
	if err := validateParticleOccurs(overflow); err == nil || !errors.Is(err, occurs.ErrOccursOverflow) {
		t.Fatalf("expected ErrOccursOverflow, got %v", err)
	}

	minGreater := &model.ElementDecl{
		MinOccurs: occurs.OccursFromInt(2),
		MaxOccurs: occurs.OccursFromInt(1),
	}
	if err := validateParticleOccurs(minGreater); err == nil || !strings.Contains(err.Error(), "minOccurs (2) cannot be greater than maxOccurs (1)") {
		t.Fatalf("unexpected error for min>max: %v", err)
	}

	maxZero := &model.ElementDecl{
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(0),
	}
	if err := validateParticleOccurs(maxZero); err == nil || !strings.Contains(err.Error(), "maxOccurs cannot be 0 when minOccurs > 0") {
		t.Fatalf("unexpected error for max=0: %v", err)
	}
}

func TestValidateAllGroupOccurrenceUsesSharedPolicy(t *testing.T) {
	group := &model.ModelGroup{
		Kind:      model.AllGroup,
		MinOccurs: occurs.OccursFromInt(2),
		MaxOccurs: occurs.OccursFromInt(1),
	}
	if err := validateAllGroupOccurrence(group); err == nil || !strings.Contains(err.Error(), "xs:all must have minOccurs='0' or '1' (got 2)") {
		t.Fatalf("unexpected minOccurs error: %v", err)
	}

	group.MinOccurs = occurs.OccursFromInt(0)
	group.MaxOccurs = occurs.OccursFromInt(2)
	if err := validateAllGroupOccurrence(group); err == nil || !strings.Contains(err.Error(), "xs:all must have maxOccurs='1' (got 2)") {
		t.Fatalf("unexpected maxOccurs error: %v", err)
	}
}

func TestValidateAllGroupParticleOccursUsesSharedPolicy(t *testing.T) {
	particles := []model.Particle{
		&model.ElementDecl{
			MinOccurs: occurs.OccursFromInt(1),
			MaxOccurs: occurs.OccursFromInt(2),
		},
	}

	if err := validateAllGroupParticleOccurs(particles); err == nil || !strings.Contains(err.Error(), "xs:all: all particles must have maxOccurs <= 1 (got 2)") {
		t.Fatalf("unexpected particle maxOccurs error: %v", err)
	}
}
