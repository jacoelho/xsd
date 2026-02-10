package schemaxml

import "slices"

func (d *Document) addNode(namespace, local string, attrs []Attr, parent NodeID) NodeID {
	id := NodeID(len(d.nodes))

	attrsOff := len(d.attrs)
	if len(attrs) > 0 {
		d.attrs = slices.Grow(d.attrs, len(attrs))
		d.attrs = d.attrs[:attrsOff+len(attrs)]
		for i, attr := range attrs {
			d.attrs[attrsOff+i] = attr
		}
	}

	d.nodes = append(d.nodes, node{
		namespace: namespace,
		local:     local,
		attrsOff:  attrsOff,
		attrsLen:  len(attrs),
		parent:    parent,
	})

	return id
}

func (d *Document) buildChildren() {
	if len(d.nodes) == 0 {
		return
	}

	counts := d.acquireCountsScratch()
	for i := range d.nodes {
		parent := d.nodes[i].parent
		if parent != InvalidNode {
			counts[parent]++
		}
	}

	total := assignOffsets(counts, func(i, off, count int) {
		d.nodes[i].childrenOff = off
		d.nodes[i].childrenLen = count
	})

	if total == 0 {
		d.children = d.children[:0]
		return
	}

	if cap(d.children) < total {
		d.children = make([]NodeID, total)
	} else {
		d.children = d.children[:total]
	}

	for i := range d.nodes {
		parent := d.nodes[i].parent
		if parent == InvalidNode {
			continue
		}
		idx := counts[parent]
		d.children[idx] = NodeID(i)
		counts[parent]++
	}
}
