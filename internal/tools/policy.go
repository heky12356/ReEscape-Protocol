package tools

import "fmt"

type Policy struct {
	AllowWriteTools bool
}

func (p Policy) Check(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool is nil")
	}
	if !tool.ReadOnly() && !p.AllowWriteTools {
		return fmt.Errorf("write tool disabled by policy: %s", tool.Name())
	}
	return nil
}
