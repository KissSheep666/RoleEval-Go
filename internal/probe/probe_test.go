package probe

import (
	"testing"

	"roleeval/internal/model"
)

func convWithAnswer(answer string) model.Conversation {
	return model.Conversation{Turns: []model.Turn{
		{Seq: 1, Sentences: []model.Sentence{{Role: model.RoleUser, Content: "我叫阿沅，住在天字二号房。"}}},
		{Seq: 2, Sentences: []model.Sentence{{Role: "rpui", Kind: "dialogue", Content: "记住了。"}}},
		{Seq: 3, Sentences: []model.Sentence{{Role: model.RoleUser, Content: "我住哪个房间？"}}},
		{Seq: 4, Sentences: []model.Sentence{{Role: "rpui", Kind: "dialogue", Content: answer}}},
	}}
}

func TestScoreRecalled(t *testing.T) {
	spec := Spec{Questions: []string{"我住哪个房间？"}, Expected: []string{"天字二号房"}}
	fs := Score(spec, convWithAnswer("你住在天字二号房。"))
	if len(fs) != 0 {
		t.Fatalf("召回正确时不应有问题，得到 %+v", fs)
	}
}

func TestScoreForgotten(t *testing.T) {
	spec := Spec{Questions: []string{"我住哪个房间？"}, Expected: []string{"天字二号房"}}
	fs := Score(spec, convWithAnswer("我记不太清了。"))
	if len(fs) != 1 || fs[0].Sub != "forgot_our_history" {
		t.Fatalf("应判定为遗忘，得到 %+v", fs)
	}
}

func TestScoreHallucinated(t *testing.T) {
	spec := Spec{Questions: []string{"我住哪个房间？"}, Expected: []string{"天字二号房"}}
	fs := Score(spec, convWithAnswer("你住在三号房。"))
	if len(fs) != 1 || fs[0].Sub != "facts_logic_wrong" || fs[0].Severity != 3 {
		t.Fatalf("答成别的房号应判为编造，得到 %+v", fs)
	}
}
