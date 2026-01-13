package xmltext

type rawToken struct {
	text         span
	raw          span
	name         qnameSpan
	attrNeeds    []bool
	attrRaw      []span
	attrRawNeeds []bool
	attrs        []attrSpan
	line         int
	column       int
	kind         Kind
	textNeeds    bool
	textRawNeeds bool
	isXMLDecl    bool
}
