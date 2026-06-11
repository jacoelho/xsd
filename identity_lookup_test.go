package xsd

import (
	"reflect"
	"testing"
)

func TestBuildIdentityFieldLookupMirrorsFields(t *testing.T) {
	elem := qName{Namespace: 1, Local: 1}
	attr := qName{Namespace: 1, Local: 2}
	otherAttr := qName{Namespace: 2, Local: 3}
	fields := []identityField{
		{Paths: []identityFieldPath{
			{Steps: []identityStep{{Name: elem}}},
			{Attr: true, Attribute: attr},
			{Attr: true, Attribute: attr, Steps: []identityStep{{Name: elem}}},
			{Attr: true, AttrWildcard: true},
			{Attr: true, AttrWildcard: true, AttrNamespaceSet: true, AttrNamespace: otherAttr.Namespace},
		}},
		{Paths: []identityFieldPath{
			{Attr: true, Attribute: otherAttr},
		}},
	}

	elementFields, attrFields, attrWildcardFields := buildIdentityFieldLookup(fields)

	wantElementFields := []compiledIdentityField{{
		Field: 0,
		Paths: []identityFieldPath{{Steps: []identityStep{{Name: elem}}}},
	}}
	wantAttrFields := map[qName][]compiledIdentityField{
		attr: {{
			Field: 0,
			Paths: []identityFieldPath{
				{Attr: true, Attribute: attr},
				{Attr: true, Attribute: attr, Steps: []identityStep{{Name: elem}}},
			},
		}},
		otherAttr: {{
			Field: 1,
			Paths: []identityFieldPath{{Attr: true, Attribute: otherAttr}},
		}},
	}
	wantAttrWildcardFields := []compiledIdentityField{{
		Field: 0,
		Paths: []identityFieldPath{
			{Attr: true, AttrWildcard: true},
			{Attr: true, AttrWildcard: true, AttrNamespaceSet: true, AttrNamespace: otherAttr.Namespace},
		},
	}}

	if !reflect.DeepEqual(elementFields, wantElementFields) {
		t.Fatalf("elementFields = %#v, want %#v", elementFields, wantElementFields)
	}
	if !reflect.DeepEqual(attrFields, wantAttrFields) {
		t.Fatalf("attrFields = %#v, want %#v", attrFields, wantAttrFields)
	}
	if !reflect.DeepEqual(attrWildcardFields, wantAttrWildcardFields) {
		t.Fatalf("attrWildcardFields = %#v, want %#v", attrWildcardFields, wantAttrWildcardFields)
	}
}
