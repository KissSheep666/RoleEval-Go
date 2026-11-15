// Package fuse 合并来自不同来源的判定。
//
// 规则和模型经常会命中同一个问题（比如同一轮既被判重复、又被模型判重复）。
// 如果直接相加，指标会虚高，所以按 (分类, 子类, 轮次) 合并：
//   - 取最高严重度
//   - 把 method 标成 rule+llm
package fuse

import (
	"fmt"

	"roleeval/internal/model"
)

// Merge 合并重复判定。
func Merge(fs []model.Finding) []model.Finding {
	index := map[string]int{}
	out := make([]model.Finding, 0, len(fs))

	for _, f := range fs {
		key := fmt.Sprintf("%s|%s|%d", f.Category, f.Sub, f.Turn)
		i, ok := index[key]
		if !ok {
			index[key] = len(out)
			out = append(out, f)
			continue
		}

		cur := out[i]
		if f.Severity > cur.Severity {
			cur.Severity = f.Severity
		}
		if f.Method != cur.Method {
			cur.Method = "rule+llm"
		}
		if cur.Evidence == "" {
			cur.Evidence = f.Evidence
		}
		if cur.Reason == "" {
			cur.Reason = f.Reason
		}
		if f.Confidence > cur.Confidence {
			cur.Confidence = f.Confidence
		}
		out[i] = cur
	}
	return out
}
