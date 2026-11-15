---
base_model: LiquidAI/d1-3B
language:
- en
license: other
license_name: lfm-open-license-v1.0
license_link: LICENSE
tags:
- memory-gate
- decision-model
- lfm2
- hindsight
- agent-memory
- memory
datasets:
- WasamiKirua/english-memory-gate
---
# Liquid d1 3B — English Memory Gate

A local decision model designed as an upstream filter for Hindsight and similar
conversational memory systems. It returns **IGNORE** for clearly disposable
interaction and **CONSIDER** for personal information, unresolved references or
memory-related requests. CONSIDER means assessment, not automatic storage or
permission to execute a command.

## Validation

On 57 English test examples from the 296-example curated/synthetic dataset, the
selected training adapter scored **56/57 (98.2%)** and its F16 llama.cpp export
scored **57/57 (100%)**. One prediction differed between runtimes. These consumed
experimental test results are not independent real-conversation validation or a
safety guarantee. End-to-end Hindsight integration has not been validated.

## License

LFM Open License v1.0 applies; see LICENSE, including the $10 million annual
revenue commercial-use threshold. This is not an Apache or MIT license. The base
LiquidAI/d1-3B model was modified through English memory-gate LoRA fine-tuning.

## This repository: complete merged model

The base and trained adapter are merged into the safetensors checkpoint.
**No separate adapter is required or should be applied.** All adapter layers were
removed, the native D1 loader/configuration are included, and saved weights were
reloaded with exact tensor equality. This verifies serialization, not a separate
accuracy benchmark for this merged-HF runtime.

For llama.cpp deployment, use
[the separate F16 GGUF repository](https://huggingface.co/WasamiKirua/liquid-d1-3b-memory-gate-GGUF).
The final dataset is [English Memory Gate](https://huggingface.co/datasets/WasamiKirua/english-memory-gate).