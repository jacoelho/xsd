package types

import internaloccurs "github.com/jacoelho/xsd/internal/occurs"

type Occurs = internaloccurs.Occurs

var (
	OccursUnbounded   = internaloccurs.OccursUnbounded
	ErrOccursOverflow = internaloccurs.ErrOccursOverflow
	ErrOccursTooLarge = internaloccurs.ErrOccursTooLarge
)

func OccursFromInt(value int) Occurs {
	return internaloccurs.OccursFromInt(value)
}

func OccursFromUint64(value uint64) Occurs {
	return internaloccurs.OccursFromUint64(value)
}

func MinOccurs(a, b Occurs) Occurs {
	return internaloccurs.MinOccurs(a, b)
}

func MaxOccurs(a, b Occurs) Occurs {
	return internaloccurs.MaxOccurs(a, b)
}

func AddOccurs(a, b Occurs) Occurs {
	return internaloccurs.AddOccurs(a, b)
}

func MulOccurs(a, b Occurs) Occurs {
	return internaloccurs.MulOccurs(a, b)
}
