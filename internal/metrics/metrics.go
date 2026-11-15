// Package metrics 把逐条 Finding 聚合成可读的数字。
package metrics

import (
	"fmt"
	"strconv"

	"roleeval/internal/model"
)

// Summary 是一次评测的汇总。
type Summary struct {
	TotalTurns      int            `json:"total_turns"`
	TotalFindings   int            `json:"total_findings"`
	AffectedTurns   int            `json:"affected_turns"`
	ErrorRate       float64        `json:"error_rate"`
	ByCategory      map[string]int `json:"by_category"`
	BySub           map[string]int `json:"by_sub"`
	BySeverity      map[string]int `json:"by_severity"`
	ByMethod        map[string]int `json:"by_method"`
	ByTurn          map[string]int `json:"by_turn"`
	WorstCategories []string       `json:"worst_categories,omitempty"`
}

// Compute 计算汇总。
func Compute(conv model.Conversation, findings []model.Finding) Summary {
	s := Summary{
		TotalTurns:    len(conv.Turns),
		TotalFindings: len(findings),
		ByCategory:    map[string]int{},
		BySub:         map[string]int{},
		BySeverity:    map[string]int{},
		ByMethod:      map[string]int{},
		ByTurn:        map[string]int{},
	}

	turns := map[int]bool{}
	for _, f := range findings {
		s.ByCategory[f.Category]++
		if f.Sub != "" {
			s.BySub[f.Category+"/"+f.Sub]++
		}
		s.BySeverity[strconv.Itoa(f.Severity)]++
		s.ByMethod[f.Method]++
		if f.Turn > 0 {
			s.ByTurn[strconv.Itoa(f.Turn)]++
			turns[f.Turn] = true
		}
	}
	s.AffectedTurns = len(turns)
	if s.TotalTurns > 0 {
		s.ErrorRate = float64(s.AffectedTurns) / float64(s.TotalTurns)
	}
	s.WorstCategories = worst(s.ByCategory, 4)
	return s
}

func worst(m map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	var xs []kv
	for k, v := range m {
		xs = append(xs, kv{k, v})
	}
	// 简单插入排序，数据量很小
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j].v > xs[j-1].v; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
	var out []string
	for i := 0; i < len(xs) && i < n; i++ {
		out = append(out, fmt.Sprintf("%s(%d)", xs[i].k, xs[i].v))
	}
	return out
}
