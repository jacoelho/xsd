package xsd

import "testing"

func TestFloatDoubleFixedUseZeroValueSpace(t *testing.T) {
	engine := mustCompile(t, `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="floatElement" type="xs:float" fixed="-0"/>
  <xs:element name="doubleElement" type="xs:double" fixed="0"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="floatAttr" type="xs:float" fixed="-0"/>
      <xs:attribute name="doubleAttr" type="xs:double" fixed="0"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	mustValidate(t, engine, `<floatElement>0</floatElement>`)
	mustValidate(t, engine, `<doubleElement>-0</doubleElement>`)
	mustValidate(t, engine, `<root floatAttr="0" doubleAttr="-0"/>`)
	mustNotValidate(t, engine, `<floatElement>1</floatElement>`, ErrValidationElement)
	mustNotValidate(t, engine, `<root floatAttr="1" doubleAttr="-0"/>`, ErrValidationAttribute)
}
