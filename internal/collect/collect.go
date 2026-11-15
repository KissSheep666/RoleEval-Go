// Package collect 负责把各种来源的数据统一成 model.Conversation。
//
// 支持的输入：
//  1. Popia 的 turn-list 原始响应（{"code":0,"data":{"message_list":[...]}}）
//  2. 油猴脚本导出的抓包数组（[{"src":"fetch","url":"...","body":"..."}]）
//  3. 本工具自己导出的 conversation JSON
package collect

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"roleeval/internal/model"
)

// Load 读取一个文件，返回其中所有会话。
func Load(path string) ([]model.Conversation, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

// Parse 从原始字节解析会话。
func Parse(raw []byte) ([]model.Conversation, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("空文件")
	}

	switch trimmed[0] {
	case '[':
		return parseArray(raw)
	case '{':
		if convs, ok := tryTurnList(raw); ok {
			return convs, nil
		}
		var c model.Conversation
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("无法识别的 JSON 结构: %w", err)
		}
		if len(c.Turns) == 0 {
			return nil, fmt.Errorf("会话里没有 turns")
		}
		normalizeConv(&c)
		return []model.Conversation{c}, nil
	default:
		return nil, fmt.Errorf("无法识别的输入格式")
	}
}

// record 是抓包数组里的一个元素。
type record struct {
	Src  string `json:"src"`
	URL  string `json:"url"`
	Body string `json:"body"`
}

// parseArray 处理「抓包数组」或「会话数组」。
func parseArray(raw []byte) ([]model.Conversation, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	// 先猜：是不是抓包数组（元素带 src/url）
	var recs []record
	isCapture := true
	for _, it := range items {
		var r record
		if err := json.Unmarshal(it, &r); err != nil || r.Src == "" {
			isCapture = false
			break
		}
		recs = append(recs, r)
	}

	if isCapture {
		return fromCapture(recs)
	}

	// 否则当作会话数组
	var convs []model.Conversation
	if err := json.Unmarshal(raw, &convs); err != nil {
		return nil, err
	}
	for i := range convs {
		normalizeConv(&convs[i])
	}
	return convs, nil
}

// fromCapture 从抓包记录里挑出 turn-list 的响应体。
func fromCapture(recs []record) ([]model.Conversation, error) {
	var out []model.Conversation
	seen := map[string]bool{}

	for _, r := range recs {
		if !strings.Contains(r.URL, "turn-list") {
			continue
		}
		convs, ok := tryTurnList([]byte(r.Body))
		if !ok {
			continue
		}
		for _, c := range convs {
			if c.ID != "" && seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("抓包里没有找到 turn-list 响应")
	}
	return out, nil
}

type turnListResp struct {
	Code int `json:"code"`
	Data struct {
		HasMore     bool `json:"has_more"`
		MessageList []struct {
			UserID         int64  `json:"user_id"`
			CharacterID    int64  `json:"character_id"`
			ConversationID string `json:"conversation_id"`
			TurnList       []struct {
				TurnID       string `json:"turn_id"`
				TurnType     string `json:"turn_type"`
				ModelID      string `json:"model_id"`
				ModelName    string `json:"model_name"`
				SentenceList []struct {
					SentenceID string `json:"sentence_id"`
					Type       string `json:"type"`
					Role       string `json:"role"`
					Content    string `json:"content"`
					SentTS     int64  `json:"sent_ts"`
					Extra      struct {
						Status string `json:"status"`
					} `json:"extra"`
				} `json:"sentence_list"`
			} `json:"turn_list"`
		} `json:"message_list"`
	} `json:"data"`
}

// tryTurnList 尝试按 turn-list 响应解析。
func tryTurnList(raw []byte) ([]model.Conversation, bool) {
	var resp turnListResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, false
	}
	if len(resp.Data.MessageList) == 0 {
		return nil, false
	}

	var out []model.Conversation
	for _, m := range resp.Data.MessageList {
		c := model.Conversation{ID: m.ConversationID, CharacterID: m.CharacterID}
		for _, t := range m.TurnList {
			turn := model.Turn{
				ID:        t.TurnID,
				Kind:      t.TurnType,
				ModelID:   t.ModelID,
				ModelName: t.ModelName,
			}
			for _, s := range t.SentenceList {
				turn.Sentences = append(turn.Sentences, model.Sentence{
					ID:      s.SentenceID,
					Role:    s.Role,
					Kind:    s.Type,
					Content: s.Content,
					TS:      s.SentTS,
					Status:  s.Extra.Status,
				})
			}
			c.Turns = append(c.Turns, turn)
		}
		if len(c.Turns) == 0 {
			continue
		}
		normalizeConv(&c)
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// normalizeConv 排序、拆分并编号。
//
// Popia 的一「turn」其实是「一轮」：用户那句话和角色的回复在同一个 turn 里。
// 所以必须先按 role 拆成「用户消息」和「角色消息」两条，再做评测——
// 否则整轮会被当成用户轮，角色的回复永远不会被检查。
//
// 另外抓包数据里 turn_list 和 sentence_list 都是「新 → 旧」，
// 所以两处都要按时间升序重排。
func normalizeConv(c *model.Conversation) {
	var flat []model.Turn

	for _, raw := range c.Turns {
		sort.SliceStable(raw.Sentences, func(a, b int) bool {
			return raw.Sentences[a].TS < raw.Sentences[b].TS
		})
		flat = append(flat, splitTurn(raw)...)
	}

	sort.SliceStable(flat, func(i, j int) bool {
		return turnTS(flat[i]) < turnTS(flat[j])
	})
	for i := range flat {
		flat[i].Seq = i + 1
	}
	c.Turns = flat
}

// splitTurn 把一轮拆成「用户消息」+「角色消息」。
func splitTurn(raw model.Turn) []model.Turn {
	var users, others []model.Sentence
	for _, s := range raw.Sentences {
		if s.Role == model.RoleUser || s.Role == "user" {
			users = append(users, s)
			continue
		}
		others = append(others, s)
	}

	var out []model.Turn
	if len(users) > 0 {
		out = append(out, model.Turn{
			ID:        raw.ID + "-u",
			Kind:      "user",
			Sentences: users,
		})
	}
	if len(others) > 0 {
		out = append(out, model.Turn{
			ID:        raw.ID + "-a",
			Kind:      raw.Kind,
			ModelID:   raw.ModelID,
			ModelName: raw.ModelName,
			Sentences: others,
		})
	}
	return out
}

func turnTS(t model.Turn) int64 {
	var min int64
	for _, s := range t.Sentences {
		if s.TS == 0 {
			continue
		}
		if min == 0 || s.TS < min {
			min = s.TS
		}
	}
	return min
}
