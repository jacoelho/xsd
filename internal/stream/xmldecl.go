package stream

import "github.com/jacoelho/xsd/internal/lex"

// UTF8BOM is the byte order mark accepted before XML input.
var UTF8BOM = []byte{0xEF, 0xBB, 0xBF}

// XMLDeclarationPrefixLen is the minimum bytes needed to recognize "<?xml ".
const XMLDeclarationPrefixLen = len("<?xml ")

// StartsXMLDeclaration reports whether buf starts with an XML declaration.
func StartsXMLDeclaration(buf []byte) bool {
	const declLen = len("<?xml")
	return len(buf) > declLen &&
		buf[0] == '<' &&
		buf[1] == '?' &&
		buf[2] == 'x' &&
		buf[3] == 'm' &&
		buf[4] == 'l' &&
		lex.IsXMLWhitespaceByte(buf[declLen])
}

// DeclaredEncoding returns the XML declaration encoding value, if present.
func DeclaredEncoding(buf []byte) string {
	value, ok := DeclaredXMLAttr(buf, "encoding")
	if !ok {
		return ""
	}
	return value
}

// DeclaredXMLVersion returns the XML declaration version value, if present.
func DeclaredXMLVersion(buf []byte) string {
	value, ok := DeclaredXMLAttr(buf, "version")
	if !ok {
		return ""
	}
	return value
}

// DeclaredXMLAttr returns a named XML declaration attribute value.
func DeclaredXMLAttr(buf []byte, want string) (string, bool) {
	if !StartsXMLDeclaration(buf) {
		return "", false
	}
	content := buf[len("<?xml"):]
	pos := XMLDeclFirstAttr
	for {
		name, value, rest, ok := ScanXMLDeclAttr(content, pos)
		if !ok {
			return "", false
		}
		if name == want {
			return value, true
		}
		content = rest
		pos = XMLDeclNextAttr
	}
}
