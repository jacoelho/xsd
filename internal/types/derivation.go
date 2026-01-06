package types

// DerivationMethod represents a derivation method
type DerivationMethod int

const (
	// DerivationExtension indicates derivation by extension.
	DerivationExtension DerivationMethod = 1 << iota
	// DerivationRestriction indicates derivation by restriction.
	DerivationRestriction
	// DerivationList indicates derivation by list.
	DerivationList
	// DerivationUnion indicates derivation by union.
	DerivationUnion
	// DerivationSubstitution indicates derivation by substitution.
	DerivationSubstitution
)

// DerivationSet represents a set of derivation methods
type DerivationSet int

// Has checks if a derivation method is in the set
func (d DerivationSet) Has(method DerivationMethod) bool {
	return int(d)&int(method) != 0
}

// Add adds a derivation method to the set
func (d DerivationSet) Add(method DerivationMethod) DerivationSet {
	return DerivationSet(int(d) | int(method))
}

// AllDerivations returns a set containing all derivation methods
func AllDerivations() DerivationSet {
	return DerivationSet(DerivationExtension | DerivationRestriction | DerivationList | DerivationUnion | DerivationSubstitution)
}

// Restriction represents a type restriction
type Restriction struct {
	Base         QName
	Facets       []any    // For simple type restrictions (contains facets.Facet instances)
	Particle     Particle // For complex content restrictions
	Attributes   []*AttributeDecl
	AttrGroups   []QName
	AnyAttribute *AnyAttribute
	SimpleType   *SimpleType // For simpleContent restrictions with nested simpleType
}

// Extension represents a type extension
type Extension struct {
	Base         QName
	Attributes   []*AttributeDecl
	AttrGroups   []QName
	Particle     Particle
	AnyAttribute *AnyAttribute
}
