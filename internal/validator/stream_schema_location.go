package validator

import (
	"io"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/xml"
)

type schemaLocationPrepass struct {
	rootLocal string
	hints     []schemaLocationHint
}

func collectSchemaLocationHintsFromStream(dec *xml.StreamDecoder) (schemaLocationPrepass, error) {
	var result schemaLocationPrepass
	for {
		ev, err := dec.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, err
		}
		if ev.Kind != xml.EventStartElement {
			continue
		}
		if result.rootLocal == "" {
			result.rootLocal = ev.Name.Local
		}
		hints := schemaLocationHintsFromAttrs(ev.Attrs)
		if len(hints) > 0 {
			result.hints = append(result.hints, hints...)
		}
	}
	return result, nil
}

func schemaLocationPolicyError(path string) error {
	return errors.ValidationList{errors.NewValidation(errors.ErrSchemaLocationHint,
		"schemaLocation hints present but reader is not seekable; set SchemaLocationIgnore to proceed", path)}
}
