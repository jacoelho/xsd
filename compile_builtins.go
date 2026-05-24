package xsd

const (
	xsdBuiltinSimpleTypeCount      = 45
	internalBuiltinSimpleTypeCount = 2
	builtinSimpleTypeCount         = xsdBuiltinSimpleTypeCount + internalBuiltinSimpleTypeCount
	builtinAttributeCount          = 11
	builtinComplexTypeCount        = 1
	builtinGlobalTypeCount         = xsdBuiltinSimpleTypeCount + builtinComplexTypeCount
)

func (c *compiler) addBuiltins() error {
	anySimple, err := c.addBuiltinAtomicSimpleType("anySimpleType", primString, noSimpleType, whitespacePreserve)
	if err != nil {
		return err
	}
	c.rt.Builtin.AnySimpleType = anySimple
	if err := c.addBuiltinStringTypes(anySimple); err != nil {
		return err
	}
	if err := c.addBuiltinListTypes(anySimple); err != nil {
		return err
	}
	if err := c.addBuiltinNumericTypes(anySimple); err != nil {
		return err
	}
	if err := c.addBuiltinOtherPrimitiveTypes(anySimple); err != nil {
		return err
	}
	if err := c.addBuiltinXMLAttributes(); err != nil {
		return err
	}
	return c.addBuiltinAnyType()
}

func (c *compiler) addBuiltinStringTypes(anySimple simpleTypeID) error {
	var err error
	c.rt.Builtin.String, err = c.addBuiltinAtomicSimpleType("string", primString, anySimple, whitespacePreserve)
	if err != nil {
		return err
	}
	for _, typ := range []struct {
		local string
		ws    whitespaceMode
	}{
		{"normalizedString", whitespaceReplace},
		{"token", whitespaceCollapse},
		{"language", whitespaceCollapse},
		{"Name", whitespaceCollapse},
		{"NCName", whitespaceCollapse},
	} {
		_, err = c.addBuiltinAtomicSimpleType(typ.local, primString, c.rt.Builtin.String, typ.ws)
		if err != nil {
			return err
		}
	}
	c.rt.Builtin.NMTOKEN, err = c.addBuiltinAtomicSimpleType("NMTOKEN", primString, c.rt.Builtin.String, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.Builtin.ID, err = c.addBuiltinAtomicSimpleType("ID", primString, c.rt.Builtin.String, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.Builtin.IDREF, err = c.addBuiltinAtomicSimpleType("IDREF", primString, c.rt.Builtin.String, whitespaceCollapse)
	return err
}

func (c *compiler) addBuiltinListTypes(anySimple simpleTypeID) error {
	minOne := uint32(1)
	var err error
	c.rt.Builtin.IDREFS, err = c.addBuiltinListSimpleType("IDREFS", c.rt.Builtin.IDREF, anySimple, &minOne)
	if err != nil {
		return err
	}
	c.rt.Builtin.NMTOKENS, err = c.addBuiltinListSimpleType("NMTOKENS", c.rt.Builtin.NMTOKEN, anySimple, &minOne)
	if err != nil {
		return err
	}
	c.rt.Builtin.ENTITY, err = c.addBuiltinAtomicSimpleType("ENTITY", primString, c.rt.Builtin.String, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.Builtin.ENTITIES, err = c.addBuiltinListSimpleType("ENTITIES", c.rt.Builtin.ENTITY, anySimple, &minOne)
	return err
}

func (c *compiler) addBuiltinNumericTypes(anySimple simpleTypeID) error {
	var err error
	c.rt.Builtin.Boolean, err = c.addBuiltinAtomicSimpleType("boolean", primBoolean, anySimple, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.Builtin.Decimal, err = c.addBuiltinAtomicSimpleType("decimal", primDecimal, anySimple, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.Builtin.Integer, err = c.addBuiltinAtomicSimpleType("integer", primDecimal, c.rt.Builtin.Decimal, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(c.rt.Builtin.Integer)
	err = c.addBuiltinIntegerDerivedTypes()
	if err != nil {
		return err
	}
	_, err = c.addBuiltinAtomicSimpleType("float", primFloat, anySimple, whitespaceCollapse)
	if err != nil {
		return err
	}
	_, err = c.addBuiltinAtomicSimpleType("double", primDouble, anySimple, whitespaceCollapse)
	return err
}

func (c *compiler) addBuiltinIntegerDerivedTypes() error {
	nonPositive, err := c.addBuiltinAtomicSimpleType("nonPositiveInteger", primDecimal, c.rt.Builtin.Integer, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(nonPositive)
	c.setBuiltinMax(nonPositive, "0")
	negative, err := c.addBuiltinAtomicSimpleType("negativeInteger", primDecimal, nonPositive, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(negative)
	c.setBuiltinMax(negative, "-1")
	nonNegative, err := c.addBuiltinAtomicSimpleType("nonNegativeInteger", primDecimal, c.rt.Builtin.Integer, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(nonNegative)
	c.setBuiltinMin(nonNegative, "0")
	positive, err := c.addBuiltinAtomicSimpleType("positiveInteger", primDecimal, nonNegative, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(positive)
	c.setBuiltinMin(positive, "1")
	long, err := c.addBuiltinAtomicSimpleType("long", primDecimal, c.rt.Builtin.Integer, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(long)
	c.setBuiltinRange(long, "-9223372036854775808", "9223372036854775807")
	c.rt.Builtin.Int, err = c.addBuiltinAtomicSimpleType("int", primDecimal, long, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(c.rt.Builtin.Int)
	c.setBuiltinRange(c.rt.Builtin.Int, "-2147483648", "2147483647")
	return c.addBuiltinSmallIntegerTypes(nonNegative)
}

func (c *compiler) addBuiltinSmallIntegerTypes(nonNegative simpleTypeID) error {
	short, err := c.addBuiltinAtomicSimpleType("short", primDecimal, c.rt.Builtin.Int, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(short)
	c.setBuiltinRange(short, "-32768", "32767")
	byteType, err := c.addBuiltinAtomicSimpleType("byte", primDecimal, short, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(byteType)
	c.setBuiltinRange(byteType, "-128", "127")
	unsignedLong, err := c.addBuiltinAtomicSimpleType("unsignedLong", primDecimal, nonNegative, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(unsignedLong)
	c.setBuiltinMax(unsignedLong, "18446744073709551615")
	unsignedInt, err := c.addBuiltinAtomicSimpleType("unsignedInt", primDecimal, unsignedLong, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(unsignedInt)
	c.setBuiltinMax(unsignedInt, "4294967295")
	unsignedShort, err := c.addBuiltinAtomicSimpleType("unsignedShort", primDecimal, unsignedInt, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(unsignedShort)
	c.setBuiltinMax(unsignedShort, "65535")
	unsignedByte, err := c.addBuiltinAtomicSimpleType("unsignedByte", primDecimal, unsignedShort, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.setBuiltinIntegerFacets(unsignedByte)
	c.setBuiltinMax(unsignedByte, "255")
	return nil
}

func (c *compiler) addBuiltinOtherPrimitiveTypes(anySimple simpleTypeID) error {
	var err error
	c.rt.Builtin.AnyURI, err = c.addBuiltinAtomicSimpleType("anyURI", primAnyURI, c.rt.Builtin.String, whitespaceCollapse)
	if err != nil {
		return err
	}
	_, err = c.addBuiltinAtomicSimpleType("duration", primDuration, anySimple, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.Builtin.DateTime, err = c.addBuiltinAtomicSimpleType("dateTime", primDateTime, anySimple, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.Builtin.Time, err = c.addBuiltinAtomicSimpleType("time", primTime, anySimple, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.Builtin.Date, err = c.addBuiltinAtomicSimpleType("date", primDate, anySimple, whitespaceCollapse)
	if err != nil {
		return err
	}
	for _, typ := range []struct {
		local     string
		primitive primitiveKind
	}{
		{"gYearMonth", primGYearMonth},
		{"gYear", primGYear},
		{"gMonthDay", primGMonthDay},
		{"gDay", primGDay},
		{"gMonth", primGMonth},
		{"hexBinary", primHexBinary},
		{"base64Binary", primBase64Binary},
	} {
		_, err = c.addBuiltinAtomicSimpleType(typ.local, typ.primitive, anySimple, whitespaceCollapse)
		if err != nil {
			return err
		}
	}
	c.rt.Builtin.qName, err = c.addBuiltinAtomicSimpleType("QName", primQName, anySimple, whitespaceCollapse)
	if err != nil {
		return err
	}
	_, err = c.addBuiltinAtomicSimpleType("NOTATION", primNotation, anySimple, whitespaceCollapse)
	return err
}

func (c *compiler) addBuiltinXMLAttributes() error {
	xmlLang, err := c.addInternalAtomicSimpleType(xmlNamespaceURI, "lang", primString, c.rt.Builtin.String, whitespaceCollapse, builtinValidationXMLLang)
	if err != nil {
		return err
	}
	xmlSpace, err := c.addInternalAtomicSimpleType(xmlNamespaceURI, "space", primString, c.rt.Builtin.String, whitespaceCollapse, builtinValidationXMLSpace)
	if err != nil {
		return err
	}
	for _, attr := range []struct {
		ns    string
		local string
		typ   simpleTypeID
	}{
		{xmlNamespaceURI, "base", c.rt.Builtin.AnyURI},
		{xmlNamespaceURI, "id", c.rt.Builtin.ID},
		{xmlNamespaceURI, "lang", xmlLang},
		{xmlNamespaceURI, "space", xmlSpace},
		{xlinkNamespaceURI, "type", c.rt.Builtin.String},
		{xlinkNamespaceURI, "href", c.rt.Builtin.AnyURI},
		{xlinkNamespaceURI, "role", c.rt.Builtin.AnyURI},
		{xlinkNamespaceURI, "arcrole", c.rt.Builtin.AnyURI},
		{xlinkNamespaceURI, "title", c.rt.Builtin.String},
		{xlinkNamespaceURI, "show", c.rt.Builtin.String},
		{xlinkNamespaceURI, "actuate", c.rt.Builtin.String},
	} {
		if err := c.addBuiltinAttribute(attr.ns, attr.local, attr.typ); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) addBuiltinAnyType() error {
	anyWildcard := wildcardID(len(c.rt.Wildcards))
	c.rt.Wildcards = append(c.rt.Wildcards, wildcard{Mode: wildAny, Process: processLax})
	attrs := attributeUseSet{wildcard: anyWildcard}
	attrSet := attributeUseSetID(len(c.rt.AttributeUseSets))
	c.rt.AttributeUseSets = append(c.rt.AttributeUseSets, attrs)
	anyModel := contentModel{Kind: modelAny, Mixed: true}
	modelID := contentModelID(len(c.rt.Models))
	c.rt.Models = append(c.rt.Models, anyModel)
	q, err := c.rt.Names.InternQName(xsdNamespaceURI, "anyType")
	if err != nil {
		return err
	}
	complexID := complexTypeID(len(c.rt.ComplexTypes))
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, complexType{Name: q, Content: modelID, Attrs: attrSet, Mixed: true, Base: typeID{Kind: typeComplex, ID: uint32(noComplexType)}})
	c.rt.Builtin.AnyType = complexID
	c.complexDone[q] = complexID
	c.rt.GlobalTypes[q] = typeID{Kind: typeComplex, ID: uint32(complexID)}
	return nil
}

func (c *compiler) addBuiltinAtomicSimpleType(local string, primitive primitiveKind, base simpleTypeID, ws whitespaceMode) (simpleTypeID, error) {
	q, err := c.rt.Names.InternQName(xsdNamespaceURI, local)
	if err != nil {
		return noSimpleType, err
	}
	id := simpleTypeID(len(c.rt.SimpleTypes))
	facets := facetSet{}
	if base != noSimpleType && validUint32Index(uint32(base), len(c.rt.SimpleTypes)) {
		facets = cloneFacetSet(c.rt.SimpleTypes[base].Facets)
	}
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, simpleType{
		Name:       q,
		Variety:    varietyAtomic,
		Primitive:  primitive,
		Base:       base,
		Whitespace: ws,
		Facets:     facets,
		Builtin:    builtinValidationForLocal(local),
	})
	c.simpleDone[q] = id
	c.rt.GlobalTypes[q] = typeID{Kind: typeSimple, ID: uint32(id)}
	return id, nil
}

func (c *compiler) addInternalAtomicSimpleType(ns, local string, primitive primitiveKind, base simpleTypeID, ws whitespaceMode, builtin builtinValidationKind) (simpleTypeID, error) {
	q, err := c.rt.Names.InternQName(ns, local)
	if err != nil {
		return noSimpleType, err
	}
	id := simpleTypeID(len(c.rt.SimpleTypes))
	facets := facetSet{}
	if base != noSimpleType && validUint32Index(uint32(base), len(c.rt.SimpleTypes)) {
		facets = cloneFacetSet(c.rt.SimpleTypes[base].Facets)
	}
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, simpleType{
		Name:       q,
		Variety:    varietyAtomic,
		Primitive:  primitive,
		Base:       base,
		Whitespace: ws,
		Facets:     facets,
		Builtin:    builtin,
	})
	return id, nil
}

func builtinValidationForLocal(local string) builtinValidationKind {
	switch local {
	case "integer", "nonPositiveInteger", "negativeInteger", "nonNegativeInteger", "positiveInteger",
		"long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		return builtinValidationInteger
	case "Name":
		return builtinValidationName
	case "NCName", "ID", "IDREF":
		return builtinValidationNCName
	case "ENTITY":
		return builtinValidationEntity
	case "NMTOKEN":
		return builtinValidationNMTOKEN
	case "language":
		return builtinValidationLanguage
	default:
		return builtinValidationNone
	}
}

func (c *compiler) addBuiltinListSimpleType(local string, item, base simpleTypeID, minLength *uint32) (simpleTypeID, error) {
	q, err := c.rt.Names.InternQName(xsdNamespaceURI, local)
	if err != nil {
		return noSimpleType, err
	}
	id := simpleTypeID(len(c.rt.SimpleTypes))
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, simpleType{
		Name:       q,
		Variety:    varietyList,
		Primitive:  primString,
		Base:       base,
		Whitespace: whitespaceCollapse,
		ListItem:   item,
		Facets:     facetSet{MinLength: minLength},
	})
	c.simpleDone[q] = id
	c.rt.GlobalTypes[q] = typeID{Kind: typeSimple, ID: uint32(id)}
	return id, nil
}

func (c *compiler) setBuiltinIntegerFacets(id simpleTypeID) {
	v := uint32(0)
	c.rt.SimpleTypes[id].Facets.FractionDigits = &v
}

func (c *compiler) setBuiltinMin(id simpleTypeID, v string) {
	lit := builtinDecimalLiteral(v)
	c.rt.SimpleTypes[id].Facets.MinInclusive = &lit
}

func (c *compiler) setBuiltinMax(id simpleTypeID, v string) {
	lit := builtinDecimalLiteral(v)
	c.rt.SimpleTypes[id].Facets.MaxInclusive = &lit
}

func (c *compiler) setBuiltinRange(id simpleTypeID, minValue, maxValue string) {
	c.setBuiltinMin(id, minValue)
	c.setBuiltinMax(id, maxValue)
}

func builtinDecimalLiteral(v string) compiledLiteral {
	dec, err := parseDecimalMode(v, decimalWithCanonical)
	if err != nil {
		return compiledLiteral{Lexical: v, Canonical: v}
	}
	return compiledLiteral{
		Lexical:   v,
		Canonical: dec.IntegerCanonical,
		Actual: actualValue{
			Kind:    primDecimal,
			Valid:   true,
			Decimal: dec,
		},
	}
}

func (c *compiler) addBuiltinAttribute(ns, local string, typ simpleTypeID) error {
	q, err := c.rt.Names.InternQName(ns, local)
	if err != nil {
		return err
	}
	id := attributeID(len(c.rt.Attributes))
	c.rt.Attributes = append(c.rt.Attributes, attributeDecl{Name: q, Type: typ})
	c.attributeDone[q] = id
	c.rt.GlobalAttributes[q] = id
	return nil
}

func (c *compiler) missingSimpleType() (simpleTypeID, error) {
	if c.missingSimple != noSimpleType {
		return c.missingSimple, nil
	}
	q, err := c.rt.Names.InternQName("", "missing")
	if err != nil {
		return noSimpleType, err
	}
	id := simpleTypeID(len(c.rt.SimpleTypes))
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, simpleType{
		Name:       q,
		Variety:    varietyAtomic,
		Primitive:  primString,
		Base:       c.rt.Builtin.AnySimpleType,
		Whitespace: whitespaceCollapse,
		Missing:    true,
	})
	c.missingSimple = id
	return id, nil
}
