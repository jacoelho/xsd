package validatorbuild

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *artifactCompiler) addAtomicValidator(kind runtime.ValidatorKind, ws runtime.WhitespaceMode, facets runtime.FacetProgramRef, stringKind runtime.StringKind, intKind runtime.IntegerKind) runtime.ValidatorID {
	stringKind, intKind = normalizeAtomicValidatorKinds(kind, stringKind, intKind)
	index := c.appendAtomicValidator(kind, stringKind, intKind)

	id := runtime.ValidatorID(len(c.bundle.Meta))
	flags := c.validatorFlags(facets)
	if kind == runtime.VString && stringKindTracksIDs(stringKind) {
		flags |= runtime.ValidatorMayTrackIDs
	}
	c.bundle.Meta = append(c.bundle.Meta, runtime.ValidatorMeta{
		Kind:       kind,
		Index:      index,
		WhiteSpace: ws,
		Facets:     facets,
		Flags:      flags,
	})
	return id
}

func normalizeAtomicValidatorKinds(kind runtime.ValidatorKind, stringKind runtime.StringKind, intKind runtime.IntegerKind) (runtime.StringKind, runtime.IntegerKind) {
	if kind == runtime.VString && stringKind == 0 {
		stringKind = runtime.StringAny
	}
	if kind == runtime.VInteger && intKind == 0 {
		intKind = runtime.IntegerAny
	}
	return stringKind, intKind
}

func appendValidatorSlot[T any](dst []T, value T) ([]T, uint32) {
	index := uint32(len(dst))
	dst = append(dst, value)
	return dst, index
}

func (c *artifactCompiler) appendAtomicValidator(kind runtime.ValidatorKind, stringKind runtime.StringKind, intKind runtime.IntegerKind) uint32 {
	switch kind {
	case runtime.VString:
		return c.appendStringValidator(runtime.StringValidator{Kind: stringKind})
	case runtime.VBoolean:
		return c.appendBooleanValidator(runtime.BooleanValidator{})
	case runtime.VDecimal:
		return c.appendDecimalValidator(runtime.DecimalValidator{})
	case runtime.VInteger:
		return c.appendIntegerValidator(runtime.IntegerValidator{Kind: intKind})
	case runtime.VFloat:
		return c.appendFloatValidator(runtime.FloatValidator{})
	case runtime.VDouble:
		return c.appendDoubleValidator(runtime.DoubleValidator{})
	case runtime.VDuration:
		return c.appendDurationValidator(runtime.DurationValidator{})
	case runtime.VDateTime:
		return c.appendDateTimeValidator(runtime.DateTimeValidator{})
	case runtime.VTime:
		return c.appendTimeValidator(runtime.TimeValidator{})
	case runtime.VDate:
		return c.appendDateValidator(runtime.DateValidator{})
	case runtime.VGYearMonth:
		return c.appendGYearMonthValidator(runtime.GYearMonthValidator{})
	case runtime.VGYear:
		return c.appendGYearValidator(runtime.GYearValidator{})
	case runtime.VGMonthDay:
		return c.appendGMonthDayValidator(runtime.GMonthDayValidator{})
	case runtime.VGDay:
		return c.appendGDayValidator(runtime.GDayValidator{})
	case runtime.VGMonth:
		return c.appendGMonthValidator(runtime.GMonthValidator{})
	case runtime.VAnyURI:
		return c.appendAnyURIValidator(runtime.AnyURIValidator{})
	case runtime.VQName:
		return c.appendQNameValidator(runtime.QNameValidator{})
	case runtime.VNotation:
		return c.appendNotationValidator(runtime.NotationValidator{})
	case runtime.VHexBinary:
		return c.appendHexBinaryValidator(runtime.HexBinaryValidator{})
	case runtime.VBase64Binary:
		return c.appendBase64BinaryValidator(runtime.Base64BinaryValidator{})
	default:
		return c.appendStringValidator(runtime.StringValidator{})
	}
}

func (c *artifactCompiler) appendStringValidator(value runtime.StringValidator) uint32 {
	var valueIndex uint32
	c.bundle.String, valueIndex = appendValidatorSlot(c.bundle.String, value)
	return valueIndex
}

func (c *artifactCompiler) appendBooleanValidator(value runtime.BooleanValidator) uint32 {
	var valueIndex uint32
	c.bundle.Boolean, valueIndex = appendValidatorSlot(c.bundle.Boolean, value)
	return valueIndex
}

func (c *artifactCompiler) appendDecimalValidator(value runtime.DecimalValidator) uint32 {
	var valueIndex uint32
	c.bundle.Decimal, valueIndex = appendValidatorSlot(c.bundle.Decimal, value)
	return valueIndex
}

func (c *artifactCompiler) appendIntegerValidator(value runtime.IntegerValidator) uint32 {
	var valueIndex uint32
	c.bundle.Integer, valueIndex = appendValidatorSlot(c.bundle.Integer, value)
	return valueIndex
}

func (c *artifactCompiler) appendFloatValidator(value runtime.FloatValidator) uint32 {
	var valueIndex uint32
	c.bundle.Float, valueIndex = appendValidatorSlot(c.bundle.Float, value)
	return valueIndex
}

func (c *artifactCompiler) appendDoubleValidator(value runtime.DoubleValidator) uint32 {
	var valueIndex uint32
	c.bundle.Double, valueIndex = appendValidatorSlot(c.bundle.Double, value)
	return valueIndex
}

func (c *artifactCompiler) appendDurationValidator(value runtime.DurationValidator) uint32 {
	var valueIndex uint32
	c.bundle.Duration, valueIndex = appendValidatorSlot(c.bundle.Duration, value)
	return valueIndex
}

func (c *artifactCompiler) appendDateTimeValidator(value runtime.DateTimeValidator) uint32 {
	var valueIndex uint32
	c.bundle.DateTime, valueIndex = appendValidatorSlot(c.bundle.DateTime, value)
	return valueIndex
}

func (c *artifactCompiler) appendTimeValidator(value runtime.TimeValidator) uint32 {
	var valueIndex uint32
	c.bundle.Time, valueIndex = appendValidatorSlot(c.bundle.Time, value)
	return valueIndex
}

func (c *artifactCompiler) appendDateValidator(value runtime.DateValidator) uint32 {
	var valueIndex uint32
	c.bundle.Date, valueIndex = appendValidatorSlot(c.bundle.Date, value)
	return valueIndex
}

func (c *artifactCompiler) appendGYearMonthValidator(value runtime.GYearMonthValidator) uint32 {
	var valueIndex uint32
	c.bundle.GYearMonth, valueIndex = appendValidatorSlot(c.bundle.GYearMonth, value)
	return valueIndex
}

func (c *artifactCompiler) appendGYearValidator(value runtime.GYearValidator) uint32 {
	var valueIndex uint32
	c.bundle.GYear, valueIndex = appendValidatorSlot(c.bundle.GYear, value)
	return valueIndex
}

func (c *artifactCompiler) appendGMonthDayValidator(value runtime.GMonthDayValidator) uint32 {
	var valueIndex uint32
	c.bundle.GMonthDay, valueIndex = appendValidatorSlot(c.bundle.GMonthDay, value)
	return valueIndex
}

func (c *artifactCompiler) appendGDayValidator(value runtime.GDayValidator) uint32 {
	var valueIndex uint32
	c.bundle.GDay, valueIndex = appendValidatorSlot(c.bundle.GDay, value)
	return valueIndex
}

func (c *artifactCompiler) appendGMonthValidator(value runtime.GMonthValidator) uint32 {
	var valueIndex uint32
	c.bundle.GMonth, valueIndex = appendValidatorSlot(c.bundle.GMonth, value)
	return valueIndex
}

func (c *artifactCompiler) appendAnyURIValidator(value runtime.AnyURIValidator) uint32 {
	var valueIndex uint32
	c.bundle.AnyURI, valueIndex = appendValidatorSlot(c.bundle.AnyURI, value)
	return valueIndex
}

func (c *artifactCompiler) appendQNameValidator(value runtime.QNameValidator) uint32 {
	var valueIndex uint32
	c.bundle.QName, valueIndex = appendValidatorSlot(c.bundle.QName, value)
	return valueIndex
}

func (c *artifactCompiler) appendNotationValidator(value runtime.NotationValidator) uint32 {
	var valueIndex uint32
	c.bundle.Notation, valueIndex = appendValidatorSlot(c.bundle.Notation, value)
	return valueIndex
}

func (c *artifactCompiler) appendHexBinaryValidator(value runtime.HexBinaryValidator) uint32 {
	var valueIndex uint32
	c.bundle.HexBinary, valueIndex = appendValidatorSlot(c.bundle.HexBinary, value)
	return valueIndex
}

func (c *artifactCompiler) appendBase64BinaryValidator(value runtime.Base64BinaryValidator) uint32 {
	var valueIndex uint32
	c.bundle.Base64Binary, valueIndex = appendValidatorSlot(c.bundle.Base64Binary, value)
	return valueIndex
}

func (c *artifactCompiler) addListValidator(ws runtime.WhitespaceMode, facets runtime.FacetProgramRef, item runtime.ValidatorID) runtime.ValidatorID {
	index := uint32(len(c.bundle.List))
	c.bundle.List = append(c.bundle.List, runtime.ListValidator{Item: item})

	id := runtime.ValidatorID(len(c.bundle.Meta))
	flags := c.validatorFlags(facets)
	if c.validatorTracksIDs(item) {
		flags |= runtime.ValidatorMayTrackIDs
	}
	c.bundle.Meta = append(c.bundle.Meta, runtime.ValidatorMeta{
		Kind:       runtime.VList,
		Index:      index,
		WhiteSpace: ws,
		Facets:     facets,
		Flags:      flags,
	})
	return id
}

func (c *artifactCompiler) addUnionValidator(ws runtime.WhitespaceMode, facets runtime.FacetProgramRef, members []runtime.ValidatorID, memberTypes []runtime.TypeID, unionName string, typeID runtime.TypeID) (runtime.ValidatorID, error) {
	if len(memberTypes) != len(members) {
		if typeID != 0 {
			return 0, fmt.Errorf("union member type count mismatch for %s (type %d): validators=%d memberTypes=%d", unionName, typeID, len(members), len(memberTypes))
		}
		return 0, fmt.Errorf("union member type count mismatch for %s: validators=%d memberTypes=%d", unionName, len(members), len(memberTypes))
	}
	off := uint32(len(c.bundle.UnionMembers))
	c.bundle.UnionMembers = append(c.bundle.UnionMembers, members...)
	c.bundle.UnionMemberTypes = append(c.bundle.UnionMemberTypes, memberTypes...)
	sameWS := make([]uint8, len(members))
	for i, member := range members {
		if int(member) >= len(c.bundle.Meta) {
			return 0, fmt.Errorf("union member validator %d out of range for %s", member, unionName)
		}
		if c.bundle.Meta[member].WhiteSpace == ws {
			sameWS[i] = 1
		}
	}
	c.bundle.UnionMemberSameWS = append(c.bundle.UnionMemberSameWS, sameWS...)
	index := uint32(len(c.bundle.Union))
	c.bundle.Union = append(c.bundle.Union, runtime.UnionValidator{
		MemberOff: off,
		MemberLen: uint32(len(members)),
	})

	id := runtime.ValidatorID(len(c.bundle.Meta))
	flags := c.validatorFlags(facets)
	if slices.ContainsFunc(members, c.validatorTracksIDs) {
		flags |= runtime.ValidatorMayTrackIDs
	}
	c.bundle.Meta = append(c.bundle.Meta, runtime.ValidatorMeta{
		Kind:       runtime.VUnion,
		Index:      index,
		WhiteSpace: ws,
		Facets:     facets,
		Flags:      flags,
	})
	return id, nil
}

func (c *artifactCompiler) validatorFlags(facets runtime.FacetProgramRef) runtime.ValidatorFlags {
	if facets.Len == 0 {
		return 0
	}
	end := facets.Off + facets.Len
	if int(end) > len(c.facets) {
		return 0
	}
	for i := facets.Off; i < end; i++ {
		if c.facets[i].Op == runtime.FEnum {
			return runtime.ValidatorHasEnum
		}
	}
	return 0
}

func (c *artifactCompiler) validatorTracksIDs(id runtime.ValidatorID) bool {
	if int(id) >= len(c.bundle.Meta) {
		return false
	}
	return c.bundle.Meta[id].Flags&runtime.ValidatorMayTrackIDs != 0
}

func stringKindTracksIDs(kind runtime.StringKind) bool {
	switch kind {
	case runtime.StringID, runtime.StringIDREF, runtime.StringEntity:
		return true
	default:
		return false
	}
}
