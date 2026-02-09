package xmlstream

import "github.com/jacoelho/xsd/pkg/xmltext"

// Option configures the xmlstream reader.
// Construct options via helpers in pkg/xmltext.
type Option = xmltext.Options

func buildOptions(opts ...Option) []xmltext.Options {
	base := []xmltext.Options{
		xmltext.ResolveEntities(false),
		xmltext.CoalesceCharData(true),
		xmltext.EmitComments(false),
		xmltext.EmitPI(false),
		xmltext.EmitDirectives(false),
		xmltext.TrackLineColumn(true),
		xmltext.MaxQNameInternEntries(qnameCacheMaxEntries),
	}
	out := make([]xmltext.Options, 0, len(base)+len(opts))
	out = append(out, base...)
	out = append(out, opts...)
	return out
}

// JoinOptions merges xmlstream options into a single xmltext options struct.
func JoinOptions(opts ...Option) xmltext.Options {
	joined := buildOptions(opts...)
	return xmltext.JoinOptions(joined...)
}
