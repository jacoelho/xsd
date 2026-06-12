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

// addBuiltinStringTypes builds the spec derivation chain
// string ← normalizedString ← token ← {language, Name ← NCName ← {ID, IDREF,
// ENTITY}, NMTOKEN}; derivation checks walk these Base links.
func (c *compiler) addBuiltinStringTypes(anySimple simpleTypeID) error {
	var err error
	c.rt.Builtin.String, err = c.addBuiltinAtomicSimpleType("string", primString, anySimple, whitespacePreserve)
	if err != nil {
		return err
	}
	normalizedString, err := c.addBuiltinAtomicSimpleType("normalizedString", primString, c.rt.Builtin.String, whitespaceReplace)
	if err != nil {
		return err
	}
	token, err := c.addBuiltinAtomicSimpleType(xsdValueToken, primString, normalizedString, whitespaceCollapse)
	if err != nil {
		return err
	}
	if _, err = c.addBuiltinAtomicSimpleType("language", primString, token, whitespaceCollapse); err != nil {
		return err
	}
	name, err := c.addBuiltinAtomicSimpleType("Name", primString, token, whitespaceCollapse)
	if err != nil {
		return err
	}
	ncName, err := c.addBuiltinAtomicSimpleType("NCName", primString, name, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.Builtin.NMTOKEN, err = c.addBuiltinAtomicSimpleType("NMTOKEN", primString, token, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.Builtin.ID, err = c.addBuiltinAtomicSimpleType("ID", primString, ncName, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.SimpleTypes[c.rt.Builtin.ID].Identity = simpleIdentityID
	c.rt.Builtin.IDREF, err = c.addBuiltinAtomicSimpleType("IDREF", primString, ncName, whitespaceCollapse)
	if err != nil {
		return err
	}
	c.rt.SimpleTypes[c.rt.Builtin.IDREF].Identity = simpleIdentityIDREF
	c.rt.Builtin.ENTITY, err = c.addBuiltinAtomicSimpleType("ENTITY", primString, ncName, whitespaceCollapse)
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
	c.setBuiltinMax(unsignedInt, maxUint32Text)
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
	c.rt.Builtin.AnyURI, err = c.addBuiltinAtomicSimpleType("anyURI", primAnyURI, anySimple, whitespaceCollapse)
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
	xmlLang, err := c.addInternalAtomicSimpleType(xmlNamespaceURI, xmlAttrLang, primString, c.rt.Builtin.String, whitespaceCollapse, builtinValidationXMLLang)
	if err != nil {
		return err
	}
	xmlSpace, err := c.addInternalAtomicSimpleType(xmlNamespaceURI, xmlAttrSpace, primString, c.rt.Builtin.String, whitespaceCollapse, builtinValidationXMLSpace)
	if err != nil {
		return err
	}
	for _, attr := range []struct {
		ns    string
		local string
		typ   simpleTypeID
	}{
		{xmlNamespaceURI, xmlAttrBase, c.rt.Builtin.AnyURI},
		{xmlNamespaceURI, xmlAttrID, c.rt.Builtin.ID},
		{xmlNamespaceURI, xmlAttrLang, xmlLang},
		{xmlNamespaceURI, xmlAttrSpace, xmlSpace},
		{xlinkNamespaceURI, xlinkAttrType, c.rt.Builtin.String},
		{xlinkNamespaceURI, xlinkAttrHref, c.rt.Builtin.AnyURI},
		{xlinkNamespaceURI, xlinkAttrRole, c.rt.Builtin.AnyURI},
		{xlinkNamespaceURI, xlinkAttrArcrole, c.rt.Builtin.AnyURI},
		{xlinkNamespaceURI, xlinkAttrTitle, c.rt.Builtin.String},
		{xlinkNamespaceURI, xlinkAttrShow, c.rt.Builtin.String},
		{xlinkNamespaceURI, xlinkAttrActuate, c.rt.Builtin.String},
	} {
		if err := c.addBuiltinAttribute(attr.ns, attr.local, attr.typ); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) addBuiltinAnyType() error {
	anyWildcard, err := c.addWildcard(wildcard{Mode: wildAny, Process: processLax})
	if err != nil {
		return err
	}
	attrs := attributeUseSet{Wildcard: anyWildcard}
	attrSet, err := nextAttributeUseSetID(len(c.rt.AttributeUseSets))
	if err != nil {
		return err
	}
	c.rt.AttributeUseSets = append(c.rt.AttributeUseSets, attrs)
	modelID, err := c.addModel(contentModel{Kind: modelAny, Mixed: true})
	if err != nil {
		return err
	}
	q, err := c.rt.Names.InternQName(xsdNamespaceURI, "anyType")
	if err != nil {
		return err
	}
	complexID, err := nextComplexTypeID(len(c.rt.ComplexTypes))
	if err != nil {
		return err
	}
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, complexType{Name: q, Content: modelID, Attrs: attrSet, TextType: noSimpleType, ContentKind: contentMixed})
	c.rt.Builtin.AnyType = complexID
	c.complexDone[q] = complexID
	c.rt.GlobalTypes[q] = complexRef(complexID)
	return nil
}

func (c *compiler) addBuiltinAtomicSimpleType(local string, primitive primitiveKind, base simpleTypeID, ws whitespaceMode) (simpleTypeID, error) {
	id, q, err := c.addAtomicSimpleType(xsdNamespaceURI, local, primitive, base, ws, builtinValidationForLocal(local))
	if err != nil {
		return noSimpleType, err
	}
	c.simpleDone[q] = id
	c.rt.GlobalTypes[q] = simpleRef(id)
	return id, nil
}

func (c *compiler) addInternalAtomicSimpleType(ns, local string, primitive primitiveKind, base simpleTypeID, ws whitespaceMode, builtin builtinValidationKind) (simpleTypeID, error) {
	id, _, err := c.addAtomicSimpleType(ns, local, primitive, base, ws, builtin)
	return id, err
}

func (c *compiler) addAtomicSimpleType(ns, local string, primitive primitiveKind, base simpleTypeID, ws whitespaceMode, builtin builtinValidationKind) (simpleTypeID, qName, error) {
	q, err := c.rt.Names.InternQName(ns, local)
	if err != nil {
		return noSimpleType, qName{}, err
	}
	id, err := nextSimpleTypeID(len(c.rt.SimpleTypes))
	if err != nil {
		return noSimpleType, qName{}, err
	}
	facets := facetSet{}
	if baseType, ok := c.rt.simpleType(base); ok {
		facets = cloneFacetSet(baseType.Facets)
	}
	st := simpleType{
		Name:       q,
		Variety:    varietyAtomic,
		Primitive:  primitive,
		Base:       base,
		ListItem:   noSimpleType,
		Whitespace: ws,
		Facets:     facets,
		Builtin:    builtin,
	}
	st.Identity = c.rt.derivedSimpleIdentity(st)
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, st)
	return id, q, nil
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
	id, err := nextSimpleTypeID(len(c.rt.SimpleTypes))
	if err != nil {
		return noSimpleType, err
	}
	st := simpleType{
		Name:       q,
		Variety:    varietyList,
		Primitive:  primString,
		Base:       base,
		Whitespace: whitespaceCollapse,
		ListItem:   item,
		Facets:     listLengthFacets(minLength),
	}
	st.Identity = c.rt.derivedSimpleIdentity(st)
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, st)
	c.simpleDone[q] = id
	c.rt.GlobalTypes[q] = simpleRef(id)
	return id, nil
}

func listLengthFacets(minLength *uint32) facetSet {
	f := facetSet{MinLength: minLength}
	if minLength != nil {
		f.Present |= facetFlagMinLength
	}
	return f
}

func (c *compiler) setBuiltinIntegerFacets(id simpleTypeID) {
	v := uint32(0)
	c.rt.SimpleTypes[id].Facets.FractionDigits = &v
	c.rt.SimpleTypes[id].Facets.Present |= facetFlagFractionDigits
}

func (c *compiler) setBuiltinMin(id simpleTypeID, v string) {
	lit := builtinDecimalLiteral(v)
	c.rt.SimpleTypes[id].Facets.MinInclusive = &lit
	c.rt.SimpleTypes[id].Facets.Present |= facetFlagMinInclusive
}

func (c *compiler) setBuiltinMax(id simpleTypeID, v string) {
	lit := builtinDecimalLiteral(v)
	c.rt.SimpleTypes[id].Facets.MaxInclusive = &lit
	c.rt.SimpleTypes[id].Facets.Present |= facetFlagMaxInclusive
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
	id, err := nextAttributeID(len(c.rt.Attributes))
	if err != nil {
		return err
	}
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
	id, err := nextSimpleTypeID(len(c.rt.SimpleTypes))
	if err != nil {
		return noSimpleType, err
	}
	st := simpleType{
		Name:       q,
		Variety:    varietyAtomic,
		Primitive:  primString,
		Base:       c.rt.Builtin.AnySimpleType,
		ListItem:   noSimpleType,
		Whitespace: whitespaceCollapse,
		Missing:    true,
	}
	st.Identity = c.rt.derivedSimpleIdentity(st)
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, st)
	c.missingSimple = id
	return id, nil
}
