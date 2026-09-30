package character

// CharacterConfig describes the stable character canon used by the runtime.
type CharacterConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	Identity     CharacterIdentity     `json:"identity"`
	Background   CharacterBackground   `json:"background"`
	Voice        CharacterVoice        `json:"voice"`
	Boundaries   CharacterBoundaries   `json:"boundaries"`
	Relationship CharacterRelationship `json:"relationship"`
	Examples     []CharacterExample    `json:"examples"`
}

type CharacterBackground struct {
	Age        string   `json:"age"`
	Occupation string   `json:"occupation"`
	Traits     []string `json:"traits"`
	Interests  []string `json:"interests"`
	Habits     []string `json:"habits"`
	Skills     []string `json:"skills"`
}

type CharacterIdentity struct {
	RoleName        string `json:"roleName"`
	ProductIdentity string `json:"productIdentity"`
	SelfReference   string `json:"selfReference"`
	IdentityPolicy  string `json:"identityPolicy"`
}

type CharacterVoice struct {
	Tone       string   `json:"tone"`
	Style      string   `json:"style"`
	Pacing     string   `json:"pacing"`
	Vocabulary []string `json:"vocabulary"`
	Avoid      []string `json:"avoid"`
}

type CharacterBoundaries struct {
	DoNotReveal       []string `json:"doNotReveal"`
	SafetyBoundaries  []string `json:"safetyBoundaries"`
	RelationshipRules []string `json:"relationshipRules"`
}

type CharacterRelationship struct {
	DefaultStage string `json:"defaultStage"`
	Addressing   string `json:"addressing"`
	IntimacyRule string `json:"intimacyRule"`
}

type CharacterExample struct {
	Situation string `json:"situation"`
	User      string `json:"user"`
	Reply     string `json:"reply"`
}
