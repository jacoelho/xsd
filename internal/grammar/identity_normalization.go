package grammar

import "github.com/jacoelho/xsd/internal/types"

// IdentityNormalizationKind describes how identity values should be normalized.
type IdentityNormalizationKind int

const (
	IdentityNormalizationAtomic IdentityNormalizationKind = iota
	IdentityNormalizationList
	IdentityNormalizationUnion
)

// IdentityNormalizationPlan captures normalization strategy for identity constraints.
type IdentityNormalizationPlan struct {
	Type    types.Type
	Item    *IdentityNormalizationPlan
	Members []*IdentityNormalizationPlan
	Kind    IdentityNormalizationKind
}
