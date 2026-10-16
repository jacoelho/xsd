package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestOrderedBoundAcceptanceContract(t *testing.T) {
	for _, test := range []struct {
		name         string
		exclusive    bool
		relation     value.OrderedFacetRelation
		lower, upper bool
	}{
		{name: "inclusive less", exclusive: false, relation: value.OrderedFacetLess, lower: false, upper: true},
		{name: "inclusive equal", exclusive: false, relation: value.OrderedFacetEqual, lower: true, upper: true},
		{name: "inclusive greater", exclusive: false, relation: value.OrderedFacetGreater, lower: true, upper: false},
		{name: "inclusive incomparable", exclusive: false, relation: value.OrderedFacetIncomparable, lower: false, upper: false},
		{name: "exclusive less", exclusive: true, relation: value.OrderedFacetLess, lower: false, upper: true},
		{name: "exclusive equal", exclusive: true, relation: value.OrderedFacetEqual, lower: false, upper: false},
		{name: "exclusive greater", exclusive: true, relation: value.OrderedFacetGreater, lower: true, upper: false},
		{name: "exclusive incomparable", exclusive: true, relation: value.OrderedFacetIncomparable, lower: false, upper: false},
		{name: "invalid relation 4 inclusive", relation: value.OrderedFacetRelation(4)},
		{name: "invalid relation 4 exclusive", exclusive: true, relation: value.OrderedFacetRelation(4)},
		{name: "invalid relation 255 inclusive", relation: value.OrderedFacetRelation(255)},
		{name: "invalid relation 255 exclusive", exclusive: true, relation: value.OrderedFacetRelation(255)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := value.OrderedFacetLowerBoundAccepts(test.exclusive, test.relation); got != test.lower {
				t.Fatalf("LowerBoundAccepts(%v, %v) = %v, want %v", test.exclusive, test.relation, got, test.lower)
			}
			if got := value.OrderedFacetUpperBoundAccepts(test.exclusive, test.relation); got != test.upper {
				t.Fatalf("UpperBoundAccepts(%v, %v) = %v, want %v", test.exclusive, test.relation, got, test.upper)
			}
		})
	}
}
