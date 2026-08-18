# Symbol: StateMachineMcp

Phase-gated coding workflow as an MCP server. See [README.md](../README.md) for project overview.

## 1. 設計目標

- **MCP 作為 State Store**：`.opencode/state.json` 是唯一的 Canonical State
- **Phase Gate**：工具呼叫層依當前 Phase 阻擋不合規的操作
- **Tree-sitter 原生 Binding**：確定性 AST Skeleton 萃取，0 幻覺
- **Double Buffer + Compiler Verify**：每次 apply_patch 通過語法檢查才寫入硬碟
- **Checkpoint 機制**：Context 清空後可秒接進度

## 2. 命名與定位

| 項目 | 值 |
|------|-----|
| MCP Server Name | `statemachine` |
| 工具命名 | `inspect_context`, `apply_patch`, `get_status`, `checkpoint`（無前綴） |
| 通訊協定 | stdio |
| Go module | `github.com/jimmy/statemachine-mcp` |

---

## 3. Phase Gate 狀態機

```
         ┌──────────┐
         │   INIT   │  ← MCP 啟動，只允許 get_status
         └────┬─────┘
              │ checkpoint(next_phase="PLANNING")
              ▼
         ┌──────────┐
         │ PLANNING │  ← 只允許 inspect_context + checkpoint
         └────┬─────┘
              │ checkpoint(next_phase="EXECUTING")
              ▼
         ┌───────────┐
         │ EXECUTING │  ← 允許 inspect_context + apply_patch + checkpoint
         └────┬──────┘
              │ checkpoint(next_phase="VERIFYING")
              ▼
         ┌───────────┐
         │ VERIFYING │  ← 只允許 inspect_context + get_status（唯讀檢查）
         └────┬──────┘
              │ checkpoint(next_phase="COMPLETED") 或 checkpoint(next_phase="EXECUTING")
              ▼
         ┌───────────┐
         │ COMPLETED │  ← 所有修改工具回傳 Error
         └───────────┘
```

**例外**：連續 3 次 apply_patch 失敗 → 自動降級至 **PAUSED**，通知人工介入。

| 當前 Phase | 允許的工具 |
|-----------|-----------|
| INIT | `get_status` |
| PLANNING | `inspect_context`, `checkpoint`, `get_status` |
| EXECUTING | `inspect_context`, `apply_patch`, `checkpoint`, `get_status` |
| VERIFYING | `inspect_context`, `get_status` |
| PAUSED | `get_status`（需人工 reset） |
| COMPLETED | `get_status` |

---

## 4. `.opencode/state.json` Schema

### 4.1 檔案路徑

```
<CWD>/.opencode/state.json
```

自動建立 `.opencode/` 目錄（若不存在）。此檔案應加入 `.gitignore`。

### 4.2 Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "StateMachineConfig",
  "type": "object",
  "required": ["version", "phase", "active_goal", "checkpoints", "staging_buffer"],
  "properties": {
    "version": { "type": "string", "default": "1.0.0" },
    "phase": {
      "type": "string",
      "enum": ["INIT", "PLANNING", "EXECUTING", "VERIFYING", "PAUSED", "COMPLETED"],
      "description": "當前任務階段。Phase Gate 會根據此欄位阻擋不合規的操作。"
    },
    "active_goal": {
      "type": "string",
      "description": "目前正在執行的高階任務描述。"
    },
    "allowed_actions": {
      "type": "array",
      "items": { "type": "string" },
      "description": "當前 Phase 允許呼叫的工具白名單。"
    },
    "staging_buffer": {
      "type": "object",
      "properties": {
        "has_pending_patch": { "type": "boolean" },
        "target_file": { "type": ["string", "null"] },
        "patch_content": { "type": ["string", "null"] },
        "last_compiler_result": {
          "type": "object",
          "properties": {
            "success": { "type": "boolean" },
            "raw_output": { "type": "string" }
          }
        }
      }
    },
    "checkpoints": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "timestamp", "summary", "modified_files"],
        "properties": {
          "id": { "type": "string" },
          "timestamp": { "type": "string", "format": "date-time" },
          "summary": { "type": "string" },
          "modified_files": {
            "type": "array",
            "items": { "type": "string" }
          }
        }
      }
    }
  }
}
```

### 4.3 狀態範例

```json
{
  "version": "1.0.0",
  "phase": "EXECUTING",
  "active_goal": "Implement tree-sitter Rust skeleton extraction",
  "allowed_actions": ["inspect_context", "apply_patch", "checkpoint"],
  "staging_buffer": {
    "has_pending_patch": false,
    "target_file": null,
    "patch_content": null,
    "last_compiler_result": {
      "success": true,
      "raw_output": "    Checking statemachine-mcp v0.1.0\n    Finished `dev` profile [unoptimized + debuginfo] in 3.42s"
    }
  },
  "checkpoints": [
    {
      "id": "chk_20260818_01",
      "timestamp": "2026-08-18T10:00:00Z",
      "summary": "Project scaffolding + MCP server scaffold",
      "modified_files": ["main.go", "go.mod", "state.go"]
    },
    {
      "id": "chk_20260818_02",
      "timestamp": "2026-08-18T12:00:00Z",
      "summary": "Tree-sitter binding integration + Phase Gate engine",
      "modified_files": ["main.go", "state.go", "phase.go"]
    }
  ]
}
```

---

## 5. 工具介面

### Symbol: inspect_context

安全讀取檔案結構。自動透過 Tree-sitter 萃取 Function/Struct 骨架與行號，極大幅度節省 Token。

**Parameters**:
```json
{
  "name": "inspect_context",
  "description": "安全讀取檔案結構。自動透過 Tree-sitter 萃取 Function/Struct 骨架與行號，極大幅度節省 Token。",
  "parameters": {
    "type": "object",
    "required": ["path"],
    "properties": {
      "path": {
        "type": "string",
        "description": "相對於專案根目錄的檔案路徑。"
      },
      "mode": {
        "type": "string",
        "enum": ["skeleton", "range", "full_cleaned"],
        "default": "skeleton",
        "description": "skeleton: 僅傳回 AST 骨架與行號；range: 精準讀取特定行號區間；full_cleaned: 移除註解與空行後的全文。"
      },
      "line_range": {
        "type": "array",
        "items": { "type": "integer" },
        "minItems": 2,
        "maxItems": 2,
        "description": "當 mode='range' 時填寫，例如 [120, 180]。"
      }
    }
  }
}
```

**Output 範例（skeleton mode）**:
```json
{
  "language": "rust",
  "total_lines": 450,
  "token_reduced_from": 3800,
  "token_reduced_to": 210,
  "content": "Line 1-12: struct StateMachineMcp { ... }\nLine 15: pub fn new() -> Self\nLine 45: pub fn apply_patch(&mut self, patch: &str) -> Result<CompilerOutput>\nLine 120-180: [Use mode='range' with line_range=[120,180] to read function body]"
}
```

**Output 範例（range mode）**:
```json
{
  "language": "rust",
  "total_lines": 450,
  "content": "fn apply_patch(&mut self, patch: &str) -> Result<CompilerOutput> {\n    // ... exact source lines 120-180 ...\n}"
}
```

### Symbol: apply_patch

套用程式碼修改。不直接修改實體檔，而是先進入 Staging Buffer 並自動觸發 Compiler 驗證（`cargo check` / `tsc --noEmit` / `go vet`）。通過後才寫入硬碟並記錄狀態。

**Parameters**:
```json
{
  "name": "apply_patch",
  "description": "套用程式碼修改。會自動通過 Compiler (cargo check / tsc) 驗證語法，通過後才寫入硬碟並記錄狀態。",
  "parameters": {
    "type": "object",
    "required": ["path", "search_block", "replace_block"],
    "properties": {
      "path": {
        "type": "string",
        "description": "要修改的檔案路徑。"
      },
      "search_block": {
        "type": "string",
        "description": "原本檔案中要被替換的精準原始碼片段。"
      },
      "replace_block": {
        "type": "string",
        "description": "準備替換進去的新原始碼片段。"
      },
      "auto_commit_checkpoint": {
        "type": "boolean",
        "default": true,
        "description": "編譯成功後是否自動建立 Checkpoint。"
      }
    }
  }
}
```

**Output 範例（驗證成功）**:
```json
{
  "success": true,
  "phase": "EXECUTING",
  "compiler_status": "PASSED",
  "checkpoint_id": "chk_20260818_03",
  "modified_files": ["src/parser.rs"],
  "compiler_output": "    Checking statemachine-mcp v0.1.0\n    Finished `dev` profile [unoptimized + debuginfo] in 3.42s"
}
```

**Output 範例（驗證失敗）**:
```json
{
  "success": false,
  "phase": "VERIFYING",
  "compiler_status": "FAILED",
  "error_message": "src/main.rs:45:12: error[E0308]: mismatched types\n  expected `bool`, found `String`",
  "hint": "Patch 本體已被攔截於 Staging Buffer，硬碟檔案未變更。請根據 error_message 修正後重新調用 apply_patch。"
}
```

### Symbol: get_status

讓雲端大模型隨時確認當前 Phase、允許的 Action 以及變更歷史。

**Parameters**:（無）

**Output 範例**:
```json
{
  "phase": "EXECUTING",
  "active_goal": "Refactor Tree-sitter Parser Engine",
  "allowed_actions": ["inspect_context", "apply_patch", "checkpoint"],
  "modified_files_in_session": ["src/parser.rs"],
  "last_checkpoint_id": "chk_20260818_01"
}
```

### Symbol: checkpoint

建立進度快照。將目前成功編譯的狀態與摘要寫入 `.opencode/state.json`，建立斷點憑證。

**Parameters**:
```json
{
  "name": "checkpoint",
  "description": "建立進度快照。將目前成功編譯的狀態與摘要寫入 state.json，建立斷點憑證。",
  "parameters": {
    "type": "object",
    "required": ["summary"],
    "properties": {
      "summary": {
        "type": "string",
        "description": "簡短描述此階段完成了什麼（例如：'完成 AST 語法解析器與錯誤處理解析'）。"
      },
      "next_phase": {
        "type": "string",
        "enum": ["PLANNING", "EXECUTING", "VERIFYING", "COMPLETED"],
        "default": "EXECUTING",
        "description": "下一個階段要切換到的 Phase。"
      }
    }
  }
}
```

**Output 範例**:
```json
{
  "success": true,
  "checkpoint_id": "chk_20260818_02",
  "phase": "VERIFYING"
}
```

---

## 6. Tree-sitter Skeleton 萃取策略

### 6.1 支援語言

| 語言 | 副檔名 | go-tree-sitter package | 萃取重點 |
|------|--------|----------------------|---------|
| Rust | `.rs` | `rust` | fn, struct, enum, trait, impl, mod, use, pub(crate) |
| TypeScript | `.ts` | `typescript/typescript` | function, class, interface, type, enum, import, export |
| TSX | `.tsx` | `typescript/tsx` | function, class, interface, type, import, export, component |
| JavaScript | `.js`, `.jsx`, `.mjs` | `javascript` | function, class, import/require, export, const/let/var |
| Python | `.py` | `python` | def, class, import, async def, decorators |
| Go | `.go` | `golang` | func, type, struct, interface, import, method |

### 6.2 Go Interface

```go
type LanguageExtractor interface {
    Skeleton(content []byte) (*SkeletonResult, error)
}

type SkeletonResult struct {
    Language    string        `json:"language"`
    Declarations []Declaration `json:"declarations"`
    Imports     []string      `json:"imports"`
    Stats       SkeletonStats `json:"stats"`
    SkeletonText string       `json:"skeleton_text"`
}

type Declaration struct {
    Kind      string `json:"kind"`      // "fn", "struct", "class", "interface", "type", "enum", "trait"
    Name      string `json:"name"`      // 名稱
    Signature string `json:"signature"`  // 完整簽名（含泛型、參數）
    Line      int    `json:"line"`      // 起始行號
    IsPublic  bool   `json:"is_public"`
}

type SkeletonStats struct {
    Structs    int `json:"structs"`
    Functions  int `json:"functions"`
    Interfaces int `json:"interfaces"`
    Classes    int `json:"classes"`
    Enums      int `json:"enums"`
    Traits     int `json:"traits"`
    Imports    int `json:"imports"`
    TotalLines int `json:"total_lines"`
}
```

### 6.3 Fallback 鏈

```
                   ┌──────────────────────┐
                   │    inspect_context    │
                   └──────────┬───────────┘
                              │
                   ┌──────────▼───────────┐
                   │  語言偵測（副檔名）    │
                   └──────────┬───────────┘
                              │
              ┌───────────────┼────────────────┐
              │ 支援語言       │ 不支援語言       │
              ▼                ▼                │
     ┌──────────────┐  ┌──────────────┐        │
     │ Tree-sitter  │  │ 行數 < 500?  │        │
     │ AST 萃取     │  │              │        │
     │ (ms級, 確定性)│  │ 是 ▼   否 ▼ │        │
     └──────┬───────┘  │ Regex   ┌───┘        │
            │          │ Cleanup │ 行數 > 500  │
            │          │ (註解/  │ 且 > 300 行 │
            │          │  空行)  │ 清理後?     │
            │          └───┬─────┘ 是 ▼  否 ▼ │
            │              │        9B LLM  ──┘
            │              │        摘要
            └──────────────┼──────────────────┘
                           │
                    ┌──────▼──────┐
                    │  回傳結果    │
                    └─────────────┘
```

`full_cleaned` mode：走 Regex Cleanup 分支（無論語言是否支援），移除註解與空行後回傳全文。

---

## 7. Double Buffer + Compiler Verify

### 7.1 流程

```
雲端大模型呼叫 apply_patch
         │
         ▼
  ┌──────────────────────┐
  │ 1. Phase Gate Check  │ ← 非 EXECUTING → 拒絕
  └──────┬───────────────┘
         ▼
  ┌──────────────────────┐
  │ 2. Parse + Verify    │ ← search_block 必須與檔案精準匹配
  │    Search Block      │    不匹配 → Error + 附近 ±5 行上下文
  └──────┬───────────────┘
         ▼
  ┌──────────────────────┐
  │ 3. Staging Copy      │ ← 複製到 /tmp/statemachine-staging/
  └──────┬───────────────┘
         ▼
  ┌──────────────────────┐
  │ 4. Apply Patch       │ ← 對 Staging 副本執行 search→replace
  └──────┬───────────────┘
         ▼
  ┌──────────────────────┐
  │ 5. Compiler Verify   │ ← 依副檔名選擇驗證工具
  └──────┬───────────────┘
         │
    ┌────┴────┬─────────────┬──────────────┐
    │ .rs     │ .ts/.tsx    │ .py          │ .go     │
    ▼         ▼             ▼              ▼
  cargo check  tsc --noEmit  python3 -m     go vet
                            py_compile
    │         │             │              │
    └────┴────┴─────────────┴──────────────┘
         │
    ┌────┴────┐
    │ Pass?   │
    └────┬────┘
         │
    ┌────┴────┐           ┌─────────────────────────┐
    │ 通過    │           │ 不通過                   │
    ▼         │           ▼                         │
  ┌─────────┐ │  ┌──────────────────────┐          │
  │ 寫入     │ │  │ 回傳 Error +         │          │
  │ 原始檔   │ │  │ Compiler Output      │          │
  │ +        │ │  │ + Staging Path       │          │
  │ 狀態更新 │ │  │ 硬碟檔案未變更        │          │
  └────┬────┘ │  └──────────────────────┘          │
       │      │         │                          │
       ▼      │         ▼                          │
  ┌─────────┐ │  ┌──────────────────────┐          │
  │ Check-  │ │  │ 雲端大模型修正後     │          │
  │ point   │ │  │ 重新呼叫 apply_patch │◄─────────┘
  └────┬────┘ │  └──────────────────────┘
       │      │
       ▼      ▼
  ┌──────────────────────┐
  │ 回傳成功結果          │
  └──────────────────────┘
```

### 7.2 驗證工具對照表

| 副檔名 | 驗證指令 | 降級選項 |
|--------|---------|---------|
| `.rs` | `cargo check` | `rustc --edition 2024 --crate-type lib` |
| `.ts` | `tsc --noEmit --strict` | 無 |
| `.tsx` | `tsc --noEmit --strict --jsx preserve` | 無 |
| `.js` / `.jsx` | 無（動態語言） | ESLint（若可用） |
| `.py` | `python3 -m py_compile` | `python3 -c "import ast; ast.parse(...)"` |
| `.go` | `go vet` | `go build -o /dev/null` |

---

## 8. 降級路徑：舊 reducer-mcp 功能

舊的 LLM 壓縮功能整合為 `inspect_context` 的內部降級路徑：

| 情境 | 行為 |
|------|------|
| mode="skeleton" + 支援語言 | Tree-sitter（確定性） |
| mode="skeleton" + 不支援語言 + 檔案 < 500 行 | Regex Cleanup |
| mode="skeleton" + 不支援語言 + 檔案 ≥ 500 行 | Regex Cleanup → 若仍 > 300 行 → 9B LLM 摘要 |
| mode="full_cleaned" | Regex Cleanup（無論語言） |

環境變數沿用：

| 變數 | 預設值 | 說明 |
|------|--------|------|
| `REDUCER_LLM_URL` | `http://localhost:8080/v1` | 9B LLM API 端點（OpenAI compatible） |
| `REDUCER_LLM_MODEL` | `qwen2.5-coder-7b` | 9B 模型名稱 |
| `LITELLM_API_KEY` | （從 auth.json 讀取） | LLM API 金鑰 |
| `STATEMACHINE_STAGING_DIR` | `/tmp/statemachine-staging` | Staging buffer 目錄 |
| `STATEMACHINE_STATE_DIR` | `<CWD>/.opencode` | state.json 存放目錄 |

---

## 9. 錯誤處理原則

| 錯誤類型 | 處理方式 |
|---------|---------|
| Phase Gate 阻擋 | 回傳 Error + 提示當前 Phase 與允許的 actions |
| 檔案不存在 | 回傳 Error + 明確的路徑提示 |
| search_block 不匹配 | 回傳 Error + 該行附近 ±5 行上下文 |
| Tree-sitter parsing 失敗 | 靜態降級到 Regex Cleanup |
| Compiler 不存在 PATH | 回傳 Error + 跳過 verify（warning） |
| 連續 3 次 apply_patch 失敗 | Phase 降級 PAUSED + 通知人工介入 |
| Staging 目錄寫入失敗 | 回傳 Error + 建議檢查磁碟空間 |

---

## 10. 實作順序

| Phase | 內容 | 依賴 | 狀態 |
|-------|------|------|------|
| 1 | 專案 rename + go.mod 更新 + 基本骨架（main.go + MCP server scaffold） | 無 | ✅ 完成 |
| 2 | `get_status` + Phase Gate 引擎（state machine + state.json 讀寫） | Phase 1 | ✅ 完成 |
| 3 | `inspect_context` — 語言偵測 + Regex Cleanup fallback | Phase 2 | ✅ 完成 |
| 4 | `inspect_context` — Tree-sitter Rust extractor | Phase 3 | ⏳ 待實作 |
| 5 | `inspect_context` — Tree-sitter TypeScript/JavaScript/Python/Go extractors | Phase 4 | ⏳ 待實作 |
| 6 | `apply_patch` — Patch parser + Staging Buffer | Phase 2 | ✅ 完成 |
| 7 | `apply_patch` — Compiler Verify（cargo check / tsc / go vet） | Phase 6 | ✅ 完成 |
| 8 | `apply_patch` — 成功 Apply + Checkpoint 自動寫入 | Phase 7 | ✅ 完成 |
| 9 | `checkpoint` — 完整實作（含 next_phase 轉移） | Phase 2 | ✅ 完成 |
| 10 | 舊 reducer-mcp 退役（更新 opencode.json + 刪除舊 binary） | Phase 9 | ⏳ 待實作 |
| 11 | Exponential Backoff + PAUSED 降級機制 | Phase 8 | ⏳ 待實作 |
| 12 | Error handling polish + edge case 測試 | Phase 11 | ⏳ 待實作 |
