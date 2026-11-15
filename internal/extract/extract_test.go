package extract

import (
	"strings"
	"testing"

	"roleeval/internal/model"
)

func user(seq int, text string) model.Turn {
	return model.Turn{Seq: seq, Sentences: []model.Sentence{
		{ID: "u", Role: model.RoleUser, Content: text, TS: int64(seq)},
	}}
}

func ai(seq int, sentences ...model.Sentence) model.Turn {
	for i := range sentences {
		sentences[i].TS = int64(seq*10 + i)
	}
	return model.Turn{Seq: seq, Sentences: sentences}
}

func nar(text string) model.Sentence {
	return model.Sentence{Role: "rpui", Kind: "narration", Content: text}
}

func dia(text string) model.Sentence {
	return model.Sentence{Role: "rpui", Kind: "dialogue", Content: text}
}

func has(fs []model.Finding, cat, sub string) bool {
	for _, f := range fs {
		if f.Category == cat && (sub == "" || f.Sub == sub) {
			return true
		}
	}
	return false
}

func TestFormatLeakAndCut(t *testing.T) {
	c := model.Conversation{Turns: []model.Turn{
		user(1, "你好"),
		ai(2, nar(`*她看了你一眼* 好的，{"action": "attack"`)),
	}}
	fs, _ := Run(c)

	if !has(fs, "format_broken", "garbled_or_leaked") {
		t.Fatal("应检测到 JSON 泄漏")
	}
	if !has(fs, "format_broken", "cut_off") {
		t.Fatal("应检测到截断")
	}
}

func TestTookOverCharacter(t *testing.T) {
	c := model.Conversation{Turns: []model.Turn{
		user(1, "我站着不动"),
		ai(2, nar("*你感到一阵刺痛，你决定忍住。*")),
	}}
	fs, _ := Run(c)

	if !has(fs, "took_over_character", "wrote_my_thoughts") {
		t.Fatal("应检测到替用户写心理")
	}
}

func TestAddressChangeIgnored(t *testing.T) {
	c := model.Conversation{Turns: []model.Turn{
		user(1, "别叫我公子，叫我阿沅。"),
		ai(2, dia("公子，你说笑了。")),
	}}
	fs, _ := Run(c)

	if !has(fs, "forgot_story", "forgot_about_me") {
		t.Fatal("用户已要求改口，仍用旧称呼应被检出")
	}
}

func TestRepetition(t *testing.T) {
	long := strings.Repeat("她的声音不大，却像一把短刀精准地切断所有杂音。", 3)
	c := model.Conversation{Turns: []model.Turn{
		user(1, "继续"),
		ai(2, nar(long)),
		user(3, "然后呢"),
		ai(4, nar(long)), // 原样重复
	}}
	fs, _ := Run(c)

	if !has(fs, "repetition", "repeated_phrases") {
		t.Fatal("应检测到重复输出")
	}
}

func TestTooLong(t *testing.T) {
	long := strings.Repeat("这是一段很长的描写。", 60) // 600 字
	c := model.Conversation{Turns: []model.Turn{
		user(1, "说说看"),
		ai(2, nar(long)),
	}}
	fs, _ := Run(c)

	if !has(fs, "writing_quality", "turn_too_long") {
		t.Fatal("应检测到单轮过长")
	}
}

func TestFacts(t *testing.T) {
	c := model.Conversation{Turns: []model.Turn{
		user(1, "我叫阿沅，住在天字二号房，我明天要去码头接人。"),
		ai(2, dia("记住了。")),
	}}
	_, facts := Run(c)

	values := map[string]string{}
	for _, f := range facts {
		values[f.Key] = f.Value
	}
	if values["name"] != "阿沅" {
		t.Errorf("名字抽取错误: %q", values["name"])
	}
	if !strings.Contains(values["residence"], "天字二号房") {
		t.Errorf("住处抽取错误: %q", values["residence"])
	}
	if values["tomorrow"] == "" {
		t.Error("明天计划未抽取")
	}
}
