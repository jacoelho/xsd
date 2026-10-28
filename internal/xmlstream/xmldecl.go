package xmlstream

import "github.com/jacoelho/xsd/internal/lex"

const utf8BOMLen = 3

// XMLDeclarationPrefixLen is the minimum bytes needed to recognize "<?xml ".
const XMLDeclarationPrefixLen = len("<?xml ")

// HasUTF8BOM reports whether buf begins with the UTF-8 byte order mark.
func HasUTF8BOM(buf []byte) bool {
	return len(buf) >= utf8BOMLen && buf[0] == 0xEF && buf[1] == 0xBB && buf[2] == 0xBF
}

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
