package skill

import "project-yume/internal/state"

// Scope identifies where a skill was mounted from.
type Scope string

const (
	ScopeGlobal    Scope = "global"
	ScopeUser      Scope = "user"
	ScopeCharacter Scope = "character"
	ScopeProject   Scope = "project"
	ScopeSystem    Scope = "system"
)

type Package struct {
	Name        string
	Description string
	RootDir     string
	Body        string
	Frontmatter map[string]any
	Scope       Scope
	Enabled     bool
}

type Match struct {
	Name        string
	Description string
	Score       int
	Reason      string
	Scope       Scope
}

type MatchInput struct {
	Message       string
	Trigger       string
	DialogueState state.DialogueState
	ToolNames     []string
	Limit         int
}
