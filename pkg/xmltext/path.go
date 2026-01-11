package xmltext

import "strconv"

// StackEntry captures the name and ordinal for a stacked element.
type StackEntry struct {
	Name  QNameSpan
	Index int64
}

// Path is a snapshot of the current element stack.
type Path []StackEntry

// String renders the path using local names only.
func (p Path) String() string {
	return string(p.AppendTo(nil))
}

// XPath renders the path using lexical prefixes when present.
func (p Path) XPath() string {
	return string(p.appendWithPrefix(nil))
}

// AppendTo appends the path using local names to dst.
func (p Path) AppendTo(dst []byte) []byte {
	for _, entry := range p {
		dst = append(dst, '/')
		name := entry.Name.Local.bytes()
		dst = append(dst, name...)
		dst = append(dst, '[')
		dst = strconv.AppendInt(dst, entry.Index, 10)
		dst = append(dst, ']')
	}
	return dst
}

func (p Path) appendWithPrefix(dst []byte) []byte {
	for _, entry := range p {
		dst = append(dst, '/')
		name := entry.Name.Local.bytes()
		if entry.Name.HasPrefix {
			name = entry.Name.Full.bytes()
		}
		dst = append(dst, name...)
		dst = append(dst, '[')
		dst = strconv.AppendInt(dst, entry.Index, 10)
		dst = append(dst, ']')
	}
	return dst
}
