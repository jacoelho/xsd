package compile

import "github.com/jacoelho/xsd/internal/runtime"

func publishCompilerRuntime(c *compiler) (*runtime.Schema, error) {
	published, err := runtime.PublishSchema(c.rt)
	if err != nil {
		return nil, err
	}
	c.rt = runtime.SchemaBuild{}
	c.names = NameInterner{}
	return published, nil
}
