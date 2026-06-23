package compile

import "github.com/jacoelho/xsd/internal/runtime"

func freezeCompilerRuntime(c *compiler) (*runtime.Schema, error) {
	published := c.rt
	if err := runtime.ValidateCompilerPublication(&published); err != nil {
		return nil, err
	}
	c.rt = runtime.Schema{}
	c.names = NameInterner{}
	return &published, nil
}
