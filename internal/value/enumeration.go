package value

import (
	"encoding/binary"
	"math"
	"strconv"
	"strings"
)

// enumerationIndexMinLiterals is the smallest group that gets an index;
// smaller groups are scanned. Membership work per group visit is therefore at
// most enumerationIndexMinLiterals-1 comparisons or one key encoding and lookup.
const enumerationIndexMinLiterals = 8

// enumerationKeyBuffer is the stack capacity for one probe key; longer keys
// spill to the heap.
const enumerationKeyBuffer = 128

// enumerationListKeyTag starts a list key. Atomic keys start with their
// PrimitiveKind, which never takes this value.
const enumerationListKeyTag = 0xff

// enumerationGroup is one derivation step's enumeration literals in declaration
// order, which diagnostics list. A value satisfies the group when it equals any
// literal; each group of a derivation chain is checked independently.
//
// A group of at least enumerationIndexMinLiterals literals has an index from
// key to the literal's first position. Key equality holds exactly when
// equalParsed does, so a lookup leaves at most one candidate and equalParsed
// still decides. Two key schemes exist:
//   - text-keyed (textKind set): every literal is a non-list value of one
//     primitive whose canonical text is its equality key: string after
//     whitespace normalization, anyURI, or base64Binary (literals are compact,
//     and prepareBase64EnumerationValue compacts a value before membership).
//     The raw string-enumeration plan relies on this scheme for string groups;
//     base64 groups use it to avoid copying and spilling long literal keys.
//   - value-keyed: any other group; the key is appendEnumerationKey's encoding.
//
// Smaller groups, and groups holding a literal without a key, have a nil index.
type enumerationGroup struct {
	index    map[string]int
	literals []parsedValue
	textKind PrimitiveKind
	textKey  bool
}

func newEnumerationGroup(literals []parsedValue) enumerationGroup {
	group := enumerationGroup{literals: literals}
	if len(literals) < enumerationIndexMinLiterals {
		return group
	}
	if kind, ok := enumerationTextKind(literals); ok {
		index := make(map[string]int, len(literals))
		for i := range literals {
			addEnumerationKey(index, literals[i].atom.lexical.Canonical, i)
		}
		group.index, group.textKind, group.textKey = index, kind, true
		return group
	}
	index := make(map[string]int, len(literals))
	var buf [enumerationKeyBuffer]byte
	for i := range literals {
		key, ok := appendEnumerationKey(buf[:0], &literals[i])
		if !ok {
			return group
		}
		addEnumerationKey(index, string(key), i)
	}
	group.index = index
	return group
}

func enumerationTextKind(literals []parsedValue) (PrimitiveKind, bool) {
	kind := literals[0].atom.kind
	if kind != PrimitiveString && kind != PrimitiveAnyURI && kind != PrimitiveBase64Binary {
		return 0, false
	}
	for i := range literals {
		if literals[i].isList || literals[i].atom.kind != kind {
			return 0, false
		}
	}
	return kind, true
}

func addEnumerationKey(index map[string]int, key string, i int) {
	if _, seen := index[key]; !seen {
		index[key] = i
	}
}

// matchEnumerationGroups returns the index of the first group value does not
// satisfy, or -1. The value key is encoded at most once for the whole chain.
func matchEnumerationGroups(groups []enumerationGroup, value *parsedValue) int {
	var buf [enumerationKeyBuffer]byte
	var key []byte
	keyed := false
	for i := range groups {
		group := &groups[i]
		if group.index != nil && !group.textKey && !keyed {
			var ok bool
			if key, ok = appendEnumerationKey(buf[:0], value); !ok {
				key = nil
			}
			keyed = true
		}
		if !group.matches(value, key) {
			return i
		}
	}
	return -1
}

// matches reports whether value equals a literal of the group. key is value's
// encoding, or nil when it has none (every encoding is non-empty); only
// value-keyed groups consult it.
func (g *enumerationGroup) matches(value *parsedValue, key []byte) bool {
	switch {
	case g.index == nil:
		return g.scan(value)
	case g.textKey:
		if value.isList || value.atom.kind != g.textKind {
			return false
		}
		return g.lookup(value, value.atom.lexical.Canonical)
	default:
		return key != nil && g.lookup(value, string(key))
	}
}

func (g *enumerationGroup) scan(value *parsedValue) bool {
	for i := range g.literals {
		if equalParsed(value, &g.literals[i]) {
			return true
		}
	}
	return false
}

func (g *enumerationGroup) lookup(value *parsedValue, key string) bool {
	i, ok := g.index[key]
	return ok && equalParsed(value, &g.literals[i])
}

// matchesBytes reports whether lexical is the canonical text of a literal of a
// group admitted by the raw string-enumeration plan, whose admission rule makes
// that text each literal's equality key and the group text-keyed when indexed.
func (g *enumerationGroup) matchesBytes(lexical []byte) bool {
	if g.index != nil {
		_, ok := g.index[string(lexical)]
		return ok
	}
	for i := range g.literals {
		if string(lexical) == g.literals[i].atom.lexical.Canonical {
			return true
		}
	}
	return false
}

// appendEnumerationKey appends a self-delimiting encoding of exactly the
// value-space fields equalParsed compares, so two values share an encoding
// exactly when they are equal. It reads only fields every validation fills,
// never the canonical projections that depend on requested needs. It reports
// false for a value it cannot encode: a list that retained fewer items than it
// holds, or an unknown primitive.
func appendEnumerationKey(b []byte, v *parsedValue) ([]byte, bool) {
	if !v.isList {
		return appendAtomicEnumerationKey(b, &v.atom)
	}
	if uint64(len(v.items)) != uint64(v.count) {
		return b, false
	}
	// Item keys are self-delimiting and items are atomic, so the tag and
	// the item sequence determine the list.
	b = append(b, enumerationListKeyTag)
	for i := range v.items {
		var ok bool
		if b, ok = appendEnumerationKey(b, &v.items[i]); !ok {
			return b, false
		}
	}
	return b, true
}

func appendAtomicEnumerationKey(b []byte, a *atomicValue) ([]byte, bool) {
	b = append(b, byte(a.kind))
	switch a.kind {
	case PrimitiveString, PrimitiveAnyURI, PrimitiveBase64Binary:
		// Base64 literals are compact, and prepareBase64EnumerationValue
		// compacts a value before membership; admitted compact text has one
		// spelling per octet sequence.
		return appendKeyText(b, a.lexical.Canonical), true
	case PrimitiveBoolean:
		return appendKeyBool(b, a.boolean), true
	case PrimitiveDecimal:
		d := &a.decimal
		b = appendKeyBool(b, d.negative)
		b = appendKeyText(b, d.text[d.intTrimStart:d.intEnd])
		return appendKeyText(b, d.text[d.fracStart:d.fracTrimEnd]), true
	case PrimitiveFloat, PrimitiveDouble:
		return binary.BigEndian.AppendUint64(b, enumerationFloatBits(a.float)), true
	case PrimitiveDuration:
		months, seconds := a.duration.coordinates()
		b = appendKeyDurationInteger(b, months)
		b = appendKeyDurationInteger(b, seconds)
		b = appendKeyBool(b, a.duration.negativeFrac)
		return appendKeyText(b, a.duration.frac), true
	case PrimitiveDate, PrimitiveDateTime:
		b = appendKeyBool(b, a.temporal.hasTZ)
		return appendKeyDateTimePoint(b, a.temporal.instant), true
	case PrimitiveTime:
		b = appendKeyBool(b, a.time.hasTZ)
		b = binary.AppendVarint(b, int64(a.time.second))
		return appendKeyText(b, a.time.frac), true
	case PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay, PrimitiveGMonth:
		b = appendKeyBool(b, a.g.tz.present)
		return appendKeyDateTimePoint(b, a.g.instant), true
	case PrimitiveHexBinary:
		// Hex equality ignores letter case.
		text := a.lexical.Canonical
		b = binary.AppendUvarint(b, uint64(len(text)))
		for i := range len(text) {
			c := text[i]
			if 'a' <= c && c <= 'f' {
				c -= 'a' - 'A'
			}
			b = append(b, c)
		}
		return b, true
	case PrimitiveQName, PrimitiveNotation:
		b = appendKeyText(b, a.expandedName.Namespace)
		return appendKeyText(b, a.expandedName.Local), true
	default:
		return b, false
	}
}

// enumerationFloatBits folds the float values EqualFloatValues equates: both
// zeros, and every NaN.
func enumerationFloatBits(f float64) uint64 {
	switch {
	case f == 0:
		return 0
	case math.IsNaN(f):
		return math.Float64bits(math.NaN())
	default:
		return math.Float64bits(f)
	}
}

func appendKeyDateTimePoint(b []byte, p xsdDateTimePoint) []byte {
	year := strings.TrimLeft(p.year.digits, "0")
	b = appendKeyBool(b, p.year.neg && year != "")
	b = appendKeyText(b, year)
	b = binary.AppendVarint(b, int64(p.month))
	b = binary.AppendVarint(b, int64(p.day))
	b = binary.AppendVarint(b, int64(p.second))
	return appendKeyText(b, p.frac)
}

// appendKeyDurationInteger appends n's decimal text, which compareDurationInteger
// equates exactly when the integers are equal, whichever form holds them.
func appendKeyDurationInteger(b []byte, n durationInteger) []byte {
	if n.digits != "" {
		return appendKeyText(b, n.digits)
	}
	var digits [20]byte
	text := strconv.AppendInt(digits[:0], n.small, 10)
	b = binary.AppendUvarint(b, uint64(len(text)))
	return append(b, text...)
}

func appendKeyBool(b []byte, v bool) []byte { //nolint:revive // v is an encoded value-space field, not a control flag.
	if v {
		return append(b, 1)
	}
	return append(b, 0)
}

func appendKeyText(b []byte, s string) []byte {
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}
