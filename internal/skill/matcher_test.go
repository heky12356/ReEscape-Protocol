package skill

import "testing"

func TestMatchPackagesPaperHumanizerPositiveAndNegativeCases(t *testing.T) {
	pkg := Package{
		Name:        "paper-humanizer",
		Description: "Polish academic papers while preserving citations and factual accuracy.",
		Frontmatter: map[string]any{
			"tags": []any{"论文润色", "学术写作"},
			"triggers": []any{
				"润色这段论文",
				"降低 AI 痕迹",
				"学术段落改得自然",
			},
		},
		Enabled: true,
	}

	positive := []string{
		"帮我润色这段论文，降低 AI 痕迹。",
		"把下面的学术段落改得自然一点，但保留引用。",
	}
	for _, message := range positive {
		matches := MatchPackages([]Package{pkg}, MatchInput{Message: message})
		if len(matches) != 1 || matches[0].Name != pkg.Name {
			t.Fatalf("expected %q to match paper-humanizer, got %#v", message, matches)
		}
		if matches[0].Score < 16 || matches[0].Confidence < 0.75 {
			t.Fatalf("expected high-confidence paper match for %q, got %#v", message, matches[0])
		}
	}

	negative := []string{
		"晚安。",
		"今天好累。",
		"你会一直记得我吗？",
		"帮我解释一下这个 Go 报错。",
	}
	for _, message := range negative {
		if matches := MatchPackages([]Package{pkg}, MatchInput{Message: message}); len(matches) != 0 {
			t.Fatalf("did not expect %q to match paper-humanizer, got %#v", message, matches)
		}
	}
}

func TestMatchPackagesReplyHumanizerOnlyMatchesExplicitStyleRequests(t *testing.T) {
	pkg := Package{
		Name:        "reply-humanizer",
		Description: "改写自然、简洁、有上下文感的中文聊天回复。",
		Frontmatter: map[string]any{
			"tags": []any{"自然表达", "回复风格"},
			"triggers": []any{
				"自然一点",
				"不要像机器人",
				"口语一点",
			},
		},
		Enabled: true,
	}

	positive := []string{
		"把这句话改得自然一点。",
		"回复不要像机器人。",
		"帮我改得口语一点。",
	}
	for _, message := range positive {
		matches := MatchPackages([]Package{pkg}, MatchInput{Message: message})
		if len(matches) != 1 || matches[0].Name != pkg.Name {
			t.Fatalf("expected %q to match reply-humanizer, got %#v", message, matches)
		}
	}

	negative := []string{
		"今天吃了烧烤。",
		"早点休息。",
		"你在干嘛？",
	}
	for _, message := range negative {
		if matches := MatchPackages([]Package{pkg}, MatchInput{Message: message}); len(matches) != 0 {
			t.Fatalf("did not expect %q to match reply-humanizer, got %#v", message, matches)
		}
	}
}

func TestMatchPackagesWeakDescriptionWordDoesNotReachCandidateThreshold(t *testing.T) {
	pkg := Package{
		Name:        "paper-humanizer",
		Description: "Use when polishing academic writing and improving natural expression.",
		Enabled:     true,
	}
	if matches := MatchPackages([]Package{pkg}, MatchInput{Message: "natural"}); len(matches) != 0 {
		t.Fatalf("expected one weak description word to be ignored, got %#v", matches)
	}
}

func TestMatchPackagesDialogueStateCannotTriggerByItself(t *testing.T) {
	pkg := Package{
		Name:        "comfort",
		Description: "Emotional support.",
		Frontmatter: map[string]any{
			"tags": []any{"sad"},
		},
		Enabled: true,
	}
	input := MatchInput{}
	input.DialogueState.Emotion = "sad"
	if matches := MatchPackages([]Package{pkg}, input); len(matches) != 0 {
		t.Fatalf("expected dialogue state alone to be ignored, got %#v", matches)
	}
}

func TestMatchPackagesSkipsDisabledSkills(t *testing.T) {
	pkg := Package{
		Name:        "reply-humanizer",
		Description: "Natural replies.",
		Frontmatter: map[string]any{
			"triggers": []any{"自然一点"},
		},
		Enabled: false,
	}
	if matches := MatchPackages([]Package{pkg}, MatchInput{Message: "自然一点"}); len(matches) != 0 {
		t.Fatalf("expected disabled skill to be skipped, got %#v", matches)
	}
}
