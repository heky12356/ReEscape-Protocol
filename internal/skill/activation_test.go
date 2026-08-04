package skill

import "testing"

func TestResolverAutoLoadsExactTrigger(t *testing.T) {
	resolver := Resolver{}
	activations := resolver.Resolve([]Match{{
		Name:       "paper-humanizer",
		Score:      20,
		Confidence: 0.95,
		Reason:     "matched exact trigger",
	}})
	if len(activations) != 1 || activations[0].Decision != ActivationAutoLoad {
		t.Fatalf("expected exact trigger to auto-load, got %#v", activations)
	}
}

func TestResolverKeepsAmbiguousTopMatchesForModelSelection(t *testing.T) {
	resolver := Resolver{}
	activations := resolver.Resolve([]Match{
		{Name: "first", Score: 20, Confidence: 0.90},
		{Name: "second", Score: 18, Confidence: 0.88},
	})
	for _, activation := range activations {
		if activation.Decision != ActivationModelSelect {
			t.Fatalf("expected close matches to require model selection, got %#v", activations)
		}
	}
}

func TestResolverIgnoresLowScore(t *testing.T) {
	resolver := Resolver{}
	activations := resolver.Resolve([]Match{{Name: "weak", Score: 6, Confidence: 0.4}})
	if len(activations) != 1 || activations[0].Decision != ActivationIgnored {
		t.Fatalf("expected low score to be ignored, got %#v", activations)
	}
}

func TestResolverLimitsAutoLoadedSkills(t *testing.T) {
	resolver := Resolver{MaxAutoLoaded: 1, AmbiguityScoreDelta: 1}
	activations := resolver.Resolve([]Match{
		{Name: "first", Score: 24, Confidence: 0.95},
		{Name: "second", Score: 20, Confidence: 0.90},
	})
	autoLoaded := 0
	for _, activation := range activations {
		if activation.Decision == ActivationAutoLoad {
			autoLoaded++
		}
	}
	if autoLoaded != 1 {
		t.Fatalf("expected exactly one auto-loaded skill, got %#v", activations)
	}
}
