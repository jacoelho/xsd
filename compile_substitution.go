package xsd

import "slices"

func (c *compiler) compileSubstitutions() error {
	direct := make(map[elementID][]elementID)
	for _, memberQName := range sortedQNames(c.elementRaw, c.rt.Names) {
		raw := c.elementRaw[memberQName]
		headLex, ok := raw.node.attr(xsdAttrSubstitutionGroup)
		if !ok {
			continue
		}
		memberID, err := c.compileElementByQName(memberQName)
		if err != nil {
			return err
		}
		headQName, err := c.resolveQNameChecked(raw.node, raw.ctx, headLex)
		if err != nil {
			return err
		}
		if _, ok := c.elementRaw[headQName]; !ok {
			continue
		}
		headID, err := c.compileElementByQName(headQName)
		if err != nil {
			return err
		}
		head := c.rt.Elements[headID]
		member := c.rt.Elements[memberID]
		mask, derived := c.typeDerivationMask(member.Type, head.Type)
		if !derived {
			return schemaCompile(ErrSchemaReference, "substitution group member "+c.rt.Names.Format(member.Name)+" type "+c.rt.typeLabel(member.Type)+" is not derived from head "+c.rt.Names.Format(head.Name)+" type "+c.rt.typeLabel(head.Type))
		}
		if head.Final&mask != 0 {
			return schemaCompile(ErrSchemaReference, "substitution group member type uses excluded derivation")
		}
		c.rt.Elements[memberID].SubstHead = headID
		direct[headID] = append(direct[headID], memberID)
	}
	if err := c.checkSubstitutionCycles(direct); err != nil {
		return err
	}
	for head := range direct {
		seen := make(map[elementID]bool)
		var out []elementID
		var walk func(elementID)
		walk = func(id elementID) {
			for _, member := range direct[id] {
				if seen[member] {
					continue
				}
				seen[member] = true
				out = append(out, member)
				walk(member)
			}
		}
		walk(head)
		slices.Sort(out)
		c.rt.Substitutions[head] = out
	}
	c.compileSubstitutionLookup()
	return nil
}

func (c *compiler) compileSubstitutionLookup() {
	c.rt.SubstitutionLookup = make(map[elementID]map[qName]elementID, len(c.rt.Substitutions))
	for head, members := range c.rt.Substitutions {
		for _, member := range members {
			if !c.rt.substitutionAllowed(head, member) {
				continue
			}
			byName := c.rt.SubstitutionLookup[head]
			if byName == nil {
				byName = make(map[qName]elementID, len(members))
				c.rt.SubstitutionLookup[head] = byName
			}
			byName[c.rt.Elements[member].Name] = member
		}
	}
}

func (c *compiler) checkSubstitutionCycles(direct map[elementID][]elementID) error {
	visiting := make(map[elementID]bool)
	visited := make(map[elementID]bool)
	var walk func(elementID) error
	walk = func(id elementID) error {
		if visiting[id] {
			return schemaCompile(ErrSchemaReference, "cyclic substitution group "+c.rt.Names.Format(c.rt.Elements[id].Name))
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, member := range direct[id] {
			if err := walk(member); err != nil {
				return err
			}
		}
		delete(visiting, id)
		visited[id] = true
		return nil
	}
	for head := range direct {
		if err := walk(head); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) typeDerivesFrom(t, base typeID) bool {
	_, ok := c.typeDerivationMask(t, base)
	return ok
}

func (c *compiler) typeDerivesByRestriction(t, base typeID) bool {
	mask, ok := c.typeDerivationMask(t, base)
	return ok && mask&blockExtension == 0
}

func (c *compiler) typeDerivationMask(t, base typeID) (derivationMask, bool) {
	return c.rt.computeTypeDerivationMask(t, base)
}

func (c *compiler) resolveTypeQName(q qName) (typeID, error) {
	if id, ok := c.simpleDone[q]; ok {
		return simpleRef(id), nil
	}
	if id, ok := c.complexDone[q]; ok {
		return complexRef(id), nil
	}
	if _, ok := c.simpleRaw[q]; ok {
		id, err := c.compileSimpleByQName(q)
		if err != nil {
			return typeID{}, err
		}
		return simpleRef(id), nil
	}
	if _, ok := c.complexRaw[q]; ok {
		id, err := c.compileComplexByQName(q)
		if err != nil {
			return typeID{}, err
		}
		return complexRef(id), nil
	}
	return typeID{}, schemaCompile(ErrSchemaReference, "unknown type "+c.rt.Names.Format(q))
}

func (c *compiler) typeQNameKnown(q qName) bool {
	return c.simpleTypeQNameKnown(q) || c.complexTypeQNameKnown(q)
}

func (c *compiler) simpleTypeQNameKnown(q qName) bool {
	if _, ok := c.simpleDone[q]; ok {
		return true
	}
	_, ok := c.simpleRaw[q]
	return ok
}

func (c *compiler) complexTypeQNameKnown(q qName) bool {
	if _, ok := c.complexDone[q]; ok {
		return true
	}
	_, ok := c.complexRaw[q]
	return ok
}
