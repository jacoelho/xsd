package schema

import (
	"cmp"
	"errors"
	"slices"

	"github.com/jacoelho/xsd/internal/lex"
	valuepkg "github.com/jacoelho/xsd/internal/value"
)

// valueConstraintQNameBinding records the schema namespace associated with one
// prefix appearing in a constraint's application text. A missing prefixed
// binding is retained as unresolved so a context-free declared value can be
// revalidated under a later QName type without consulting instance namespaces.
type valueConstraintQNameBinding struct {
	prefix    string
	namespace string
	bound     bool
}

// valueConstraintQNameContext is immutable after construction. Its entries
// are one per distinct valid QName prefix in application text and are sorted
// for repeatable lookup.
type valueConstraintQNameContext struct {
	bindings []valueConstraintQNameBinding
}

func newValueConstraintQNameContext(application string, lookup func(string) (string, bool)) *valueConstraintQNameContext {
	context := &valueConstraintQNameContext{}
	seen := make(map[string]struct{})
	for field := range lex.XMLFieldsSeq(application) {
		parts := lex.SplitQName(field)
		if !parts.Valid {
			continue
		}
		if _, ok := seen[parts.Prefix]; ok {
			continue
		}
		seen[parts.Prefix] = struct{}{}
		namespace, bound := lookup(parts.Prefix)
		context.bindings = append(context.bindings, valueConstraintQNameBinding{
			prefix:    parts.Prefix,
			namespace: namespace,
			bound:     bound,
		})
	}
	slices.SortFunc(context.bindings, func(a, b valueConstraintQNameBinding) int {
		return cmp.Compare(a.prefix, b.prefix)
	})
	return context
}

func cloneValueConstraintQNameContext(in *valueConstraintQNameContext) *valueConstraintQNameContext {
	if in == nil {
		return nil
	}
	return &valueConstraintQNameContext{bindings: slices.Clone(in.bindings)}
}

func equalValueConstraintQNameContexts(a, b *valueConstraintQNameContext) bool {
	if a == nil || b == nil {
		return a == b
	}
	return slices.Equal(a.bindings, b.bindings)
}

// ResolveQName resolves one application-text QName without consuming a
// captured entry. The method is safe to call repeatedly for union members and
// list items.
func (c *valueConstraintQNameContext) ResolveQName(lexical string) (valuepkg.ExpandedName, bool) {
	if c == nil {
		return valuepkg.ExpandedName{}, false
	}
	parts := resolvedValueNameLexicalParts(lexical)
	if !parts.Valid || parts.Prefixed && parts.Prefix == "" {
		return valuepkg.ExpandedName{}, false
	}
	index, ok := slices.BinarySearchFunc(c.bindings, parts.Prefix, func(binding valueConstraintQNameBinding, prefix string) int {
		return cmp.Compare(binding.prefix, prefix)
	})
	if !ok || !c.bindings[index].bound {
		return valuepkg.ExpandedName{}, false
	}
	return valuepkg.ExpandedName{
		Namespace: c.bindings[index].namespace,
		Local:     parts.Local,
	}, true
}

// validateValueConstraintQNameContext checks the retained context against the
// exact application spelling and the accepted datatype proof. The proof may
// be empty: prospective contexts deliberately cover string-first unions and
// untyped anyType constraints that did not resolve a QName during admission.
func validateValueConstraintQNameContext(application string, context *valueConstraintQNameContext, names []ResolvedValueName) error {
	if context == nil {
		return errors.New("value constraint QName context is missing")
	}
	expected := valueConstraintQNamePrefixes(application)
	if err := validateValueConstraintQNameBindings(expected, context.bindings); err != nil {
		return err
	}
	return validateValueConstraintQNameProof(context, names)
}

func valueConstraintQNamePrefixes(application string) map[string]struct{} {
	expected := make(map[string]struct{})
	for field := range lex.XMLFieldsSeq(application) {
		parts := lex.SplitQName(field)
		if parts.Valid {
			expected[parts.Prefix] = struct{}{}
		}
	}
	return expected
}

func validateValueConstraintQNameBindings(expected map[string]struct{}, bindings []valueConstraintQNameBinding) error {
	if len(expected) != len(bindings) {
		return errors.New("value constraint QName context does not cover application prefixes")
	}
	for i, binding := range bindings {
		if i > 0 && bindings[i-1].prefix >= binding.prefix {
			return errors.New("value constraint QName context is not sorted and unique")
		}
		if _, ok := expected[binding.prefix]; !ok {
			return errors.New("value constraint QName context contains an unused prefix")
		}
		if !binding.bound && binding.namespace != "" {
			return errors.New("unresolved value constraint QName prefix has a namespace")
		}
	}
	return nil
}

func validateValueConstraintQNameProof(context *valueConstraintQNameContext, names []ResolvedValueName) error {
	for _, entry := range names {
		parts := resolvedValueNameLexicalParts(entry.Lexical)
		if !parts.Valid {
			return errors.New("resolved name proof is not a valid QName")
		}
		resolved, ok := context.ResolveQName(entry.Lexical)
		if !ok || resolved.Namespace != entry.NS || resolved.Local != entry.Local {
			return errors.New("value constraint QName context disagrees with resolved name proof")
		}
	}
	return nil
}
