package agent

import (
	"strings"

	"project-yume/internal/domain/affection"
	"project-yume/internal/memory"
	"project-yume/internal/state"
)

// BehaviorProjection is the model-facing projection of private runtime state.
// It deliberately contains qualitative tendencies, never scores or raw IDs.
type BehaviorProjection struct {
	Warmth         string `json:"warmth"`
	Initiative     string `json:"initiative"`
	Directness     string `json:"directness"`
	SelfDisclosure string `json:"self_disclosure"`
	ConflictRepair string `json:"conflict_recovery"`
}

func ProjectBehavior(userID int64, sessionID string) BehaviorProjection {
	profile := memory.GetProfileManager().GetProfile(userID)
	affectionState := affection.GetManager().Get(userID)
	dialogue := state.GetManager().GetDialogueState(sessionID)
	projection := BehaviorProjection{
		Warmth:         "warm",
		Initiative:     "moderate",
		Directness:     "gentle",
		SelfDisclosure: "limited",
		ConflictRepair: "open",
	}

	switch strings.ToLower(strings.TrimSpace(affectionState.Stage)) {
	case "close", "intimate", "familiar":
		projection.Warmth = "warm and familiar"
		projection.SelfDisclosure = "moderate"
	case "distant", "cautious":
		projection.Warmth = "polite and reserved"
	}
	if strings.TrimSpace(profile.RelationshipStyle) != "" {
		projection.Initiative = "follow the user's established relationship style"
	}
	if strings.TrimSpace(dialogue.UserNeed) != "" || strings.TrimSpace(dialogue.Intention) != "" {
		projection.Directness = "respond to the user's immediate need"
	}
	return projection
}

func (p BehaviorProjection) Prompt() string {
	return "【Behavior Projection】\n" +
		"warmth: " + p.Warmth + "\n" +
		"initiative: " + p.Initiative + "\n" +
		"directness: " + p.Directness + "\n" +
		"self_disclosure: " + p.SelfDisclosure + "\n" +
		"conflict_recovery: " + p.ConflictRepair + "\n" +
		"只将其作为当前轮行为倾向参考，不要暴露内部状态、分数或字段名。"
}
