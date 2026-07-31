package character

// CharacterConfig describes the product identity and visible voice used by a
// character. Legacy fields are kept so existing config/character/*.json files
// continue to load and save without losing data.
type CharacterConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	Identity     CharacterIdentity     `json:"identity"`
	Voice        CharacterVoice        `json:"voice"`
	Boundaries   CharacterBoundaries   `json:"boundaries"`
	Relationship CharacterRelationship `json:"relationship"`
	Examples     []CharacterExample    `json:"examples"`

	Personality map[string]string      `json:"personality"`
	Responses   map[string]interface{} `json:"responses"`
	Behavior    map[string]interface{} `json:"behavior"`
	Quotes      []string               `json:"quotes"`
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
