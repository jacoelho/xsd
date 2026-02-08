package runtime

func digestModels(h *digestBuilder, models ModelsBundle) {
	h.u32(uint32(len(models.DFA)))
	for _, m := range models.DFA {
		h.u32(m.Start)
		h.u32(uint32(len(m.States)))
		for _, s := range m.States {
			h.bool(s.Accept)
			h.u32(s.TransOff)
			h.u32(s.TransLen)
			h.u32(s.WildOff)
			h.u32(s.WildLen)
		}
		h.u32(uint32(len(m.Transitions)))
		for _, tr := range m.Transitions {
			h.u32(uint32(tr.Sym))
			h.u32(tr.Next)
			h.u32(uint32(tr.Elem))
		}
		h.u32(uint32(len(m.Wildcards)))
		for _, w := range m.Wildcards {
			h.u32(uint32(w.Rule))
			h.u32(w.Next)
		}
	}
	h.u32(uint32(len(models.NFA)))
	for _, m := range models.NFA {
		digestU64Slice(h, m.Bitsets.Words)
		digestBitsetRef(h, m.Start)
		digestBitsetRef(h, m.Accept)
		h.bool(m.Nullable)
		h.u32(m.FollowOff)
		h.u32(m.FollowLen)
		h.u32(uint32(len(m.Matchers)))
		for _, match := range m.Matchers {
			h.u8(uint8(match.Kind))
			h.u32(uint32(match.Sym))
			h.u32(uint32(match.Elem))
			h.u32(uint32(match.Rule))
		}
		h.u32(uint32(len(m.Follow)))
		for _, ref := range m.Follow {
			digestBitsetRef(h, ref)
		}
	}
	h.u32(uint32(len(models.All)))
	for _, m := range models.All {
		h.u32(m.MinOccurs)
		h.bool(m.Mixed)
		h.u32(uint32(len(m.Members)))
		for _, member := range m.Members {
			h.u32(uint32(member.Elem))
			h.bool(member.Optional)
			h.bool(member.AllowsSubst)
			h.u32(member.SubstOff)
			h.u32(member.SubstLen)
		}
	}
	digestElemIDs(h, models.AllSubst)
}

func digestWildcards(h *digestBuilder, wildcards []WildcardRule, nsList []NamespaceID) {
	h.u32(uint32(len(wildcards)))
	for _, w := range wildcards {
		h.u8(uint8(w.NS.Kind))
		h.bool(w.NS.HasTarget)
		h.bool(w.NS.HasLocal)
		h.u32(w.NS.Off)
		h.u32(w.NS.Len)
		h.u8(uint8(w.PC))
		h.u32(uint32(w.TargetNS))
	}
	digestNamespaceIDs(h, nsList)
}

func digestIdentity(h *digestBuilder, ics []IdentityConstraint, elemICs []ICID, selectors, fields []PathID, paths []PathProgram) {
	h.u32(uint32(len(ics)))
	for _, ic := range ics {
		h.u32(uint32(ic.Name))
		h.u8(uint8(ic.Category))
		h.u32(ic.SelectorOff)
		h.u32(ic.SelectorLen)
		h.u32(ic.FieldOff)
		h.u32(ic.FieldLen)
		h.u32(uint32(ic.Referenced))
	}
	digestICIDs(h, elemICs)
	digestPathIDs(h, selectors)
	digestPathIDs(h, fields)
	h.u32(uint32(len(paths)))
	for _, p := range paths {
		h.u32(uint32(len(p.Ops)))
		for _, op := range p.Ops {
			h.u8(uint8(op.Op))
			h.u32(uint32(op.Sym))
			h.u32(uint32(op.NS))
		}
	}
}
