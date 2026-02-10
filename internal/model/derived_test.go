package model

import "testing"

func TestIsValidlyDerivedFrom_NilInputs(t *testing.T) {
	base := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "Base",
		},
	}

	if IsValidlyDerivedFrom(nil, base) {
		t.Fatal("expected nil derived to return false")
	}
	if IsValidlyDerivedFrom(base, nil) {
		t.Fatal("expected nil base to return false")
	}
}

func TestIsValidlyDerivedFrom_SameQName(t *testing.T) {
	base := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "Shared",
		},
	}
	derived := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "Shared",
		},
	}

	if !IsValidlyDerivedFrom(derived, base) {
		t.Fatal("expected same QName to be valid derivation")
	}
}

func TestIsValidlyDerivedFrom_AnonymousTypes(t *testing.T) {
	base := &SimpleType{}
	derived := &SimpleType{}

	if IsValidlyDerivedFrom(derived, base) {
		t.Fatal("expected distinct anonymous types to be invalid derivation")
	}
	if !IsValidlyDerivedFrom(base, base) {
		t.Fatal("expected identical anonymous type to be valid derivation")
	}
}

func TestIsValidlyDerivedFrom_UnionMemberMatch(t *testing.T) {
	member := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "Member",
		},
	}
	unionBase := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "UnionBase",
		},
		Union: &UnionType{
			MemberTypes: []QName{member.QName},
		},
		MemberTypes: []Type{member},
	}
	derived := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "Member",
		},
	}

	if !IsValidlyDerivedFrom(derived, unionBase) {
		t.Fatal("expected union member QName to be valid derivation")
	}
}

func TestIsValidlyDerivedFrom_UnionMemberDerived(t *testing.T) {
	member := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "Member",
		},
	}
	derived := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "DerivedMember",
		},
		ResolvedBase: member,
	}
	unionBase := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "UnionBase",
		},
		Union: &UnionType{
			MemberTypes: []QName{member.QName},
		},
		MemberTypes: []Type{member},
	}

	if !IsValidlyDerivedFrom(derived, unionBase) {
		t.Fatal("expected derivation from union member to be valid")
	}
}

func TestIsValidlyDerivedFrom_UnionNoMembers(t *testing.T) {
	unionBase := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "UnionBase",
		},
		Union: &UnionType{
			MemberTypes: nil,
		},
	}
	derived := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "Derived",
		},
	}

	if IsValidlyDerivedFrom(derived, unionBase) {
		t.Fatal("expected union without members to be invalid")
	}
}

func TestIsValidlyDerivedFrom_UnionMembersWithoutUnionDef(t *testing.T) {
	member := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "Member",
		},
	}
	unionBase := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "UnionBase",
		},
		Union: &UnionType{
			MemberTypes: []QName{member.QName},
		},
		MemberTypes: []Type{member},
	}

	derived := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "Member",
		},
	}

	if !IsValidlyDerivedFrom(derived, unionBase) {
		t.Fatal("expected union member QName to be valid derivation without union definition")
	}
}
