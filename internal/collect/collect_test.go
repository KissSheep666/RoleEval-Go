package collect

import (
	"strings"
	"testing"
)

// 抓包数据里 turn_list 和 sentence_list 都是「新 → 旧」，
// 归一化必须按时间升序重排，否则评测顺序会反。
func TestNormalizeOrder(t *testing.T) {
	raw := []byte(`{
	  "code":0,
	  "data":{"message_list":[{
	    "user_id":1,"character_id":42,"conversation_id":"c1",
	    "turn_list":[
	      {"turn_id":"t2","turn_type":"normal","sentence_list":[
	        {"sentence_id":"s2","role":"user","content":"第二句","sent_ts":200}
	      ]},
	      {"turn_id":"t1","turn_type":"opening_words","sentence_list":[
	        {"sentence_id":"s1b","role":"rpui","content":"后","sent_ts":110,"extra":{"status":"complete"}},
	        {"sentence_id":"s1a","role":"rpui","content":"前","sent_ts":100,"extra":{"status":"complete"}}
	      ]}
	    ]
	  }]}
	}`)

	convs, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("期望 1 个会话，得到 %d", len(convs))
	}
	c := convs[0]
	if len(c.Turns) != 2 {
		t.Fatalf("期望 2 轮，得到 %d", len(c.Turns))
	}
	if !strings.HasPrefix(c.Turns[0].ID, "t1") || !strings.HasPrefix(c.Turns[1].ID, "t2") {
		t.Fatalf("轮次顺序错误: %s, %s", c.Turns[0].ID, c.Turns[1].ID)
	}
	if c.Turns[0].Seq != 1 || c.Turns[1].Seq != 2 {
		t.Fatalf("seq 编号错误: %d, %d", c.Turns[0].Seq, c.Turns[1].Seq)
	}
	if got := c.Turns[0].Sentences[0].Content; got != "前" {
		t.Fatalf("句子顺序错误，第一句应为「前」，得到 %q", got)
	}
}

func TestParseCaptureArray(t *testing.T) {
	raw := []byte(`[
	  {"src":"fetch","url":"https://api/x/chat/turn-list?x=1","body":"{\"code\":0,\"data\":{\"message_list\":[{\"conversation_id\":\"c2\",\"character_id\":7,\"turn_list\":[{\"turn_id\":\"t1\",\"sentence_list\":[{\"sentence_id\":\"s1\",\"role\":\"user\",\"content\":\"hi\",\"sent_ts\":1}]}]}]}}"},
	  {"src":"fetch","url":"https://api/x/other","body":"{}"}
	]`)

	convs, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(convs) != 1 || convs[0].ID != "c2" {
		t.Fatalf("应当从抓包里提取出 c2，得到 %+v", convs)
	}
}

// Popia 的一个 turn 里同时包含用户提问和角色回复，
// 归一化后必须拆成两条，否则角色回复永远不会被评测。
func TestSplitUserAndAssistant(t *testing.T) {
	raw := []byte(`{"code":0,"data":{"message_list":[{
	  "conversation_id":"c3","character_id":1,
	  "turn_list":[{"turn_id":"t1","turn_type":"normal","model_name":"Auto","sentence_list":[
	    {"sentence_id":"a1","role":"rpui","type":"narration","content":"旁白","sent_ts":200},
	    {"sentence_id":"u1","role":"user","content":"用户的话","sent_ts":100}
	  ]}]
	}]}}`)

	convs, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	c := convs[0]
	if len(c.Turns) != 2 {
		t.Fatalf("应拆成 2 条消息，得到 %d", len(c.Turns))
	}
	if !c.Turns[0].IsUser() || c.Turns[0].UserText() != "用户的话" {
		t.Fatalf("第一条应为用户消息，得到 %+v", c.Turns[0])
	}
	if c.Turns[1].IsUser() || c.Turns[1].AssistantText() != "旁白" {
		t.Fatalf("第二条应为角色消息，得到 %+v", c.Turns[1])
	}
	if c.Turns[0].Seq != 1 || c.Turns[1].Seq != 2 {
		t.Fatalf("seq 错误: %d, %d", c.Turns[0].Seq, c.Turns[1].Seq)
	}
}
