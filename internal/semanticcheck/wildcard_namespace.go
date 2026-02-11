package semanticcheck

import "github.com/jacoelho/xsd/internal/model"

func processContentsName(pc model.ProcessContents) string {
	switch pc {
	case model.Strict:
		return "strict"
	case model.Lax:
		return "lax"
	case model.Skip:
		return "skip"
	default:
		return "unknown"
	}
}
