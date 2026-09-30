package openloop

import "testing"

func TestActionableOpenLoop(t *testing.T) {
	loop := Normalize(OpenLoop{Kind: KindUnansweredQuestion, Description: "  需要回答  "})
	if !IsActionable(loop) || loop.Status != "open" || loop.Description != "需要回答" {
		t.Fatalf("unexpected normalized open loop: %+v", loop)
	}
}
