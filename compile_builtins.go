package xsd

func (c *compiler) addBuiltins() {
	anySimple := c.addBuiltinAtomicSimpleType("anySimpleType", primString, noSimpleType, whitespacePreserve)
	c.rt.Builtin.AnySimpleType = anySimple
	c.addBuiltinStringTypes(anySimple)
	c.addBuiltinListTypes(anySimple)
	c.addBuiltinNumericTypes(anySimple)
	c.addBuiltinOtherPrimitiveTypes(anySimple)
	c.addBuiltinXMLAttributes()
	c.addBuiltinAnyType()
}

func (c *compiler) addBuiltinStringTypes(anySimple simpleTypeID) {
	c.rt.Builtin.String = c.addBuiltinAtomicSimpleType("string", primString, anySimple, whitespacePreserve)
	c.addBuiltinAtomicSimpleType("normalizedString", primString, c.rt.Builtin.String, whitespaceReplace)
	c.addBuiltinAtomicSimpleType("token", primString, c.rt.Builtin.String, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("language", primString, c.rt.Builtin.String, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("Name", primString, c.rt.Builtin.String, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("NCName", primString, c.rt.Builtin.String, whitespaceCollapse)
	c.rt.Builtin.NMTOKEN = c.addBuiltinAtomicSimpleType("NMTOKEN", primString, c.rt.Builtin.String, whitespaceCollapse)
	c.rt.Builtin.ID = c.addBuiltinAtomicSimpleType("ID", primString, c.rt.Builtin.String, whitespaceCollapse)
	c.rt.Builtin.IDREF = c.addBuiltinAtomicSimpleType("IDREF", primString, c.rt.Builtin.String, whitespaceCollapse)
}

func (c *compiler) addBuiltinListTypes(anySimple simpleTypeID) {
	minOne := uint32(1)
	c.rt.Builtin.IDREFS = c.addBuiltinListSimpleType("IDREFS", c.rt.Builtin.IDREF, anySimple, &minOne)
	c.rt.Builtin.NMTOKENS = c.addBuiltinListSimpleType("NMTOKENS", c.rt.Builtin.NMTOKEN, anySimple, &minOne)
	c.rt.Builtin.ENTITY = c.addBuiltinAtomicSimpleType("ENTITY", primString, c.rt.Builtin.String, whitespaceCollapse)
	c.rt.Builtin.ENTITIES = c.addBuiltinListSimpleType("ENTITIES", c.rt.Builtin.ENTITY, anySimple, &minOne)
}

func (c *compiler) addBuiltinNumericTypes(anySimple simpleTypeID) {
	c.rt.Builtin.Boolean = c.addBuiltinAtomicSimpleType("boolean", primBoolean, anySimple, whitespaceCollapse)
	c.rt.Builtin.Decimal = c.addBuiltinAtomicSimpleType("decimal", primDecimal, anySimple, whitespaceCollapse)
	c.rt.Builtin.Integer = c.addBuiltinAtomicSimpleType("integer", primDecimal, c.rt.Builtin.Decimal, whitespaceCollapse)
	c.setBuiltinIntegerFacets(c.rt.Builtin.Integer)
	c.addBuiltinIntegerDerivedTypes()
	c.addBuiltinAtomicSimpleType("float", primFloat, anySimple, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("double", primDouble, anySimple, whitespaceCollapse)
}

func (c *compiler) addBuiltinIntegerDerivedTypes() {
	nonPositive := c.addBuiltinAtomicSimpleType("nonPositiveInteger", primDecimal, c.rt.Builtin.Integer, whitespaceCollapse)
	c.setBuiltinIntegerFacets(nonPositive)
	c.setBuiltinMax(nonPositive, "0")
	negative := c.addBuiltinAtomicSimpleType("negativeInteger", primDecimal, nonPositive, whitespaceCollapse)
	c.setBuiltinIntegerFacets(negative)
	c.setBuiltinMax(negative, "-1")
	nonNegative := c.addBuiltinAtomicSimpleType("nonNegativeInteger", primDecimal, c.rt.Builtin.Integer, whitespaceCollapse)
	c.setBuiltinIntegerFacets(nonNegative)
	c.setBuiltinMin(nonNegative, "0")
	positive := c.addBuiltinAtomicSimpleType("positiveInteger", primDecimal, nonNegative, whitespaceCollapse)
	c.setBuiltinIntegerFacets(positive)
	c.setBuiltinMin(positive, "1")
	long := c.addBuiltinAtomicSimpleType("long", primDecimal, c.rt.Builtin.Integer, whitespaceCollapse)
	c.setBuiltinIntegerFacets(long)
	c.setBuiltinRange(long, "-9223372036854775808", "9223372036854775807")
	c.rt.Builtin.Int = c.addBuiltinAtomicSimpleType("int", primDecimal, long, whitespaceCollapse)
	c.setBuiltinIntegerFacets(c.rt.Builtin.Int)
	c.setBuiltinRange(c.rt.Builtin.Int, "-2147483648", "2147483647")
	c.addBuiltinSmallIntegerTypes(nonNegative)
}

func (c *compiler) addBuiltinSmallIntegerTypes(nonNegative simpleTypeID) {
	short := c.addBuiltinAtomicSimpleType("short", primDecimal, c.rt.Builtin.Int, whitespaceCollapse)
	c.setBuiltinIntegerFacets(short)
	c.setBuiltinRange(short, "-32768", "32767")
	byteType := c.addBuiltinAtomicSimpleType("byte", primDecimal, short, whitespaceCollapse)
	c.setBuiltinIntegerFacets(byteType)
	c.setBuiltinRange(byteType, "-128", "127")
	unsignedLong := c.addBuiltinAtomicSimpleType("unsignedLong", primDecimal, nonNegative, whitespaceCollapse)
	c.setBuiltinIntegerFacets(unsignedLong)
	c.setBuiltinMax(unsignedLong, "18446744073709551615")
	unsignedInt := c.addBuiltinAtomicSimpleType("unsignedInt", primDecimal, unsignedLong, whitespaceCollapse)
	c.setBuiltinIntegerFacets(unsignedInt)
	c.setBuiltinMax(unsignedInt, "4294967295")
	unsignedShort := c.addBuiltinAtomicSimpleType("unsignedShort", primDecimal, unsignedInt, whitespaceCollapse)
	c.setBuiltinIntegerFacets(unsignedShort)
	c.setBuiltinMax(unsignedShort, "65535")
	unsignedByte := c.addBuiltinAtomicSimpleType("unsignedByte", primDecimal, unsignedShort, whitespaceCollapse)
	c.setBuiltinIntegerFacets(unsignedByte)
	c.setBuiltinMax(unsignedByte, "255")
}

func (c *compiler) addBuiltinOtherPrimitiveTypes(anySimple simpleTypeID) {
	c.rt.Builtin.AnyURI = c.addBuiltinAtomicSimpleType("anyURI", primAnyURI, c.rt.Builtin.String, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("duration", primDuration, anySimple, whitespaceCollapse)
	c.rt.Builtin.DateTime = c.addBuiltinAtomicSimpleType("dateTime", primDateTime, anySimple, whitespaceCollapse)
	c.rt.Builtin.Time = c.addBuiltinAtomicSimpleType("time", primTime, anySimple, whitespaceCollapse)
	c.rt.Builtin.Date = c.addBuiltinAtomicSimpleType("date", primDate, anySimple, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("gYearMonth", primGYearMonth, anySimple, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("gYear", primGYear, anySimple, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("gMonthDay", primGMonthDay, anySimple, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("gDay", primGDay, anySimple, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("gMonth", primGMonth, anySimple, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("hexBinary", primHexBinary, anySimple, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("base64Binary", primBase64Binary, anySimple, whitespaceCollapse)
	c.rt.Builtin.qName = c.addBuiltinAtomicSimpleType("QName", primQName, anySimple, whitespaceCollapse)
	c.addBuiltinAtomicSimpleType("NOTATION", primNotation, anySimple, whitespaceCollapse)
}

func (c *compiler) addBuiltinXMLAttributes() {
	c.addBuiltinAttribute(xmlNamespaceURI, "base", c.rt.Builtin.AnyURI)
	c.addBuiltinAttribute(xmlNamespaceURI, "id", c.rt.Builtin.ID)
	c.addBuiltinAttribute(xmlNamespaceURI, "lang", c.rt.Builtin.String)
	c.addBuiltinAttribute(xmlNamespaceURI, "space", c.rt.Builtin.String)
	c.addBuiltinAttribute(xlinkNamespaceURI, "type", c.rt.Builtin.String)
	c.addBuiltinAttribute(xlinkNamespaceURI, "href", c.rt.Builtin.AnyURI)
	c.addBuiltinAttribute(xlinkNamespaceURI, "role", c.rt.Builtin.AnyURI)
	c.addBuiltinAttribute(xlinkNamespaceURI, "arcrole", c.rt.Builtin.AnyURI)
	c.addBuiltinAttribute(xlinkNamespaceURI, "title", c.rt.Builtin.String)
	c.addBuiltinAttribute(xlinkNamespaceURI, "show", c.rt.Builtin.String)
	c.addBuiltinAttribute(xlinkNamespaceURI, "actuate", c.rt.Builtin.String)
}

func (c *compiler) addBuiltinAnyType() {
	anyWildcard := wildcardID(len(c.rt.Wildcards))
	c.rt.Wildcards = append(c.rt.Wildcards, wildcard{Mode: wildAny, Process: processLax})
	attrs := attributeUseSet{wildcard: anyWildcard}
	attrSet := attributeUseSetID(len(c.rt.AttributeUseSets))
	c.rt.AttributeUseSets = append(c.rt.AttributeUseSets, attrs)
	anyModel := contentModel{Kind: modelAny, Mixed: true}
	modelID := contentModelID(len(c.rt.Models))
	c.rt.Models = append(c.rt.Models, anyModel)
	q := c.rt.Names.InternQName(xsdNamespaceURI, "anyType")
	complexID := complexTypeID(len(c.rt.ComplexTypes))
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, complexType{Name: q, Content: modelID, Attrs: attrSet, Mixed: true, Base: typeID{Kind: typeComplex, ID: uint32(noComplexType)}})
	c.rt.Builtin.AnyType = complexID
	c.complexDone[q] = complexID
	c.rt.GlobalTypes[q] = typeID{Kind: typeComplex, ID: uint32(complexID)}
}

func (c *compiler) addBuiltinAtomicSimpleType(local string, primitive primitiveKind, base simpleTypeID, ws whitespaceMode) simpleTypeID {
	q := c.rt.Names.InternQName(xsdNamespaceURI, local)
	id := simpleTypeID(len(c.rt.SimpleTypes))
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, simpleType{Name: q, Variety: varietyAtomic, Primitive: primitive, Base: base, Whitespace: ws})
	c.simpleDone[q] = id
	c.rt.GlobalTypes[q] = typeID{Kind: typeSimple, ID: uint32(id)}
	return id
}

func (c *compiler) addBuiltinListSimpleType(local string, item, base simpleTypeID, minLength *uint32) simpleTypeID {
	q := c.rt.Names.InternQName(xsdNamespaceURI, local)
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
	return id
}

func (c *compiler) setBuiltinIntegerFacets(id simpleTypeID) {
	v := uint32(0)
	c.rt.SimpleTypes[id].Facets.FractionDigits = &v
}

func (c *compiler) setBuiltinMin(id simpleTypeID, v string) {
	c.rt.SimpleTypes[id].Facets.MinInclusive = &compiledLiteral{Lexical: v, Canonical: v}
}

func (c *compiler) setBuiltinMax(id simpleTypeID, v string) {
	c.rt.SimpleTypes[id].Facets.MaxInclusive = &compiledLiteral{Lexical: v, Canonical: v}
}

func (c *compiler) setBuiltinRange(id simpleTypeID, minValue, maxValue string) {
	c.setBuiltinMin(id, minValue)
	c.setBuiltinMax(id, maxValue)
}

func (c *compiler) addBuiltinAttribute(ns, local string, typ simpleTypeID) {
	q := c.rt.Names.InternQName(ns, local)
	id := attributeID(len(c.rt.Attributes))
	c.rt.Attributes = append(c.rt.Attributes, attributeDecl{Name: q, Type: typ})
	c.attributeDone[q] = id
	c.rt.GlobalAttributes[q] = id
}

func (c *compiler) missingSimpleType() simpleTypeID {
	if c.missingSimple != noSimpleType {
		return c.missingSimple
	}
	id := simpleTypeID(len(c.rt.SimpleTypes))
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, simpleType{
		Name:       c.rt.Names.InternQName("", "missing"),
		Variety:    varietyAtomic,
		Primitive:  primString,
		Base:       c.rt.Builtin.AnySimpleType,
		Whitespace: whitespaceCollapse,
		Missing:    true,
	})
	c.missingSimple = id
	return id
}
