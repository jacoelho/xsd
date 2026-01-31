package types

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/num"
)

type occursKind uint8

const (
	occursSmall occursKind = iota
	occursBig
	occursUnbounded
)

const (
	maxIntValue    = int(^uint(0) >> 1)
	maxOccursValue = uint64(^uint32(0))
)

var (
	// OccursUnbounded represents maxOccurs="unbounded".
	OccursUnbounded = Occurs{kind: occursUnbounded}

	// ErrOccursOverflow indicates occurrence arithmetic overflow.
	ErrOccursOverflow = errors.New("PARTICLES_OCCURS_OVERFLOW")
	// ErrOccursTooLarge indicates occurrence values exceed compile limits.
	ErrOccursTooLarge = errors.New("SCHEMA_OCCURS_TOO_LARGE")
)

// Occurs represents a non-negative occurrence bound or "unbounded".
type Occurs struct {
	big   num.Int
	small uint32
	kind  occursKind
}

// OccursFromInt returns an Occurs value from a non-negative integer.
func OccursFromInt(value int) Occurs {
	if value < 0 {
		return Occurs{kind: occursBig, big: num.FromInt64(int64(value))}
	}
	return OccursFromUint64(uint64(value))
}

// OccursFromUint64 returns an Occurs value from a non-negative uint64.
func OccursFromUint64(value uint64) Occurs {
	if value > maxOccursValue {
		return Occurs{kind: occursBig, big: num.FromUint64(value)}
	}
	return Occurs{kind: occursSmall, small: uint32(value)}
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
		if uint64(o.small) > uint64(maxIntValue) {
			return 0, false
		}
		return int(o.small), true
	}
	return 0, false
}

// IsOverflow reports whether the occurrence is too large to represent as uint32.
func (o Occurs) IsOverflow() bool {
	return o.kind == occursBig
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
		return o.big.Compare(other.big)
	}
	if o.kind == occursBig {
		if o.big.Sign < 0 {
			return -1
		}
		return 1
	}
	if other.kind == occursBig && other.big.Sign < 0 {
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
		v := uint64(value)
		switch {
		case uint64(o.small) < v:
			return -1
		case uint64(o.small) > v:
			return 1
		default:
			return 0
		}
	}
	if o.big.Sign < 0 {
		return -1
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
		sum := uint64(a.small) + uint64(b.small)
		if sum <= maxOccursValue {
			return Occurs{kind: occursSmall, small: uint32(sum)}
		}
		return Occurs{kind: occursBig, big: num.FromUint64(sum)}
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
		product := uint64(a.small) * uint64(b.small)
		if product <= maxOccursValue {
			return Occurs{kind: occursSmall, small: uint32(product)}
		}
		return Occurs{kind: occursBig, big: num.FromUint64(product)}
	}
	return Occurs{kind: occursBig, big: mulOccursBig(a, b)}
}

// String formats the occurrence bound for diagnostics.
func (o Occurs) String() string {
	switch o.kind {
	case occursUnbounded:
		return "unbounded"
	case occursBig:
		if o.big.Sign == 0 && len(o.big.Digits) == 0 {
			return "overflow"
		}
		return string(o.big.RenderCanonical(nil))
	default:
		return fmt.Sprintf("%d", o.small)
	}
}

func addOccursBig(a, b Occurs) num.Int {
	left := occursBigValue(a)
	right := occursBigValue(b)
	return num.Add(left, right)
}

func mulOccursBig(a, b Occurs) num.Int {
	left := occursBigValue(a)
	right := occursBigValue(b)
	return num.Mul(left, right)
}

func occursBigValue(o Occurs) num.Int {
	switch o.kind {
	case occursBig:
		return o.big
	case occursSmall:
		return num.FromUint64(uint64(o.small))
	default:
		return num.FromUint64(0)
	}
}
