---
base_model: MemTensor/MemOperator-0.6B
library_name: peft
pipeline_tag: text-classification
license: apache-2.0
language:
- zh
- en
tags:
- lora
- memory
- text-classification
datasets:
- chris0809/memory-write-gate-20k
metrics:
- accuracy
- roc_auc
---

# MemOperator 0.6B Memory Write Gate

This PEFT LoRA adapter turns `MemTensor/MemOperator-0.6B` into a bilingual `SKIP`/`SAVE` sequence classifier for long-term-memory admission.

The model is intentionally tuned as a conservative gate: false saves are preferred over false skips. Use the threshold in `classifier_metadata.json`, rather than an implicit 0.5 threshold.

## Evaluation

| Evaluation set | Accuracy | ROC-AUC | SAVE precision | SAVE recall |
|---|---:|---:|---:|---:|
| Held-out synthetic seed families (2,600 rows) | 89.73% | 96.89% | 86.66% | 93.92% |
| Small human-authored diagnostic set (60 rows) | 85.00% | 97.78% | 76.92% | 100.00% |

The 60-row diagnostic set is a small author-curated check, not a production benchmark. The synthetic test split isolates seed families but remains generator-domain data.

## Usage

```python
import json
import torch
from huggingface_hub import hf_hub_download
from peft import AutoPeftModelForSequenceClassification
from transformers import AutoTokenizer

model_id = "chris0809/memoperator-0.6b-memory-write-gate"
tokenizer = AutoTokenizer.from_pretrained(model_id)
if tokenizer.pad_token_id is None:
    tokenizer.pad_token = tokenizer.eos_token
model = AutoPeftModelForSequenceClassification.from_pretrained(model_id)
model.config.pad_token_id = tokenizer.pad_token_id
metadata = json.loads(open(hf_hub_download(model_id, "classifier_metadata.json"), encoding="utf-8").read())

inputs = tokenizer("以后给我写周报时先写结论。", return_tensors="pt")
with torch.inference_mode():
    save_probability = torch.softmax(model(**inputs).logits, dim=-1)[0, 1].item()
decision = "SAVE" if save_probability >= metadata["threshold"] else "SKIP"
```

Evaluate the gate on independently labeled traffic before using it for persistent memory. Do not store secrets merely because the classifier returns `SAVE`; deterministic privacy rules should run before this model.
