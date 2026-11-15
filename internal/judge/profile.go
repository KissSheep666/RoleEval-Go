package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"roleeval/internal/model"
)

const profilePrompt = `你是人物画像分析器。下面是某个角色在对话前若干条消息里的表现。
请只总结「稳定的特征」，不要总结剧情。输出严格 JSON：

{"self":"角色的自称","address_to_user":"角色对用户的称呼","catchphrases":["口癖或固定说法"],
 "tone":"语气（短句/长叙事/命令式/温柔 等）","values":["价值观"],
 "knowledge_bounds":["角色知道什么、不知道什么"],"taboos":["禁忌话题"]}

没有把握的字段留空数组或空字符串。只输出 JSON，不要解释。`

// ExtractProfile 用模型从前 N 条角色消息里抽人物画像。
// 失败时调用方应回退到 persona.Heuristic。
func (c Config) ExtractProfile(ctx context.Context, conv model.Conversation, firstN int) (model.PersonaProfile, error) {
	if firstN <= 0 {
		firstN = 10
	}

	var b strings.Builder
	var texts []string
	for _, t := range conv.Turns {
		if t.IsUser() {
			continue
		}
		txt := strings.TrimSpace(t.AssistantText())
		if txt == "" {
			continue
		}
		texts = append(texts, txt)
		b.WriteString(fmt.Sprintf("### 第 %d 条\n%s\n\n", t.Seq, truncateRunes(txt, 500)))
		if len(texts) >= firstN {
			break
		}
	}
	if len(texts) == 0 {
		return model.PersonaProfile{}, fmt.Errorf("没有角色消息，无法抽取画像")
	}

	text, err := c.chat(ctx, profilePrompt, b.String())
	if err != nil {
		return model.PersonaProfile{}, err
	}

	p, err := parseProfile(text)
	if err != nil {
		return model.PersonaProfile{}, err
	}
	p.Source = "llm"
	p.BaselineTurns = len(texts)

	if p.AvgLen == 0 {
		total := 0
		for _, t := range texts {
			total += utf8.RuneCountInString(t)
		}
		p.AvgLen = total / len(texts)
	}
	return p, nil
}

func parseProfile(text string) (model.PersonaProfile, error) {
	text = strings.TrimSpace(text)
	if m := fenceRe.FindStringSubmatch(text); m != nil {
		text = strings.TrimSpace(m[1])
	}
	if i := strings.Index(text, "{"); i >= 0 {
		if j := strings.LastIndex(text, "}"); j > i {
			text = text[i : j+1]
		}
	}

	var p model.PersonaProfile
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		return model.PersonaProfile{}, fmt.Errorf("画像不是合法 JSON: %w; 原文=%s", err, clipStr(text, 200))
	}
	return p, nil
}
