package loadmerge

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/model"
)

func elementDeclEquivalent(a, b *model.ElementDecl) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if a.Nillable != b.Nillable || a.Abstract != b.Abstract || a.SubstitutionGroup != b.SubstitutionGroup {
		return false
	}
	if a.Block != b.Block || a.Final != b.Final {
		return false
	}
	if a.HasFixed != b.HasFixed || a.HasDefault != b.HasDefault {
		return false
	}
	if a.HasFixed && a.Fixed != b.Fixed {
		return false
	}
	if a.HasDefault && a.Default != b.Default {
		return false
	}
	if a.Form != b.Form {
		return false
	}
	if !model.ElementTypesCompatible(a.Type, b.Type) {
		return false
	}
	return identityConstraintsEquivalent(a.Constraints, b.Constraints)
}

func identityConstraintsEquivalent(a, b []*model.IdentityConstraint) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	keysA := make([]string, 0, len(a))
	for _, constraint := range a {
		keysA = append(keysA, identityConstraintKey(constraint))
	}
	keysB := make([]string, 0, len(b))
	for _, constraint := range b {
		keysB = append(keysB, identityConstraintKey(constraint))
	}
	slices.Sort(keysA)
	slices.Sort(keysB)
	return slices.Equal(keysA, keysB)
}

func identityConstraintKey(constraint *model.IdentityConstraint) string {
	if constraint == nil {
		return "<nil>"
	}
	var builder strings.Builder
	builder.WriteString(constraint.Name)
	builder.WriteByte('|')
	builder.WriteString(strconv.Itoa(int(constraint.Type)))
	builder.WriteByte('|')
	builder.WriteString(constraint.Selector.XPath)
	builder.WriteByte('|')
	builder.WriteString(constraint.ReferQName.String())
	builder.WriteByte('|')
	builder.WriteString(constraint.TargetNamespace)
	builder.WriteByte('|')
	for _, field := range constraint.Fields {
		builder.WriteString(field.XPath)
		builder.WriteByte('\x1f')
	}
	builder.WriteByte('|')
	if len(constraint.NamespaceContext) == 0 {
		return builder.String()
	}
	keys := slices.Collect(maps.Keys(constraint.NamespaceContext))
	slices.Sort(keys)
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(constraint.NamespaceContext[key])
		builder.WriteByte(';')
	}
	return builder.String()
}
