package drift

import (
	"testing"

	"roleeval/internal/model"
)

func aiTurn(seq int, text string) model.Turn {
	return model.Turn{Seq: seq, Sentences: []model.Sentence{
		{Role: "rpui", Kind: "narration", Content: text},
	}}
}

func userTurn(seq int, text string) model.Turn {
	return model.Turn{Seq: seq, Sentences: []model.Sentence{
		{Role: model.RoleUser, Content: text},
	}}
}

func TestDetectMetaSpeech(t *testing.T) {
	conv := model.Conversation{Turns: []model.Turn{
		userTurn(1, "你好"),
		aiTurn(2, "*她冷冷地看了你一眼，没有说话。*"),
		userTurn(3, "你好"),
		aiTurn(4, "*她哼了一声，转身继续擦剑。*"),
		userTurn(5, "你好"),
		aiTurn(6, "*她没有理你。*"),
		userTurn(7, "你怎么不说话"),
		aiTurn(8, "作为一个 AI，我不能假装自己是人类。"),
	}}
	p := model.PersonaProfile{BaselineTurns: 3, Catchphrases: []string{"哼"}}
	s := Compute(conv, p, 3)

	var found bool
	for _, pt := range s.Points {
		for _, f := range pt.Flags {
			if f == "meta_speech" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("应检测到元话语")
	}

	if len(Findings(s)) == 0 {
		t.Fatal("漂移应转成 findings")
	}
}

func TestDetectLanguageSwitch(t *testing.T) {
	conv := model.Conversation{Turns: []model.Turn{
		aiTurn(1, "她看着你，没有说话。"),
		userTurn(2, "嗯"),
		aiTurn(3, "她转过身去，看向窗外。"),
		userTurn(4, "嗯"),
		aiTurn(5, "She turns around and looks at you silently."),
	}}
	s := Compute(conv, model.PersonaProfile{}, 2)

	var found bool
	for _, pt := range s.Points {
		for _, f := range pt.Flags {
			if f == "language_switch" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("应检测到语言切换")
	}
}

func TestNoFlagOnStableStyle(t *testing.T) {
	conv := model.Conversation{Turns: []model.Turn{
		aiTurn(1, "她看了你一眼。"),
		aiTurn(2, "她转过身去。"),
		aiTurn(3, "她哼了一声。"),
		aiTurn(4, "她没有说话。"),
	}}
	s := Compute(conv, model.PersonaProfile{}, 3)
	for _, pt := range s.Points {
		if len(pt.Flags) > 0 {
			t.Fatalf("稳定风格不应报警，第 %d 轮: %v", pt.Turn, pt.Flags)
		}
	}
}
