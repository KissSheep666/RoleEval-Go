---
base_model: Qwen/Qwen3-1.7B
pipeline_tag: text-generation
tags:
  - qwen3
  - lora
  - sft
  - grpo
  - memory
  - agent
  - retrieval
  - gguf
---

# KnowMe Memory Gate

面向个人 Agent 长期记忆检索的轻量级本地模型。

本模型基于 **Qwen3-1.7B**，经过 **LoRA SFT + GRPO** 后训练，用于完成两个任务：

1. 判断当前用户输入是否需要检索长期记忆；
2. 在需要检索时，生成适合 SQLite FTS5 / BM25 的高信号检索 query。

对应项目：

- KnowMe: https://github.com/Longlong418/KnowMe
- Post-training: https://github.com/Longlong418/KnowMe-memory_gate_model_post_train

---

## 模型信息

| 项目 | 内容 |
|---|---|
| Base Model | Qwen3-1.7B |
| Training | LoRA SFT + GRPO |
| Task | Memory Retrieval Gate + Query Generation |
| Output | JSON |
| Retrieval Backend | SQLite FTS5 / BM25 |
| Deployment | Ollama / llama.cpp / Transformers |

模型输出格式：

```json
{
  "retrieve": true,
  "query": "新服装品牌 首次购物 体验",
  "reason": "需要查看相关历史信息"
}
```

字段说明：

- `retrieve`：是否需要读取长期记忆；
- `query`：真正发送给检索器的关键词；
- `reason`：给用户展示的简短原因。

---

## 评测结果

### Standard Test

| Model | Macro F1 ↑ | Precision ↑ | Recall ↑ | JSON Valid ↑ | Format Valid ↑ |
|---|---:|---:|---:|---:|---:|
| Qwen3-1.7B (Base) | 0.4960 | 0.5263 | 0.8451 | 1.0000 | 0.8134 |
| Qwen3-1.7B + SFT | 0.9577 | 0.9452 | 0.9718 | 1.0000 | 0.9930 |
| **Qwen3-1.7B + SFT + GRPO** | **0.9613** | **0.9456** | **0.9789** | **1.0000** | **1.0000** |
| DeepSeek V4.1 Flash | 0.7746 | 0.7786 | 0.7676 | 1.0000 | 0.9824 |
| MiMo-V2.6-Flash | 0.7676 | 0.7676 | 0.7676 | 0.9965 | 0.9648 |

### Distractor-Augmented Retrieval

在候选记忆池中加入额外干扰记忆，测试 query 的检索和排序能力。

| Model | Hit Rate ↑ | Hit@1 ↑ | Hit@3 ↑ | Hit@5 ↑ | MRR ↑ | Conditional MRR ↑ |
|---|---:|---:|---:|---:|---:|---:|
| Qwen3-1.7B (Base) | 0.4366 | 0.3380 | 0.4366 | 0.4366 | 0.3826 | 0.5906 |
| Qwen3-1.7B + SFT | 0.8310 | 0.6620 | 0.8239 | 0.8310 | 0.7330 | 0.7597 |
| **Qwen3-1.7B + SFT + GRPO** | **0.8873** | **0.6761** | **0.8592** | **0.8873** | **0.7582** | **0.7746** |
| DeepSeek V4.1 Flash | 0.6690 | 0.4859 | 0.6690 | 0.6690 | 0.5669 | 0.7594 |
| MiMo-V2.6-Flash | 0.6268 | 0.5141 | 0.6268 | 0.6268 | 0.5634 | 0.7692 |

Checkpoint 仅使用 dev set 选择，test set 只用于最终结果汇报。

---

## 输入格式

模型训练时使用单个 `user` message，不使用独立的 `system` role。

推荐输入：

```text
You are a retrieval gate for a personal assistant's long-term memory.
Given the user's current message, decide whether answering well requires the user's stored long-term memory.
Long-term memory may contain facts, preferences, past events, prior conversations, decisions, plans, ongoing work, or personal constraints.

Reply with ONLY this JSON, nothing else:
{"retrieve": true/false, "query": "<2-6 high-signal search keywords if true, else empty>", "reason": "<一句中文，给用户看，不超过15字>"}

Rules:
- General knowledge, math, coding, small talk, translation, rewriting, and other self-contained requests usually do not need memory.
- Retrieve when important information needed to answer is missing from the current message but may exist in long-term memory.
- Retrieve when relevant stored preferences, constraints, previous decisions, ongoing projects, or personal history would materially improve the answer or prevent a conflicting answer.
- Do NOT retrieve merely because the message mentions the user, their life, another person, a project, or the past.
- If the current message already provides all personal information needed to answer well, do NOT retrieve.
- When retrieve=true, query must contain 2-6 concise, high-signal keywords derived ONLY from the current message. Do not invent hidden facts or answers.
- For Chinese queries, separate important search terms with ASCII spaces.
- When retrieve=false, query must be an empty string.
- Never return an array.

User message: {当前用户输入}
```

---

## Chat Template

部署时需要保持与训练一致的 Qwen3 chat template。

训练时实际输入结构为：

```text
<|im_start|>user
{完整 Gate Prompt}<|im_end|>
<|im_start|>assistant
<think>

</think>
```

对于 GGUF / Ollama，推荐使用：

```text
{{- range .Messages }}
<|im_start|>{{ .Role }}
{{ .Content }}<|im_end|>
{{- end }}
<|im_start|>assistant
<think>

</think>
```

> 不建议使用裸 `{{ .Prompt }}` 模板，否则推理分布会与训练阶段不一致。

---

## Ollama

如果使用 GGUF 版本，在 GGUF 文件同目录创建 `Modelfile`：

````text
FROM ./knowme-memory-gate-model-grpo-f16.gguf

PARAMETER temperature 0
PARAMETER num_predict 128
PARAMETER stop "<|im_end|>"

TEMPLATE """{{- range .Messages }}
<|im_start|>{{ .Role }}
{{ .Content }}<|im_end|>
{{- end }}
<|im_start|>assistant
<think>

</think>

"""
````

创建模型：

```bash
ollama create knowme-memory-gate -f Modelfile
```

运行：

```bash
ollama run knowme-memory-gate
```

---

## OpenAI-compatible API

Ollama 默认提供本地 API：

```text
http://127.0.0.1:11434/v1
```

示例：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:11434/v1",
    api_key="ollama",
)

response = client.chat.completions.create(
    model="knowme-memory-gate",
    messages=[
        {
            "role": "user",
            "content": gate_prompt,
        }
    ],
    temperature=0,
    max_tokens=128,
)

print(response.choices[0].message.content)
```

其中 `gate_prompt` 应为上文完整 Gate Prompt 与当前用户消息拼接后的文本。

---

## 与 KnowMe 的关系

本模型是 [KnowMe](https://github.com/Longlong418/KnowMe) 长期记忆模块中的 Memory Gate。

调用链：

```text
User Message
    ↓
Memory Gate
    ↓
retrieve=false ──→ Skip memory retrieval
    ↓
retrieve=true
    ↓
Generate Query
    ↓
SQLite FTS5 / BM25
    ↓
Retrieve Facts / Episodes
    ↓
Agent Context
```

模型只负责：

```text
是否需要检索
      +
生成什么 Query
```

真正的记忆存储和检索由 KnowMe 完成。

---

## 数据

训练与评测数据由公开 memory benchmark 构造，包括：

- LoCoMo
- LongMemEval
- PersonaMem-v2
- RHELM

训练仓库与真实用户数据完全隔离，不读取 KnowMe 的 `.knowme/state.db`，也不会复制真实用户记忆用于训练或评测。

---

## 说明

- 本模型仍保留 Qwen3-1.7B 的基础语言能力，但推荐只作为 Memory Gate 使用。
- 部署时建议保持训练阶段的 Prompt、Chat Template 和 `thinking=false` 设置。
- GGUF F16 适合高精度本地测试；低显存设备可进一步量化为 `Q4_K_M`。
