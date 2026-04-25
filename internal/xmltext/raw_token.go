package xmltext

type rawToken struct {
	attrs        []attrSpan
	attrNeeds    []bool
	attrRaw      []span
	attrRawNeeds []bool
	raw          span
	text         span
	name         qnameSpan
	line         int
	column       int
	kind         Kind
	textNeeds    bool
	textRawNeeds bool
	isXMLDecl    bool
}
