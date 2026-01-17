package types

import (
	"fmt"
	"math/big"
)

type occursKind uint8

const (
	occursSmall occursKind = iota
	occursBig
	occursUnbounded
)

const maxIntValue = int(^uint(0) >> 1)

var (
	// OccursUnbounded represents maxOccurs="unbounded".
	OccursUnbounded = Occurs{kind: occursUnbounded}
	maxIntBig       = big.NewInt(int64(maxIntValue))
)

// Occurs represents a non-negative occurrence bound or "unbounded".
type Occurs struct {
	kind  occursKind
	small int
	big   *big.Int
}

// OccursFromInt returns an Occurs value from a non-negative integer.
func OccursFromInt(value int) Occurs {
	return Occurs{kind: occursSmall, small: value}
}

// OccursFromBig returns an Occurs value from a non-negative big integer.
func OccursFromBig(value *big.Int) (Occurs, error) {
	if value == nil {
		return Occurs{}, fmt.Errorf("nil occurs value")
	}
	if value.Sign() < 0 {
		return Occurs{}, fmt.Errorf("occurs value must be non-negative")
	}
	if value.Cmp(maxIntBig) <= 0 {
		return Occurs{kind: occursSmall, small: int(value.Int64())}, nil
	}
	return Occurs{kind: occursBig, big: new(big.Int).Set(value)}, nil
}

// IsUnbounded reports whether the occurrence bound is unbounded.
func (o Occurs) IsUnbounded() bool {
	return o.kind == occursUnbounded
}

// IsZero reports whether the occurrence bound is zero.
func (o Occurs) IsZero() bool {
	return o.kind == occursSmall && o.small == 0
}

// IsOne reports whether the occurrence bound is one.
func (o Occurs) IsOne() bool {
	return o.kind == occursSmall && o.small == 1
}

// Int returns the small integer value and true if it fits in int.
func (o Occurs) Int() (int, bool) {
	if o.kind == occursSmall {
		return o.small, true
	}
	return 0, false
}

// Cmp compares two occurrence bounds.
func (o Occurs) Cmp(other Occurs) int {
	if o.kind == occursUnbounded {
		if other.kind == occursUnbounded {
			return 0
		}
		return 1
	}
	if other.kind == occursUnbounded {
		return -1
	}
	if o.kind == occursSmall && other.kind == occursSmall {
		switch {
		case o.small < other.small:
			return -1
		case o.small > other.small:
			return 1
		default:
			return 0
		}
	}
	if o.kind == occursBig && other.kind == occursBig {
		return o.big.Cmp(other.big)
	}
	if o.kind == occursBig {
		return 1
	}
	return -1
}

// CmpInt compares the occurrence bound to a non-negative int.
func (o Occurs) CmpInt(value int) int {
	if value < 0 {
		return 1
	}
	if o.kind == occursUnbounded {
		return 1
	}
	if o.kind == occursSmall {
		switch {
		case o.small < value:
			return -1
		case o.small > value:
			return 1
		default:
			return 0
		}
	}
	return 1
}

// Equal reports whether the occurrence bounds are equal.
func (o Occurs) Equal(other Occurs) bool {
	return o.Cmp(other) == 0
}

// EqualInt reports whether the occurrence bound equals the provided int.
func (o Occurs) EqualInt(value int) bool {
	return o.CmpInt(value) == 0
}

// LessThanInt reports whether the occurrence bound is less than the provided int.
func (o Occurs) LessThanInt(value int) bool {
	return o.CmpInt(value) < 0
}

// GreaterThanInt reports whether the occurrence bound is greater than the provided int.
func (o Occurs) GreaterThanInt(value int) bool {
	return o.CmpInt(value) > 0
}

// MinOccurs returns the smaller occurrence bound (treating unbounded as infinity).
func MinOccurs(a, b Occurs) Occurs {
	if a.kind == occursUnbounded {
		return b
	}
	if b.kind == occursUnbounded {
		return a
	}
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}

// MaxOccurs returns the larger occurrence bound (treating unbounded as infinity).
func MaxOccurs(a, b Occurs) Occurs {
	if a.kind == occursUnbounded || b.kind == occursUnbounded {
		return OccursUnbounded
	}
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}

// AddOccurs adds two occurrence bounds, returning unbounded if either is unbounded.
func AddOccurs(a, b Occurs) Occurs {
	if a.kind == occursUnbounded || b.kind == occursUnbounded {
		return OccursUnbounded
	}
	if a.kind == occursSmall && b.kind == occursSmall {
		if a.small <= maxIntValue-b.small {
			return Occurs{kind: occursSmall, small: a.small + b.small}
		}
	}
	return Occurs{kind: occursBig, big: addOccursBig(a, b)}
}

// MulOccurs multiplies two occurrence bounds, treating unbounded as infinity.
func MulOccurs(a, b Occurs) Occurs {
	if a.IsZero() || b.IsZero() {
		return OccursFromInt(0)
	}
	if a.kind == occursUnbounded || b.kind == occursUnbounded {
		return OccursUnbounded
	}
	if a.kind == occursSmall && b.kind == occursSmall {
		if a.small <= maxIntValue/b.small {
			return Occurs{kind: occursSmall, small: a.small * b.small}
		}
	}
	return Occurs{kind: occursBig, big: mulOccursBig(a, b)}
}

// String formats the occurrence bound for diagnostics.
func (o Occurs) String() string {
	switch o.kind {
	case occursUnbounded:
		return "unbounded"
	case occursBig:
		if o.big == nil {
			return "0"
		}
		return o.big.String()
	default:
		return fmt.Sprintf("%d", o.small)
	}
}

func addOccursBig(a, b Occurs) *big.Int {
	left := occursBigValue(a)
	right := occursBigValue(b)
	return new(big.Int).Add(left, right)
}

func mulOccursBig(a, b Occurs) *big.Int {
	left := occursBigValue(a)
	right := occursBigValue(b)
	return new(big.Int).Mul(left, right)
}

func occursBigValue(o Occurs) *big.Int {
	switch o.kind {
	case occursBig:
		if o.big == nil {
			return big.NewInt(0)
		}
		return new(big.Int).Set(o.big)
	case occursSmall:
		return big.NewInt(int64(o.small))
	default:
		return big.NewInt(0)
	}
}
