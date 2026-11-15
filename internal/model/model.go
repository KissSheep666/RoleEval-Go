// Package model 定义评测系统的核心数据结构。
package model

import (
	"strings"
)

// Sentence 是对话里的一个最小单位：一句旁白、一句台词，或一张图。
type Sentence struct {
	ID      string `json:"id"`
	Role    string `json:"role"` // user | rpui | assistant | image_url
	Kind    string `json:"kind"` // narration | dialogue | image
	Content string `json:"content"`
	TS      int64  `json:"ts,omitempty"`
	Status  string `json:"status,omitempty"`
}

// Turn 是一轮交互：用户说一句，角色回一段。
type Turn struct {
	ID        string     `json:"id"`
	Seq       int        `json:"seq"`
	Kind      string     `json:"kind"` // opening_words | normal
	ModelID   string     `json:"model_id,omitempty"`
	ModelName string     `json:"model_name,omitempty"`
	Sentences []Sentence `json:"sentences"`
}

// Conversation 是一次完整会话。
type Conversation struct {
	ID          string `json:"id"`
	CharacterID int64  `json:"character_id"`
	Turns       []Turn `json:"turns"`
}

// 角色类型
const (
	RoleUser      = "user"
	RoleAssistant = "rpui"
	RoleImage     = "image_url"
)

// IsUser 判断这一条消息是不是用户发的。
// 归一化之后每条 Turn 只包含一侧的句子。
func (t Turn) IsUser() bool {
	for _, s := range t.Sentences {
		if s.Role == RoleUser || s.Role == "user" {
			return true
		}
	}
	return false
}

// UserText 拼接这一轮所有用户句子。
func (t Turn) UserText() string {
	var b strings.Builder
	for _, s := range t.Sentences {
		if s.Role == RoleUser || s.Role == "user" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(s.Content)
		}
	}
	return b.String()
}

// AssistantText 拼接这一轮所有角色输出（旁白 + 台词）。
func (t Turn) AssistantText() string {
	var b strings.Builder
	for _, s := range t.Sentences {
		if s.Role == RoleImage || s.Kind == "image" {
			continue
		}
		if s.Role == RoleUser || s.Role == "user" {
			continue
		}
		if s.Content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(s.Content)
	}
	return b.String()
}

// NarrationText 只取旁白部分。
func (t Turn) NarrationText() string {
	var b strings.Builder
	for _, s := range t.Sentences {
		if s.Kind == "narration" && s.Content != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(s.Content)
		}
	}
	return b.String()
}

// Finding 是评测系统的一条判定。
// 所有判定都必须带 Evidence，否则报告不可读。
type Finding struct {
	Category   string   `json:"category"`       // 9 大类 key
	Sub        string   `json:"sub,omitempty"`  // 子标签 key
	Severity   int      `json:"severity"`       // 1 轻微 2 明显 3 破坏
	Turn       int      `json:"turn,omitempty"` // 轮次 seq，0 表示会话级
	SentenceID string   `json:"sentence_id,omitempty"`
	Evidence   string   `json:"evidence,omitempty"`   // 原文引用
	Reason     string   `json:"reason,omitempty"`     // 为什么判它
	Method     string   `json:"method"`               // rule | llm | probe | human
	Confidence float64  `json:"confidence,omitempty"` // 0-1
	Votes      int      `json:"votes,omitempty"`      // 多采样时命中次数
	Samples    int      `json:"samples,omitempty"`    // 多采样时总次数
	Tags       []string `json:"tags,omitempty"`
}

// Result 是一次评测的完整产出。
type Result struct {
	ConversationID string         `json:"conversation_id"`
	CharacterID    int64          `json:"character_id"`
	TurnCount      int            `json:"turn_count"`
	Findings       []Finding      `json:"findings"`
	Metrics        map[string]any `json:"metrics,omitempty"`
	GeneratedAt    string         `json:"generated_at"`
}

// PersonaProfile 是从对话里抽出来的「人物画像」，用作 OOC 判定的基线。
// 黑盒场景下拿不到角色卡，只能用前 N 轮自己建基线。
type PersonaProfile struct {
	Self          string   `json:"self,omitempty"`            // 自称
	AddressToUser string   `json:"address_to_user,omitempty"` // 对用户的称呼
	Catchphrases  []string `json:"catchphrases,omitempty"`    // 口癖
	Tone          string   `json:"tone,omitempty"`            // 语气
	Values        []string `json:"values,omitempty"`          // 价值观
	Knowledge     []string `json:"knowledge_bounds,omitempty"`
	Taboos        []string `json:"taboos,omitempty"`
	AvgLen        int      `json:"avg_len,omitempty"` // 基线平均句长
	Source        string   `json:"source,omitempty"`  // llm | heuristic
	BaselineTurns int      `json:"baseline_turns,omitempty"`
}
