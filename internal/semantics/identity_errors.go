package semantics

import "errors"

// ErrFieldSelectsComplexContent indicates a field XPath selects an element with
// complex content, which is invalid per XSD 1.0 identity-constraint rules.
var ErrFieldSelectsComplexContent = errors.New("field selects element with complex content")

// ErrXPathUnresolvable indicates a selector or field XPath cannot be resolved
// statically, such as when wildcard steps are present.
var ErrXPathUnresolvable = errors.New("xpath cannot be resolved statically")

// ErrFieldXPathIncompatibleTypes indicates a field XPath resolves to
// incompatible element types.
var ErrFieldXPathIncompatibleTypes = errors.New("field xpath resolves to incompatible element types")

// ErrFieldXPathUnresolved indicates a field XPath cannot be resolved.
var ErrFieldXPathUnresolved = errors.New("field xpath unresolved")

// ErrFieldSelectsNillable indicates a field XPath selects a nillable element.
var ErrFieldSelectsNillable = errors.New("field selects nillable element")
