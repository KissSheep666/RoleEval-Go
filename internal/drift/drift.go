// Package drift 做「前后文风格漂移」检测。
//
// 黑盒没有角色卡，但可以假设：同一个角色在前 N 轮的说话方式相对稳定。
// 后面任何一维（句长、语言、口癖、元话语）突变，都值得看一眼。
package drift

import (
	"math"
	"strings"
	"unicode"

	"roleeval/internal/model"
)

// Point 是一条角色消息的风格指标。
type Point struct {
	Turn  int      `json:"turn"`
	Len   int      `json:"len"`
	LenZ  float64  `json:"len_z"`
	CJK   float64  `json:"cjk_ratio"`
	Meta  int      `json:"meta"`
	Catch int      `json:"catchphrase_hits"`
	Flags []string `json:"flags,omitempty"`
}

// Series 是整段对话的漂移曲线。
type Series struct {
	BaselineTurns int     `json:"baseline_turns"`
	MeanLen       float64 `json:"mean_len"`
	StdLen        float64 `json:"std_len"`
	MeanCJK       float64 `json:"mean_cjk"`
	Points        []Point `json:"points"`
}

// Compute 用前 baselineN 条角色消息建基线，再逐条算偏离。
func Compute(conv model.Conversation, p model.PersonaProfile, baselineN int) Series {
	if baselineN <= 0 {
		baselineN = 10
	}
	if p.BaselineTurns > 0 && p.BaselineTurns < baselineN {
		baselineN = p.BaselineTurns
	}

	var turns []model.Turn
	for _, t := range conv.Turns {
		if t.IsUser() || strings.TrimSpace(t.AssistantText()) == "" {
			continue
		}
		turns = append(turns, t)
	}
	s := Series{BaselineTurns: min(baselineN, len(turns))}
	if len(turns) == 0 || s.BaselineTurns == 0 {
		return s
	}

	// 基线统计
	var lens []float64
	var cjks []float64
	for i := 0; i < s.BaselineTurns; i++ {
		txt := turns[i].AssistantText()
		lens = append(lens, float64(runeLen(txt)))
		cjks = append(cjks, cjkRatio(txt))
	}
	s.MeanLen = mean(lens)
	s.StdLen = std(lens, s.MeanLen)
	s.MeanCJK = mean(cjks)

	baselineCatch := 0
	for i := 0; i < s.BaselineTurns; i++ {
		baselineCatch += catchHits(turns[i].AssistantText(), p.Catchphrases)
	}

	for i, t := range turns {
		txt := t.AssistantText()
		pnt := Point{
			Turn:  t.Seq,
			Len:   runeLen(txt),
			CJK:   round4(cjkRatio(txt)),
			Meta:  countMeta(txt),
			Catch: catchHits(txt, p.Catchphrases),
		}
		if s.StdLen > 0 {
			pnt.LenZ = round2((float64(pnt.Len) - s.MeanLen) / s.StdLen)
		}

		if pnt.Meta > 0 {
			pnt.Flags = append(pnt.Flags, "meta_speech")
		}
		if math.Abs(pnt.LenZ) >= 2 {
			pnt.Flags = append(pnt.Flags, "length_jump")
		}
		if s.MeanCJK >= 0.6 && pnt.CJK < 0.3 {
			pnt.Flags = append(pnt.Flags, "language_switch")
		}
		if s.MeanCJK < 0.3 && pnt.CJK > 0.6 {
			pnt.Flags = append(pnt.Flags, "language_switch")
		}
		// 口癖在后半段完全消失，标在第一个消失的点上
		if baselineCatch >= 2 && i >= len(turns)/2 && pnt.Catch == 0 && !hasFlag(turns[:i], p, i) {
			pnt.Flags = append(pnt.Flags, "catchphrase_drop")
		}

		s.Points = append(s.Points, pnt)
	}
	return s
}

func hasFlag(prev []model.Turn, p model.PersonaProfile, upto int) bool {
	// 之前已经标过一次就不重复标
	for i := 0; i < upto && i < len(prev); i++ {
		if catchHits(prev[i].AssistantText(), p.Catchphrases) == 0 {
			return true
		}
	}
	return false
}

func runeLen(s string) int {
	return len([]rune(strings.TrimSpace(s)))
}

func cjkRatio(s string) float64 {
	var cjk, total int
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if unicode.Is(unicode.Han, r) {
			cjk++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(cjk) / float64(total)
}

func catchHits(s string, phrases []string) int {
	n := 0
	for _, p := range phrases {
		if p == "" {
			continue
		}
		n += strings.Count(s, p)
	}
	return n
}

// CountMeta 统计「AI 腔」表达出现次数，供 extract 与 drift 共用。
func CountMeta(s string) int {
	return countMeta(s)
}

func countMeta(s string) int {
	markers := []string{
		"作为一个AI", "作为 AI", "作为一个 AI", "语言模型", "我是AI", "我只是个AI",
		"as an AI", "language model", "I'm an AI", "我是人工智能",
	}
	n := 0
	lower := strings.ToLower(s)
	for _, m := range markers {
		n += strings.Count(s, m)
		n += strings.Count(lower, strings.ToLower(m))
	}
	return n
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func std(xs []float64, m float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
func round4(f float64) float64 { return math.Round(f*10000) / 10000 }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Findings 把漂移曲线转成可展示的问题。
func Findings(s Series) []model.Finding {
	var out []model.Finding
	for _, p := range s.Points {
		for _, f := range p.Flags {
			fd := model.Finding{Turn: p.Turn, Method: "rule", Evidence: "turn " + itoa(p.Turn)}
			switch f {
			case "meta_speech":
				fd.Category, fd.Sub, fd.Severity, fd.Confidence = "out_of_character", "feels_robotic", 3, 0.95
				fd.Reason = "出现「作为一个 AI」类元话语"
			case "language_switch":
				fd.Category, fd.Sub, fd.Severity, fd.Confidence = "out_of_character", "dialogue_not_like_them", 2, 0.8
				fd.Reason = "语言与基线不一致"
			case "length_jump":
				fd.Category, fd.Sub, fd.Severity, fd.Confidence = "writing_quality", "turn_too_long", 1, 0.6
				fd.Reason = "句长明显偏离基线"
			case "catchphrase_drop":
				fd.Category, fd.Sub, fd.Severity, fd.Confidence = "out_of_character", "inconsistent_character", 1, 0.6
				fd.Reason = "口癖在后半段消失"
			default:
				continue
			}
			out = append(out, fd)
		}
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
