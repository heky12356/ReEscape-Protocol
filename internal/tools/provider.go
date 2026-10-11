package tools

import "context"

// ToolProvider supplies tools from one integration boundary. Providers are
// intentionally small so MCP and plugin implementations can be added later
// without changing the registry contract.
type ToolProvider interface {
	Name() string
	Origin() string
	Version() string
	List(ctx context.Context) ([]Tool, error)
	Close() error
}

type ToolMetadata struct {
	Provider       string `json:"provider"`
	Origin         string `json:"origin"`
	Version        string `json:"version,omitempty"`
	ConflictReason string `json:"conflictReason,omitempty"`
}

type ToolEntry struct {
	Tool     Tool         `json:"-"`
	Metadata ToolMetadata `json:"metadata"`
}

type ToolConflict struct {
	Name     string       `json:"name"`
	Existing ToolMetadata `json:"existing"`
	Rejected ToolMetadata `json:"rejected"`
	Reason   string       `json:"reason"`
}
