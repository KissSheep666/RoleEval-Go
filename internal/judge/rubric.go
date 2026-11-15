package judge

// Category 描述评测分类。分类 key 直接对齐产品反馈体系，
// 这样评测结果和产品同频，也方便对账。
type Category struct {
	Key        string
	Definition string
	Subs       []string
}

// LLMCategories 是需要模型判定的分类（规则判不了的那部分）。
var LLMCategories = []Category{
	{
		Key:        "out_of_character",
		Definition: "角色的言行与其已确立的人设、说话风格不一致",
		Subs:       []string{"not_expected_character", "inconsistent_character", "personality_shift", "dialogue_not_like_them", "one_dimensional", "feels_robotic"},
	},
	{
		Key:        "forgot_story",
		Definition: "角色遗忘了此前已经建立的用户信息、共同经历或剧情设定",
		Subs:       []string{"forgot_about_me", "forgot_about_other_character", "forgot_our_history", "weird_callbacks"},
	},
	{
		Key:        "logically_weird",
		Definition: "对话中的物理细节、事实或逻辑前后对不上",
		Subs:       []string{"physical_details_wrong", "facts_logic_wrong", "other_logic_problem"},
	},
	{
		Key:        "story_losing_me",
		Definition: "剧情节奏失控、缺乏钩子、破坏沉浸感或破坏世界观",
		Subs:       []string{"plot_rushing", "plot_dragging", "nothing_to_hook", "breaks_immersion", "pointless_npcs", "breaks_world"},
	},
	{
		Key:        "took_over_character",
		Definition: "角色替用户说话、行动、写心理或强行决定剧情走向",
		Subs:       []string{"ignored_what_i_said", "acting_for_me", "wrote_my_thoughts", "decided_for_me", "forced_outcome", "invented_my_backstory"},
	},
	{
		Key:        "writing_quality",
		Definition: "文笔质量有问题：用词奇怪、比喻怪异、过度夸张、套话",
		Subs:       []string{"mixed_up_pronouns", "repetitive_or_cliche", "awkward_wording", "weird_metaphors", "overly_exaggerated"},
	},
	{
		Key:        "repetition",
		Definition: "重复同样的句子、套路或情节走向",
		Subs:       []string{"parrots_user_input", "repeated_phrases", "same_plot_patterns"},
	},
	{
		Key:        "nsfw_not_wanted",
		Definition: "亲密内容节奏、氛围或尺度不符合用户期待，或被错误过滤",
		Subs:       []string{"escalating_too_fast", "too_slow_burn", "wrong_vibe", "filtered_wrongly", "crossed_line"},
	},
}

// Find 按 key 查分类。
func Find(key string) (Category, bool) {
	for _, c := range LLMCategories {
		if c.Key == key {
			return c, true
		}
	}
	return Category{}, false
}
