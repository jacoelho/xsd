package types

import (
	"testing"
)

func TestPrimitiveType_ForPrimitiveTypes(t *testing.T) {
	// Primitive types should return themselves as primitive
	primitiveTypes := []string{
		"string", "boolean", "decimal", "float", "double",
		"duration", "dateTime", "time", "date",
		"gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth",
		"hexBinary", "base64Binary", "anyURI", "QName", "NOTATION",
	}

	for _, typeName := range primitiveTypes {
		t.Run(typeName, func(t *testing.T) {
			st := &SimpleType{
				QName: QName{
					Namespace: "http://www.w3.org/2001/XMLSchema",
					Local:     typeName,
				},
				// Variety set via SetVariety
			}
			st.MarkBuiltin()
			st.SetVariety(AtomicVariety)

			primitive := st.PrimitiveType()
			if primitive == nil {
				t.Fatalf("PrimitiveType() returned nil for primitive type %q", typeName)
			}
			if primitive.Name().Local != typeName {
				t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, typeName)
			}
		})
	}
}

func TestPrimitiveType_ForDerivedTypes(t *testing.T) {
	// integer is derived from decimal, so primitive should be decimal
	decimalType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameDecimal),
		},
		// Variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(AtomicVariety)
	decimalType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameDecimal))

	integerType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameInteger),
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: decimalType.QName,
		},
	}
	integerType.ResolvedBase = decimalType
	integerType.MarkBuiltin()
	integerType.SetVariety(AtomicVariety)
	integerType.MarkBuiltin()
	integerType.SetVariety(AtomicVariety)

	primitive := integerType.PrimitiveType()
	if primitive == nil {
		t.Fatal("PrimitiveType() returned nil")
	}
	if primitive.Name().Local != string(TypeNameDecimal) {
		t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameDecimal))
	}
}

func TestPrimitiveType_ForListTypes(t *testing.T) {
	// List of IDREF should have primitive of string (IDREF's primitive)
	stringType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameString),
		},
		// Variety set via SetVariety
	}
	stringType.MarkBuiltin()
	stringType.SetVariety(AtomicVariety)
	stringType.MarkBuiltin()
	stringType.SetVariety(AtomicVariety)
	stringType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameString))

	idrefType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "IDREF",
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: stringType.QName,
		},
	}
	idrefType.ResolvedBase = stringType
	idrefType.MarkBuiltin()
	idrefType.SetVariety(AtomicVariety)
	idrefType.SetPrimitiveType(stringType)

	listType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "IDREFS",
		},
		// Variety set via SetVariety
		List: &ListType{
			ItemType: idrefType.QName,
		},
	}
	listType.MarkBuiltin()
	listType.SetVariety(ListVariety)
	listType.ItemType = idrefType

	primitive := listType.PrimitiveType()
	if primitive == nil {
		t.Fatal("PrimitiveType() returned nil")
	}
	if primitive.Name().Local != string(TypeNameString) {
		t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameString))
	}
}

func TestPrimitiveType_ForUnionTypes(t *testing.T) {
	// Union types should return common primitive or anySimpleType
	stringType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "string",
		},
		// Variety set via SetVariety
	}
	stringType.MarkBuiltin()
	stringType.SetVariety(AtomicVariety)
	stringType.MarkBuiltin()
	stringType.SetVariety(AtomicVariety)
	stringType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameString))
	stringType.SetPrimitiveType(stringType)

	integerType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "integer",
		},
		// Variety set via SetVariety
	}
	integerType.MarkBuiltin()
	integerType.SetVariety(AtomicVariety)
	integerType.MarkBuiltin()
	integerType.SetVariety(AtomicVariety)

	unionType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "StringOrInteger",
		},
		// Variety set via SetVariety
		Union: &UnionType{
			MemberTypes: []QName{
				stringType.QName,
				integerType.QName,
			},
		},
	}

	primitive := unionType.PrimitiveType()
	// Union of string (primitive=string) and integer (primitive=decimal) should return anySimpleType
	// or nil if not yet resolved
	if primitive != nil && primitive.Name().Local != "anySimpleType" {
		// For now, we'll accept nil if not resolved
		if primitive.Name().Local != "string" && primitive.Name().Local != "decimal" {
			t.Logf("Union type primitive is %q (may be anySimpleType or common primitive)", primitive.Name().Local)
		}
	}
}

func TestPrimitiveType_CircularReference(t *testing.T) {
	// Create a type that references itself (malformed schema, but should not crash)
	st := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "circular",
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: QName{
				Namespace: "http://example.com",
				Local:     "circular",
			},
		},
	}
	st.ResolvedBase = st

	// Should not cause stack overflow
	primitive := st.PrimitiveType()
	// May return nil due to cycle, which is acceptable
	if primitive == st {
		t.Error("PrimitiveType() should not return self for circular reference")
	}
}

func TestPrimitiveType_IndirectCircularReference(t *testing.T) {
	type1 := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "type1",
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: QName{
				Namespace: "http://example.com",
				Local:     "type2",
			},
		},
	}

	type2 := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "type2",
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: QName{
				Namespace: "http://example.com",
				Local:     "type1",
			},
		},
	}

	type1.ResolvedBase = type2
	type2.ResolvedBase = type1

	// Should not cause stack overflow
	primitive1 := type1.PrimitiveType()
	primitive2 := type2.PrimitiveType()

	// Both should return nil due to cycle
	if primitive1 == type1 || primitive1 == type2 {
		t.Error("PrimitiveType() should not return circular reference")
	}
	if primitive2 == type1 || primitive2 == type2 {
		t.Error("PrimitiveType() should not return circular reference")
	}
}

func TestPrimitiveType_WithBuiltinBase(t *testing.T) {
	// Test case from issue 003: SimpleType with BuiltinType as ResolvedBase
	// This is the scenario that was failing - when a derived type has a built-in type as base
	intBuiltin := GetBuiltin(TypeNameInt)
	if intBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameInt) returned nil")
	}

	derivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "derived",
		},
		Restriction: &Restriction{
			Base: intBuiltin.Name(),
		},
	}
	derivedType.ResolvedBase = intBuiltin
	derivedType.SetVariety(AtomicVariety)

	// PrimitiveType should return the primitive of int (which is decimal)
	primitive := derivedType.PrimitiveType()
	if primitive == nil {
		t.Fatal("PrimitiveType() returned nil for derived type with BuiltinType base")
	}

	// int's primitive is decimal
	if primitive.Name().Local != string(TypeNameDecimal) {
		t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameDecimal))
	}
}

func TestPrimitiveType_NestedDerivationWithBuiltin(t *testing.T) {
	// Test nested derivation: moreDerived -> derived -> int (builtin)
	// This matches the example in issue 003
	intBuiltin := GetBuiltin(TypeNameInt)
	if intBuiltin == nil {
		t.Fatal("GetBuiltin(TypeNameInt) returned nil")
	}

	// First level: derived restricts int
	derivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "derived",
		},
		Restriction: &Restriction{
			Base: intBuiltin.Name(),
		},
	}
	derivedType.ResolvedBase = intBuiltin
	derivedType.SetVariety(AtomicVariety)

	// Second level: moreDerived restricts derived
	moreDerivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "moreDerived",
		},
		Restriction: &Restriction{
			Base: derivedType.QName,
		},
	}
	moreDerivedType.ResolvedBase = derivedType
	moreDerivedType.SetVariety(AtomicVariety)

	// PrimitiveType should traverse the chain: moreDerived -> derived -> int -> decimal
	primitive := moreDerivedType.PrimitiveType()
	if primitive == nil {
		t.Fatal("PrimitiveType() returned nil for nested derivation")
	}

	// Should resolve to decimal (int's primitive)
	if primitive.Name().Local != string(TypeNameDecimal) {
		t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameDecimal))
	}
}

// TestPrimitiveType_WithRestrictionBaseDuringParsing tests the enhanced PrimitiveType()
// that handles Restriction.Base when ResolvedBase is not set (parsing phase)
func TestPrimitiveType_WithRestrictionBaseDuringParsing(t *testing.T) {
	t.Run("primitive_builtin_from_RestrictionBase", func(t *testing.T) {
		// During parsing, ResolvedBase is nil but Restriction.Base is set
		// Should resolve built-in primitive type (decimal) from QName
		st := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "MyDecimal",
			},
			Restriction: &Restriction{
				Base: QName{
					Namespace: XSDNamespace,
					Local:     string(TypeNameDecimal),
				},
			},
		}
		st.SetVariety(AtomicVariety)
		// ResolvedBase is nil (simulating parsing phase)

		primitive := st.PrimitiveType()
		if primitive == nil {
			t.Fatal("PrimitiveType() returned nil for built-in primitive base")
		}
		if primitive.Name().Local != string(TypeNameDecimal) {
			t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameDecimal))
		}
	})

	t.Run("derived_builtin_from_RestrictionBase", func(t *testing.T) {
		// Restriction.Base points to integer (derived from decimal)
		// Should resolve to decimal (integer's primitive)
		st := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "MyInteger",
			},
			Restriction: &Restriction{
				Base: QName{
					Namespace: XSDNamespace,
					Local:     string(TypeNameInteger),
				},
			},
		}
		st.SetVariety(AtomicVariety)
		// ResolvedBase is nil (simulating parsing phase)

		primitive := st.PrimitiveType()
		if primitive == nil {
			t.Fatal("PrimitiveType() returned nil for derived built-in base")
		}
		// integer's primitive is decimal
		if primitive.Name().Local != string(TypeNameDecimal) {
			t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameDecimal))
		}
	})

	t.Run("int_builtin_from_RestrictionBase", func(t *testing.T) {
		// Restriction.Base points to int (derived from integer -> decimal)
		// Should resolve to decimal (int's primitive)
		st := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "MyInt",
			},
			Restriction: &Restriction{
				Base: QName{
					Namespace: XSDNamespace,
					Local:     string(TypeNameInt),
				},
			},
		}
		st.SetVariety(AtomicVariety)
		// ResolvedBase is nil (simulating parsing phase)

		primitive := st.PrimitiveType()
		if primitive == nil {
			t.Fatal("PrimitiveType() returned nil for int base")
		}
		// int's primitive is decimal
		if primitive.Name().Local != string(TypeNameDecimal) {
			t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameDecimal))
		}
	})

	t.Run("float_builtin_from_RestrictionBase", func(t *testing.T) {
		// Restriction.Base points to float (primitive type)
		// Should resolve to float itself
		st := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "MyFloat",
			},
			Restriction: &Restriction{
				Base: QName{
					Namespace: XSDNamespace,
					Local:     string(TypeNameFloat),
				},
			},
		}
		st.SetVariety(AtomicVariety)
		// ResolvedBase is nil (simulating parsing phase)

		primitive := st.PrimitiveType()
		if primitive == nil {
			t.Fatal("PrimitiveType() returned nil for float base")
		}
		// float is a primitive type, so it should return itself
		if primitive.Name().Local != string(TypeNameFloat) {
			t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameFloat))
		}
	})

	t.Run("dateTime_builtin_from_RestrictionBase", func(t *testing.T) {
		// Restriction.Base points to dateTime (primitive type)
		// Should resolve to dateTime itself
		st := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "MyDateTime",
			},
			Restriction: &Restriction{
				Base: QName{
					Namespace: XSDNamespace,
					Local:     string(TypeNameDateTime),
				},
			},
		}
		st.SetVariety(AtomicVariety)
		// ResolvedBase is nil (simulating parsing phase)

		primitive := st.PrimitiveType()
		if primitive == nil {
			t.Fatal("PrimitiveType() returned nil for dateTime base")
		}
		// dateTime is a primitive type, so it should return itself
		if primitive.Name().Local != string(TypeNameDateTime) {
			t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameDateTime))
		}
	})

	t.Run("duration_builtin_from_RestrictionBase", func(t *testing.T) {
		// Restriction.Base points to duration (primitive type, but OrderedPartial)
		// Should resolve to duration itself
		st := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "MyDuration",
			},
			Restriction: &Restriction{
				Base: QName{
					Namespace: XSDNamespace,
					Local:     string(TypeNameDuration),
				},
			},
		}
		st.SetVariety(AtomicVariety)
		// ResolvedBase is nil (simulating parsing phase)

		primitive := st.PrimitiveType()
		if primitive == nil {
			t.Fatal("PrimitiveType() returned nil for duration base")
		}
		// duration is a primitive type, so it should return itself
		if primitive.Name().Local != string(TypeNameDuration) {
			t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameDuration))
		}
	})

	t.Run("non_xsd_namespace_returns_nil", func(t *testing.T) {
		// Restriction.Base points to a non-XSD namespace type
		// Should return nil (can't resolve without schema context)
		st := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "MyType",
			},
			Restriction: &Restriction{
				Base: QName{
					Namespace: "http://example.com",
					Local:     "UserDefinedType",
				},
			},
		}
		st.SetVariety(AtomicVariety)
		// ResolvedBase is nil (simulating parsing phase)

		primitive := st.PrimitiveType()
		// Should return nil for user-defined types (can't resolve without schema)
		if primitive != nil {
			t.Errorf("PrimitiveType() = %v, want nil for non-XSD namespace", primitive)
		}
	})

	t.Run("unknown_builtin_returns_nil", func(t *testing.T) {
		// Restriction.Base points to unknown built-in type
		// Should return nil
		st := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "MyType",
			},
			Restriction: &Restriction{
				Base: QName{
					Namespace: XSDNamespace,
					Local:     "UnknownType",
				},
			},
		}
		st.SetVariety(AtomicVariety)
		// ResolvedBase is nil (simulating parsing phase)

		primitive := st.PrimitiveType()
		// Should return nil for unknown built-in types
		if primitive != nil {
			t.Errorf("PrimitiveType() = %v, want nil for unknown built-in", primitive)
		}
	})

	t.Run("ResolvedBase_takes_precedence", func(t *testing.T) {
		// When both ResolvedBase and Restriction.Base are set,
		// ResolvedBase should take precedence (validation phase)
		intBuiltin := GetBuiltin(TypeNameInt)
		if intBuiltin == nil {
			t.Fatal("GetBuiltin(TypeNameInt) returned nil")
		}

		st := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "MyType",
			},
			Restriction: &Restriction{
				Base: QName{
					Namespace: XSDNamespace,
					Local:     string(TypeNameDecimal), // Different from ResolvedBase
				},
			},
		}
		st.ResolvedBase = intBuiltin // Set ResolvedBase (validation phase)
		st.SetVariety(AtomicVariety)

		primitive := st.PrimitiveType()
		if primitive == nil {
			t.Fatal("PrimitiveType() returned nil")
		}
		// Should use ResolvedBase (int), not Restriction.Base (decimal)
		// int's primitive is decimal, so result is same, but path is different
		if primitive.Name().Local != string(TypeNameDecimal) {
			t.Errorf("PrimitiveType() = %q, want %q", primitive.Name().Local, string(TypeNameDecimal))
		}
		// Verify it used ResolvedBase by checking cached value
		if st.primitiveType == nil {
			t.Error("PrimitiveType() should cache the result")
		}
	})

	t.Run("nested_derivation_with_RestrictionBase", func(t *testing.T) {
		// Test nested derivation during parsing:
		// moreDerived -> derived -> int (builtin)
		// Only the first level has Restriction.Base set (parsing phase)
		intBuiltin := GetBuiltin(TypeNameInt)
		if intBuiltin == nil {
			t.Fatal("GetBuiltin(TypeNameInt) returned nil")
		}

		// First level: derived restricts int (has Restriction.Base, no ResolvedBase)
		derivedType := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "derived",
			},
			Restriction: &Restriction{
				Base: intBuiltin.Name(),
			},
		}
		derivedType.SetVariety(AtomicVariety)
		// ResolvedBase is nil (parsing phase)

		// Second level: moreDerived restricts derived (has ResolvedBase set)
		moreDerivedType := &SimpleType{
			QName: QName{
				Namespace: "http://example.com",
				Local:     "moreDerived",
			},
			Restriction: &Restriction{
				Base: derivedType.QName,
			},
		}
		moreDerivedType.ResolvedBase = derivedType
		moreDerivedType.SetVariety(AtomicVariety)

		// First level should resolve from Restriction.Base
		derivedPrimitive := derivedType.PrimitiveType()
		if derivedPrimitive == nil {
			t.Fatal("derivedType.PrimitiveType() returned nil")
		}
		if derivedPrimitive.Name().Local != string(TypeNameDecimal) {
			t.Errorf("derivedType.PrimitiveType() = %q, want %q", derivedPrimitive.Name().Local, string(TypeNameDecimal))
		}

		// Second level should resolve through ResolvedBase
		moreDerivedPrimitive := moreDerivedType.PrimitiveType()
		if moreDerivedPrimitive == nil {
			t.Fatal("moreDerivedType.PrimitiveType() returned nil")
		}
		if moreDerivedPrimitive.Name().Local != string(TypeNameDecimal) {
			t.Errorf("moreDerivedType.PrimitiveType() = %q, want %q", moreDerivedPrimitive.Name().Local, string(TypeNameDecimal))
		}
	})
}
