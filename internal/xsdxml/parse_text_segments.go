package xsdxml

func (d *Document) addTextSegment(parent NodeID, childIndex, textOff, textLen int) {
	if textLen == 0 {
		return
	}
	if len(d.textScratch) > 0 {
		last := &d.textScratch[len(d.textScratch)-1]
		if last.parent == parent && last.childIndex == childIndex && last.textOff+last.textLen == textOff {
			last.textLen += textLen
			return
		}
	}
	d.textScratch = append(d.textScratch, textScratchEntry{
		parent:     parent,
		childIndex: childIndex,
		textOff:    textOff,
		textLen:    textLen,
	})
}

func (d *Document) buildTextSegments() {
	if len(d.nodes) == 0 || len(d.textScratch) == 0 {
		d.textSegments = d.textSegments[:0]
		return
	}

	counts := d.acquireCountsScratch()

	for _, entry := range d.textScratch {
		if entry.parent == InvalidNode {
			continue
		}
		counts[entry.parent]++
	}

	total := assignOffsets(counts, func(i, off, count int) {
		d.nodes[i].textSegOff = off
		d.nodes[i].textSegLen = count
	})

	if total == 0 {
		d.textSegments = d.textSegments[:0]
		return
	}
	if cap(d.textSegments) < total {
		d.textSegments = make([]textSegment, total)
	} else {
		d.textSegments = d.textSegments[:total]
	}

	for _, entry := range d.textScratch {
		if entry.parent == InvalidNode {
			continue
		}
		idx := counts[entry.parent]
		d.textSegments[idx] = textSegment{
			childIndex: entry.childIndex,
			textOff:    entry.textOff,
			textLen:    entry.textLen,
		}
		counts[entry.parent]++
	}
}

func (d *Document) acquireCountsScratch() []int {
	counts := d.countsScratch
	if cap(counts) < len(d.nodes) {
		counts = make([]int, len(d.nodes))
	} else {
		counts = counts[:len(d.nodes)]
		clear(counts)
	}
	d.countsScratch = counts
	return counts
}

func assignOffsets(counts []int, setOffset func(i, off, count int)) int {
	total := 0
	for i := range counts {
		count := counts[i]
		setOffset(i, total, count)
		counts[i] = total
		total += count
	}
	return total
}
