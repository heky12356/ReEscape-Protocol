package agent

import "testing"

func TestBehaviorProjectionPromptHidesPrivateState(t *testing.T) {
	prompt := (BehaviorProjection{Warmth: "warm", Initiative: "moderate", Directness: "gentle", SelfDisclosure: "limited", ConflictRepair: "open"}).Prompt()
	if prompt == "" || containsProjectionSecret(prompt) {
		t.Fatalf("unexpected behavior projection prompt: %q", prompt)
	}
}

func containsProjectionSecret(prompt string) bool {
	for _, item := range []string{"affection", "trust", "emotion_id", "score"} {
		if len(prompt) >= len(item) && containsText(prompt, item) {
			return true
		}
	}
	return false
}

func containsText(text, needle string) bool {
	for i := 0; i+len(needle) <= len(text); i++ {
		if text[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
