# roleeval

Roleplay 对话的**离线评测工具**。把抓到的会话（或半自动探针跑出来的会话）丢进去，输出一份带证据的问题清单和 HTML 报告。

- 规则指标：零成本、可复现，覆盖格式 / 长度 / 重复 / 人称 / 称呼一致性
- OOC 检测：黑盒下用前 N 条消息建人物画像，再做风格漂移与语义判定
- LLM 判定：只判语义层（OOC、忘剧情、逻辑、剧情节奏），输出必须带原文证据
- 分类体系直接对齐产品自己的反馈 taxonomy，评测结果和产品同频

## 快速开始

```bash
# 1. 从抓包或 turn-list 响应里提取会话
go run ./cmd/roleeval import testdata/sample.json -o data/conversations.json

# 2. 只跑规则（不需要 API key）
go run ./cmd/roleeval eval data/conversations.json --out data/report.html --json data/results.json

# 3. 加上 OOC 检测（人物画像 + 风格漂移）
go run ./cmd/roleeval eval data/conversations.json --ooc --baseline 10 --out data/report_ooc.html

# 3b. 只抽人物画像
go run ./cmd/roleeval profile data/conversations.json -o data/profile.json

# 4. 加上 LLM 判定
export ROLEEVAL_PROVIDER=openai          # openai | anthropic
export ROLEEVAL_BASE_URL=https://api.deepseek.com/v1
export ROLEEVAL_API_KEY=sk-xxx
export ROLEEVAL_MODEL=deepseek-chat
go run ./cmd/roleeval eval data/conversations.json --llm --ooc --out data/report.html
```

## 演示界面（推荐）

```bash
# 启动后自动打开浏览器
go run ./cmd/roleeval serve --data data/conversations.json

# 带 LLM 判定（先配置环境变量）
go run ./cmd/roleeval serve --llm
```

打开 `http://127.0.0.1:8080`，可以看到：

- **会话列表**：问题多的排前面，显示轮数 / 发现数 / 命中率
- **时间线**：逐轮展示，命中的轮次标红，问题徽章带严重度与来源
- **分类筛选**：点分类只留该类问题，演示时可以逐个讲
- **重新评测**：按钮直接跑一遍（配了 key 且带 `--llm` 时走多采样）
- **导入抓包**：页面上传 JSON，自动解析、去重、评测

## 记忆探针（半自动）

回放模式不能主动提问，所以记忆类指标用「半自动探针」：系统生成脚本，人照着输入，抓包回收后自动评分。

```bash
# 生成脚本
go run ./cmd/roleeval probe gen testdata/probe_memory.json -o data/probe.md

# 按脚本聊完后导出抓包，再评分
go run ./cmd/roleeval probe score testdata/probe_memory.json data/conversations.json
```

探针能区分三种结果：**召回 / 遗忘 / 编造**。编造比遗忘严重——它会给出一个错误的事实。

## 数据流

```
抓包 JSON / turn-list 响应
        │  collect：按时间升序重排（抓包数据是「新→旧」）
        ▼
   Conversation
        │
        ├─ extract：规则抽取 → Finding[]（method=rule）
        ├─ persona：人物画像 → OOC 判定基线
        ├─ drift：风格漂移   → Finding[]（method=rule）
        ├─ judge：LLM 判定   → Finding[]（method=llm）
        └─ probe：探针评分   → Finding[]（method=probe）
        ▼
   metrics 聚合 → report（HTML）+ results（JSON）
```

## 分类与判定方式

| 分类 | 规则 | LLM | 说明 |
|---|---|---|---|
| `format_broken` | ✅ | — | 乱码 / JSON 泄漏 / 截断 / 空回复 |
| `repetition` | ✅ | 部分 | 复读输入、跨轮重复、套路重复 |
| `writing_quality` | ✅ | 部分 | 长度异常、人称、用词、比喻 |
| `out_of_character` | 部分 | ✅ | 人设漂移、前后不一致、假人感 |
| `forgot_story` | 部分 | ✅ | 称呼/事实对账 + 共同经历 |
| `logically_weird` | 部分 | ✅ | 物理细节、事实逻辑 |
| `story_losing_me` | 部分 | ✅ | 节奏、钩子、沉浸感、世界观 |
| `took_over_character` | ✅ | 部分 | 替用户说话/行动/写心理 |
| `nsfw_not_wanted` | 部分 | ✅ | 节奏、氛围、误过滤 |

规则先跑，LLM 只判规则判不了的语义问题——省 token，也避免模型对明显问题胡说。

## 目录

```
cmd/roleeval/        CLI
internal/collect/    turn-list / 抓包 → Conversation
internal/extract/    规则抽取（6 个检查器）
internal/judge/      rubric + LLM judge（OpenAI 兼容 / Anthropic）
internal/probe/      探针生成与评分
internal/persona/    人物画像（黑盒基线）
internal/drift/      风格漂移（句长 / 语言 / 口癖 / 元话语）
internal/metrics/    聚合
internal/report/     HTML 报告
testdata/            样例数据
```

## 注意

- 抓包里带登录 token，**不要把原始 JSON 提交到 git**（`.gitignore` 已排除 `data/`）
- LLM 判定需要抽样人工复核，judge 的准确率本身也要评估
- 被测对象和 judge 必须是不同模型

## 测试

```bash
go test ./...
```
