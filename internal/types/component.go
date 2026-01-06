package types

// NamedComponent exposes the component name without namespace details.
type NamedComponent interface {
	// ComponentName returns the QName of this component.
	ComponentName() QName
}

// NamespacedComponent exposes the declared namespace for a component.
type NamespacedComponent interface {
	// DeclaredNamespace returns the targetNamespace where this component was declared.
	DeclaredNamespace() NamespaceURI
}

// SchemaComponent is implemented by any named component in a schema.
type SchemaComponent interface {
	NamedComponent
	NamespacedComponent
}
