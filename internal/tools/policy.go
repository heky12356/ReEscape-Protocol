package tools

import "fmt"

type Policy struct {
	AllowWriteTools bool
}

func (p Policy) IsAvailable(tool Tool) bool {
	if tool == nil {
		return false
	}
	if !tool.ReadOnly() && !p.AllowWriteTools {
		return false
	}
	return true
}

func (p Policy) Check(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool is nil")
	}
	if !p.IsAvailable(tool) {
		return fmt.Errorf("write tool disabled by policy: %s", tool.Name())
	}
	return nil
}
