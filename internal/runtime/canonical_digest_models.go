package runtime

func digestModels(h FingerprintWriter, models ModelsBundle) {
	h.WriteU32(uint32(len(models.DFA)))
	for _, m := range models.DFA {
		h.WriteU32(m.Start)
		h.WriteU32(uint32(len(m.States)))
		for _, s := range m.States {
			h.WriteBool(s.Accept)
			h.WriteU32(s.TransOff)
			h.WriteU32(s.TransLen)
			h.WriteU32(s.WildOff)
			h.WriteU32(s.WildLen)
		}
		h.WriteU32(uint32(len(m.Transitions)))
		for _, tr := range m.Transitions {
			h.WriteU32(uint32(tr.Sym))
			h.WriteU32(tr.Next)
			h.WriteU32(uint32(tr.Elem))
		}
		h.WriteU32(uint32(len(m.Wildcards)))
		for _, w := range m.Wildcards {
			h.WriteU32(uint32(w.Rule))
			h.WriteU32(w.Next)
		}
	}
	h.WriteU32(uint32(len(models.NFA)))
	for _, m := range models.NFA {
		digestU64Slice(h, m.Bitsets.Words)
		digestBitsetRef(h, m.Start)
		digestBitsetRef(h, m.Accept)
		h.WriteBool(m.Nullable)
		h.WriteU32(m.FollowOff)
		h.WriteU32(m.FollowLen)
		h.WriteU32(uint32(len(m.Matchers)))
		for _, match := range m.Matchers {
			h.WriteU8(uint8(match.Kind))
			h.WriteU32(uint32(match.Sym))
			h.WriteU32(uint32(match.Elem))
			h.WriteU32(uint32(match.Rule))
		}
		h.WriteU32(uint32(len(m.Follow)))
		for _, ref := range m.Follow {
			digestBitsetRef(h, ref)
		}
	}
	h.WriteU32(uint32(len(models.All)))
	for _, m := range models.All {
		h.WriteU32(m.MinOccurs)
		h.WriteBool(m.Mixed)
		h.WriteU32(uint32(len(m.Members)))
		for _, member := range m.Members {
			h.WriteU32(uint32(member.Elem))
			h.WriteBool(member.Optional)
			h.WriteBool(member.AllowsSubst)
			h.WriteU32(member.SubstOff)
			h.WriteU32(member.SubstLen)
		}
	}
	digestElemIDs(h, models.AllSubst)
}

func digestWildcards(h FingerprintWriter, wildcards []WildcardRule, nsList []NamespaceID) {
	h.WriteU32(uint32(len(wildcards)))
	for _, w := range wildcards {
		h.WriteU8(uint8(w.NS.Kind))
		h.WriteBool(w.NS.HasTarget)
		h.WriteBool(w.NS.HasLocal)
		h.WriteU32(w.NS.Off)
		h.WriteU32(w.NS.Len)
		h.WriteU8(uint8(w.PC))
		h.WriteU32(uint32(w.TargetNS))
	}
	digestNamespaceIDs(h, nsList)
}

func digestIdentity(h FingerprintWriter, ics []IdentityConstraint, elemICs []ICID, selectors, fields []PathID, paths []PathProgram) {
	h.WriteU32(uint32(len(ics)))
	for _, ic := range ics {
		h.WriteU32(uint32(ic.Name))
		h.WriteU8(uint8(ic.Category))
		h.WriteU32(ic.SelectorOff)
		h.WriteU32(ic.SelectorLen)
		h.WriteU32(ic.FieldOff)
		h.WriteU32(ic.FieldLen)
		h.WriteU32(uint32(ic.Referenced))
	}
	digestICIDs(h, elemICs)
	digestPathIDs(h, selectors)
	digestPathIDs(h, fields)
	h.WriteU32(uint32(len(paths)))
	for _, p := range paths {
		h.WriteU32(uint32(len(p.Ops)))
		for _, op := range p.Ops {
			h.WriteU8(uint8(op.Op))
			h.WriteU32(uint32(op.Sym))
			h.WriteU32(uint32(op.NS))
		}
	}
}
