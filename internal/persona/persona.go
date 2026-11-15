// Package persona 从对话里建立「人物画像」基线。
//
// 黑盒场景拿不到角色卡，只能用它自己的前 N 轮当标准：
// 自称、称呼、口癖、平均句长。LLM 抽取在 judge 包里（避免循环依赖）。
package persona

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"roleeval/internal/model"
)

var stopChars = map[rune]bool{
	'的': true, '了': true, '是': true, '在': true, '我': true, '不': true, '你': true,
	'他': true, '她': true, '这': true, '那': true, '有': true, '和': true, '就': true,
	'都': true, '一': true, '个': true, '上': true, '也': true, '很': true, '到': true,
	'说': true, '要': true, '去': true, '会': true, '着': true, '没': true, '好': true,
	'看': true, '自': true, '己': true, '把': true, '被': true, '让': true, '给': true,
}

// Heuristic 不调模型，用统计方法建一个粗略画像。
func Heuristic(conv model.Conversation, firstN int) model.PersonaProfile {
	if firstN <= 0 {
		firstN = 10
	}

	var texts []string
	var total int
	for _, t := range conv.Turns {
		if t.IsUser() {
			continue
		}
		txt := strings.TrimSpace(t.AssistantText())
		if txt == "" {
			continue
		}
		texts = append(texts, txt)
		total += utf8.RuneCountInString(txt)
		if len(texts) >= firstN {
			break
		}
	}

	p := model.PersonaProfile{Source: "heuristic", BaselineTurns: len(texts)}
	if len(texts) == 0 {
		return p
	}
	p.AvgLen = total / len(texts)

	joined := strings.Join(texts, "\n")
	p.Self = "我"
	p.AddressToUser = mostFrequentAddress(joined)
	p.Catchphrases = catchphrases(texts, 3)
	p.Tone = toneHint(p.AvgLen)
	return p
}

func toneHint(avg int) string {
	switch {
	case avg <= 25:
		return "短句、简洁"
	case avg <= 80:
		return "中等长度、有描写"
	default:
		return "长句、叙事细腻"
	}
}

func mostFrequentAddress(s string) string {
	cands := []string{"你", "您", "阁下", "公子", "客官", "先生", "大人"}
	best, bestN := "", 0
	for _, c := range cands {
		if n := strings.Count(s, c); n > bestN {
			best, bestN = c, n
		}
	}
	return best
}

// 单字语气词也算口癖（"哼""嗤"在中文角色扮演里很常见）
var interjections = []string{"哼", "嗤", "呵", "唉", "啧", "唔", "噢", "嗯", "喂", "哈"}

func catchphrases(texts []string, top int) []string {
	counts := map[string]int{}
	for _, txt := range texts {
		for _, w := range interjections {
			if n := strings.Count(txt, w); n > 0 {
				counts[w] += n
			}
		}
	}

	for _, txt := range texts {
		// 按标点切，避免跨句拼出伪口癖
		segs := strings.FieldsFunc(txt, func(r rune) bool {
			return unicode.IsPunct(r) || unicode.IsSpace(r) || unicode.IsSymbol(r)
		})
		for _, seg := range segs {
			r := []rune(strings.TrimSpace(seg))
			for n := 2; n <= 3; n++ {
				for i := 0; i+n <= len(r); i++ {
					gram := string(r[i : i+n])
					if hasStop(gram) || utf8.RuneCountInString(gram) != n {
						continue
					}
					counts[gram]++
				}
			}
		}
	}

	type kv struct {
		k string
		v int
	}
	var xs []kv
	for k, v := range counts {
		if v >= 3 {
			xs = append(xs, kv{k, v})
		}
	}
	sort.Slice(xs, func(i, j int) bool {
		if xs[i].v != xs[j].v {
			return xs[i].v > xs[j].v
		}
		return xs[i].k < xs[j].k
	})

	var out []string
	for i := 0; i < len(xs) && i < top; i++ {
		out = append(out, xs[i].k)
	}
	return out
}

func hasStop(gram string) bool {
	for _, r := range gram {
		if stopChars[r] {
			return true
		}
	}
	return false
}
