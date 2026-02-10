package model

import internaloccurs "github.com/jacoelho/xsd/internal/occurs"

type Occurs = internaloccurs.Occurs

var (
	OccursUnbounded   = internaloccurs.OccursUnbounded
	ErrOccursOverflow = internaloccurs.ErrOccursOverflow
	ErrOccursTooLarge = internaloccurs.ErrOccursTooLarge
	OccursFromInt     = internaloccurs.OccursFromInt
	OccursFromUint64  = internaloccurs.OccursFromUint64
	MinOccurs         = internaloccurs.MinOccurs
	MaxOccurs         = internaloccurs.MaxOccurs
	AddOccurs         = internaloccurs.AddOccurs
	MulOccurs         = internaloccurs.MulOccurs
)
