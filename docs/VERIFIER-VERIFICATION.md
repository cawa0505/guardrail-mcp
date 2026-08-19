# Verifier HTTP Service — 驗證文件

## 前置條件

- Docker & Docker Compose
- 後端 OpenAI-compatible LLM（已部署 llama.cpp + Qwen3.5-9B-VLM，port 8081）
- `docker-llm-as-a-verifier` 倉庫已 clone

## 1. 部屬與啟動

```bash
cd /path/to/docker-llm-as-a-verifier
cp .env.example .env
# 編輯 .env，填入正確的 OPENAI_BASE_URL
docker compose build
docker compose up -d
```

確認容器啟動：

```bash
docker compose ps
# NAME           IMAGE     COMMAND     SERVICE    STATUS        PORTS
# llm-verifier   ...       "entryp…"   verifier   Up 3 seconds  0.0.0.0:8010->8010/tcp
```

## 2. Health 檢查

```bash
curl http://localhost:8010/health
```

預期回應：

```json
{"status":"ok","model":"qwen3.5-9b","backend":"http://<backend-host>:8081/v1"}
```

`backend` 欄位會顯示實際使用的 `OPENAI_BASE_URL`；若未設定則為 `null`。

## 3. Compare API — 核心驗證

Compare 是 GuardrailMcp 最常呼叫的端點，用於比對原始程式碼與修改後程式碼的品質。

### 3.1 預設標準

| 欄位 | 說明 |
|---|---|
| `score_a` | `trace_a`（原始碼）的 LLM 評分，值域 `[0, 1]` |
| `score_b` | `trace_b`（修改後）的 LLM 評分，值域 `[0, 1]` |
| `accepted` | `score_b >= VERIFIER_MIN_SCORE`（預設 `0.8`） |
| `model` | 實際使用的 model alias |

### 3.2 已知正確值的測試 pair

使用以下一組 clearly-good 與 clearly-bad 的 pair 確認評分方向正確：

```bash
curl -s -X POST http://localhost:8010/v1/compare \
  -H "Content-Type: application/json" \
  -d '{
    "problem": "Write a function that returns 42.",
    "trace_a": "def foo():\n    return 42",
    "trace_b": "def foo():\n    return 0",
    "criteria": [{"id": "correctness", "name": "Correctness", "description": "Does the code solve the task?"}]
  }'
```

預期：

```json
{"score_a":0.999,"score_b":0.007,"accepted":false,"model":"qwen3.5-9b"}
```

- `score_a`（正確答案）應接近 `1.0`
- `score_b`（錯誤答案）應接近 `0.0`
- `accepted` 為 `false`（因為 `score_b < 0.8`）

### 3.3 角色對調測試

交換 `trace_a` 與 `trace_b`，確認評分跟隨內容而非位置：

```bash
curl -s -X POST http://localhost:8010/v1/compare \
  -H "Content-Type: application/json" \
  -d '{
    "problem": "Write a function that returns 42.",
    "trace_a": "def foo():\n    return 0",
    "trace_b": "def foo():\n    return 42",
    "criteria": [{"id": "correctness", "name": "Correctness", "description": "Does the code solve the task?"}]
  }'
```

預期：

```json
{"score_a":0.007,"score_b":0.999,"accepted":true,"model":"qwen3.5-9b"}
```

`accepted` 為 `true`，因為 `score_b >= 0.8`。

## 4. Select API

Select 用於從多個候選中選出最佳解答：

```bash
curl -s -X POST http://localhost:8010/v1/select \
  -H "Content-Type: application/json" \
  -d '{
    "problem": "Write a function that returns 42.",
    "candidates": ["def foo():\n    return 42", "def foo():\n    return 0"],
    "criteria": [{"id": "correctness", "name": "Correctness", "description": "Does the code solve the task?"}]
  }'
```

預期：

```json
{"index":0,"scores":[0.65,0.35],"n_comparisons":3,"model":"qwen3.5-9b"}
```

- `index: 0` — 第一個候選（正確答案）被選為最佳
- `scores[0] > scores[1]` — 正確答案分數較高
- `n_comparisons` — 執行次數（可能因 pivots 設定而不同）

## 5. GuardrailMcp 整合流程

`mcp_apply_patch` 在 Compiler 驗證通過後、實際寫入檔案前，呼叫 verifier：

```
Staging → Compiler check → POST /v1/compare → score_b ≥ 0.8? → Apply / Reject
```

### 5.1 成功案例

```bash
curl -s -X POST http://localhost:8010/v1/compare \
  -H "Content-Type: application/json" \
  -d '{
    "problem": "Fix the login timeout bug by adding error handling",
    "trace_a": "function login(user) {\n  return api.post(\"/login\", user);\n}",
    "trace_b": "function login(user) {\n  try {\n    return api.post(\"/login\", user);\n  } catch (e) {\n    logger.error(e);\n    throw e;\n  }\n}",
    "criteria": [{"id": "correctness", "name": "Correctness", "description": "Does the code correctly solve the task?"}]
  }'
```

預期 `accepted: true`（修改後程式碼有錯誤處理，品質較佳）。

### 5.2 拒絕案例

```bash
curl -s -X POST http://localhost:8010/v1/compare \
  -H "Content-Type: application/json" \
  -d '{
    "problem": "Fix the login timeout bug by adding error handling",
    "trace_a": "function login(user) {\n  return api.post(\"/login\", user);\n}",
    "trace_b": "function login(user) {\n  return null;\n}",
    "criteria": [{"id": "correctness", "name": "Correctness", "description": "Does the code correctly solve the task?"}]
  }'
```

預期 `accepted: false`（修改後破壞了功能）。

## 6. 邊界情況與錯誤處理

| 情況 | 預期行為 |
|---|---|
| `OPENAI_BASE_URL` 未設定 | 容器啟動時 exit 1，提示設定 |
| 後端無法連線 | `POST /v1/compare` 回傳 `502` 含錯誤訊息 |
| `trace_a` / `trace_b` 為空字串 | 仍可評分，但分數可能偏低 |
| `criteria` 為空列表 | 使用預設 `correctness` criterion |
| 後端回傳不含 logprobs | 評分可能不準確，需確認 backend 支援 logprobs |

## 7. 環境變數一覽

| 變數 | 預設值 | 說明 |
|---|---|---|
| `OPENAI_BASE_URL` | — | 後端 API endpoint（必要） |
| `MODEL_ALIAS` | `qwen3.5-9b` | Model 名稱 |
| `OPENAI_API_KEY` | `EMPTY` | API key |
| `VERIFIER_PORT` | `8010` | HTTP 服務埠號 |
| `VERIFIER_MIN_SCORE` | `0.8` | `accepted` 門檻值 |

## 8. 快速驗證腳本

```bash
#!/bin/bash
# 快速驗證 Verifier 服務是否正常
set -e

HOST=${1:-localhost:8010}

echo "=== Health check ==="
curl -s "$HOST/health" | python3 -m json.tool

echo ""
echo "=== Compare: good vs bad ==="
curl -s -X POST "$HOST/v1/compare" \
  -H "Content-Type: application/json" \
  -d '{"problem":"Return 42","trace_a":"def f(): return 42","trace_b":"def f(): return 0","criteria":[{"id":"x","name":"x","description":"x"}]}' | python3 -m json.tool

echo ""
echo "=== Select: good vs bad ==="
curl -s -X POST "$HOST/v1/select" \
  -H "Content-Type: application/json" \
  -d '{"problem":"Return 42","candidates":["def f(): return 42","def f(): return 0"],"criteria":[{"id":"x","name":"x","description":"x"}]}' | python3 -m json.tool

echo ""
echo "=== PASS ==="
```