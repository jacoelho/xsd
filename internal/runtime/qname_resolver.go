package runtime

// ResolveQNameParts resolves a lexical QName into namespace URI and local name.
type ResolveQNameParts func(string) (string, string, bool)
