package validator

import (
	"strings"

	"github.com/jacoelho/xsd/internal/types"
)

type pathStack struct {
	parts []types.QName
}

func (p *pathStack) reset() {
	p.parts = p.parts[:0]
}

func (p *pathStack) push(part types.QName) {
	p.parts = append(p.parts, part)
}

func (p *pathStack) pop() {
	if len(p.parts) == 0 {
		return
	}
	p.parts = p.parts[:len(p.parts)-1]
}

func (p *pathStack) String() string {
	if len(p.parts) == 0 {
		return "/"
	}
	total := 0
	for _, part := range p.parts {
		if part.Namespace.IsEmpty() {
			total += 1 + len(part.Local)
		} else {
			total += 1 + len(part.Namespace) + len(part.Local) + 2
		}
	}
	var b strings.Builder
	b.Grow(total)
	for _, part := range p.parts {
		b.WriteByte('/')
		if part.Namespace.IsEmpty() {
			b.WriteString(part.Local)
			continue
		}
		b.WriteByte('{')
		b.WriteString(part.Namespace.String())
		b.WriteByte('}')
		b.WriteString(part.Local)
	}
	return b.String()
}
