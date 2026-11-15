package persona

import (
	"testing"

	"roleeval/internal/model"
)

func TestHeuristic(t *testing.T) {
	mk := func(seq int, text string) model.Turn {
		return model.Turn{Seq: seq, Sentences: []model.Sentence{
			{Role: "rpui", Kind: "narration", Content: text},
		}}
	}
	conv := model.Conversation{Turns: []model.Turn{
		{Seq: 1, Sentences: []model.Sentence{{Role: model.RoleUser, Content: "你是谁"}}},
		mk(2, "*她哼了一声，冷冷地看着你，没有说话。*"),
		mk(3, "*她又哼了一声，转身去擦剑。*"),
		mk(4, "*哼，少废话。她转过身，没再看你。*"),
	}}

	p := Heuristic(conv, 10)
	if p.Source != "heuristic" {
		t.Fatalf("来源应为 heuristic，得到 %q", p.Source)
	}
	if p.AvgLen == 0 {
		t.Fatal("平均句长不应为 0")
	}
	if p.AddressToUser == "" {
		t.Fatal("应识别出对用户的称呼")
	}
	if len(p.Catchphrases) == 0 {
		t.Fatal("应识别出口癖")
	}
}
