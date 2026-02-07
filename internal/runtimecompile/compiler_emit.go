package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *compiler) addAtomicValidator(kind runtime.ValidatorKind, ws runtime.WhitespaceMode, facets runtime.FacetProgramRef, stringKind runtime.StringKind, intKind runtime.IntegerKind) runtime.ValidatorID {
	index := uint32(0)
	switch kind {
	case runtime.VString:
		index = uint32(len(c.bundle.String))
		if stringKind == 0 {
			stringKind = runtime.StringAny
		}
		c.bundle.String = append(c.bundle.String, runtime.StringValidator{Kind: stringKind})
	case runtime.VBoolean:
		index = uint32(len(c.bundle.Boolean))
		c.bundle.Boolean = append(c.bundle.Boolean, runtime.BooleanValidator{})
	case runtime.VDecimal:
		index = uint32(len(c.bundle.Decimal))
		c.bundle.Decimal = append(c.bundle.Decimal, runtime.DecimalValidator{})
	case runtime.VInteger:
		index = uint32(len(c.bundle.Integer))
		if intKind == 0 {
			intKind = runtime.IntegerAny
		}
		c.bundle.Integer = append(c.bundle.Integer, runtime.IntegerValidator{Kind: intKind})
	case runtime.VFloat:
		index = uint32(len(c.bundle.Float))
		c.bundle.Float = append(c.bundle.Float, runtime.FloatValidator{})
	case runtime.VDouble:
		index = uint32(len(c.bundle.Double))
		c.bundle.Double = append(c.bundle.Double, runtime.DoubleValidator{})
	case runtime.VDuration:
		index = uint32(len(c.bundle.Duration))
		c.bundle.Duration = append(c.bundle.Duration, runtime.DurationValidator{})
	case runtime.VDateTime:
		index = uint32(len(c.bundle.DateTime))
		c.bundle.DateTime = append(c.bundle.DateTime, runtime.DateTimeValidator{})
	case runtime.VTime:
		index = uint32(len(c.bundle.Time))
		c.bundle.Time = append(c.bundle.Time, runtime.TimeValidator{})
	case runtime.VDate:
		index = uint32(len(c.bundle.Date))
		c.bundle.Date = append(c.bundle.Date, runtime.DateValidator{})
	case runtime.VGYearMonth:
		index = uint32(len(c.bundle.GYearMonth))
		c.bundle.GYearMonth = append(c.bundle.GYearMonth, runtime.GYearMonthValidator{})
	case runtime.VGYear:
		index = uint32(len(c.bundle.GYear))
		c.bundle.GYear = append(c.bundle.GYear, runtime.GYearValidator{})
	case runtime.VGMonthDay:
		index = uint32(len(c.bundle.GMonthDay))
		c.bundle.GMonthDay = append(c.bundle.GMonthDay, runtime.GMonthDayValidator{})
	case runtime.VGDay:
		index = uint32(len(c.bundle.GDay))
		c.bundle.GDay = append(c.bundle.GDay, runtime.GDayValidator{})
	case runtime.VGMonth:
		index = uint32(len(c.bundle.GMonth))
		c.bundle.GMonth = append(c.bundle.GMonth, runtime.GMonthValidator{})
	case runtime.VAnyURI:
		index = uint32(len(c.bundle.AnyURI))
		c.bundle.AnyURI = append(c.bundle.AnyURI, runtime.AnyURIValidator{})
	case runtime.VQName:
		index = uint32(len(c.bundle.QName))
		c.bundle.QName = append(c.bundle.QName, runtime.QNameValidator{})
	case runtime.VNotation:
		index = uint32(len(c.bundle.Notation))
		c.bundle.Notation = append(c.bundle.Notation, runtime.NotationValidator{})
	case runtime.VHexBinary:
		index = uint32(len(c.bundle.HexBinary))
		c.bundle.HexBinary = append(c.bundle.HexBinary, runtime.HexBinaryValidator{})
	case runtime.VBase64Binary:
		index = uint32(len(c.bundle.Base64Binary))
		c.bundle.Base64Binary = append(c.bundle.Base64Binary, runtime.Base64BinaryValidator{})
	default:
		index = uint32(len(c.bundle.String))
		c.bundle.String = append(c.bundle.String, runtime.StringValidator{})
	}

	id := runtime.ValidatorID(len(c.bundle.Meta))
	c.bundle.Meta = append(c.bundle.Meta, runtime.ValidatorMeta{
		Kind:       kind,
		Index:      index,
		WhiteSpace: ws,
		Facets:     facets,
		Flags:      c.validatorFlags(facets),
	})
	return id
}

func (c *compiler) addListValidator(ws runtime.WhitespaceMode, facets runtime.FacetProgramRef, item runtime.ValidatorID) runtime.ValidatorID {
	index := uint32(len(c.bundle.List))
	c.bundle.List = append(c.bundle.List, runtime.ListValidator{Item: item})

	id := runtime.ValidatorID(len(c.bundle.Meta))
	c.bundle.Meta = append(c.bundle.Meta, runtime.ValidatorMeta{
		Kind:       runtime.VList,
		Index:      index,
		WhiteSpace: ws,
		Facets:     facets,
		Flags:      c.validatorFlags(facets),
	})
	return id
}

func (c *compiler) addUnionValidator(ws runtime.WhitespaceMode, facets runtime.FacetProgramRef, members []runtime.ValidatorID, memberTypes []runtime.TypeID, unionName string, typeID runtime.TypeID) (runtime.ValidatorID, error) {
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
	c.bundle.Meta = append(c.bundle.Meta, runtime.ValidatorMeta{
		Kind:       runtime.VUnion,
		Index:      index,
		WhiteSpace: ws,
		Facets:     facets,
		Flags:      c.validatorFlags(facets),
	})
	return id, nil
}

func (c *compiler) validatorFlags(facets runtime.FacetProgramRef) runtime.ValidatorFlags {
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
