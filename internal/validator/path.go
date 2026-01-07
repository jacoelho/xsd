package validator

import "strings"

type pathStack struct {
	parts []string
}

func (p *pathStack) reset() {
	p.parts = p.parts[:0]
}

func (p *pathStack) push(part string) {
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
		total += 1 + len(part)
	}
	var b strings.Builder
	b.Grow(total)
	for _, part := range p.parts {
		b.WriteByte('/')
		b.WriteString(part)
	}
	return b.String()
}
