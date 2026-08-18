# Symbol: StateMachineMcp

Phase-gated coding workflow as an MCP server.

## 1. 設計目標

- **Phase Gate**：工具呼叫層依當前 Phase 阻擋不合規的操作
- **Compiler Verify**：每次 apply_patch 通過語法檢查才寫入硬碟，失敗自動復原
- **Token 節省**：inspect_context 透過 regex 骨架萃取減少 LLM context 消耗；深度 AST 分析委託 Graphify MCP
- **Checkpoint 機制**：進度寫入 `.opencode/state.json`，支援斷點續接
- **Graphify 整合**：apply_patch 成功後背景觸發 graphify extract，get_status 可查詢 ast_synced 狀態

## 2. 定位

| 項目 | 值 |
|------|-----|
| MCP Server Name | `statemachine` |
| 工具命名 | `inspect_context`, `apply_patch`, `get_status`, `checkpoint` |
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
         │ PLANNING │  ← 只允許 inspect_context + checkpoint + get_status
         └────┬─────┘
              │ checkpoint(next_phase="EXECUTING")
              ▼
         ┌───────────┐
         │ EXECUTING │  ← 允許 inspect_context + apply_patch + checkpoint + get_status
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

| 當前 Phase | 允許的工具 |
|-----------|-----------|
| INIT | `get_status` |
| PLANNING | `inspect_context`, `checkpoint`, `get_status` |
| EXECUTING | `inspect_context`, `apply_patch`, `checkpoint`, `get_status` |
| VERIFYING | `inspect_context`, `get_status` |
| COMPLETED | `get_status` |

Phase 轉移規則（實作於 `main.go:307-314`）：

| 當前 Phase | 允許轉移至 |
|-----------|-----------|
| INIT | PLANNING |
| PLANNING | EXECUTING, PLANNING |
| EXECUTING | VERIFYING, EXECUTING |
| VERIFYING | COMPLETED, EXECUTING |
| COMPLETED | （無） |

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
      "enum": ["INIT", "PLANNING", "EXECUTING", "VERIFYING", "COMPLETED"],
      "description": "當前任務階段。Phase Gate 根據此欄位阻擋不合規的操作。"
    },
    "active_goal": {
      "type": "string",
      "description": "目前正在執行的高階任務描述。"
    },
    "allowed_actions": {
      "type": "array",
      "items": { "type": "string" },
      "description": "當前 Phase 允許呼叫的工具白名單。由 loadState() 自動同步。"
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
    },
    "ast_synced": {
      "type": "boolean",
      "description": "Graphify AST 是否與當前程式碼同步。apply_patch 成功後設為 false，graphify extract 完成後設為 true。"
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
    }
  ],
  "ast_synced": true
}
```

---

## 5. 工具介面

### Symbol: inspect_context

安全讀取檔案結構。支援 skeleton / range / full_cleaned 三種模式。深度 AST 分析請使用 Graphify MCP 的 `graphify_skeleton_extract`。

**Parameters**（實作於 `main.go:20-43`）:
```json
{
  "name": "inspect_context",
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
        "description": "skeleton: regex 萃取骨架（最多 100 行）；range: 精準讀取行號區間；full_cleaned: 移除註解與空行後的全文。"
      },
      "line_range": {
        "type": "array",
        "items": { "type": "integer" },
        "minItems": 2,
        "maxItems": 2,
        "description": "mode='range' 時填寫，例如 [120, 180]。"
      }
    }
  }
}
```

**內部架構**（實作於 `inspect.go`）:
- 語言偵測：依副檔名（`.rs`→rust, `.ts`→typescript, `.go`→go, `.py`→python 等）
- skeleton mode：`reduceText()` 移除註解/空行/重複行 → 取前 100 行；另以 `skeletonByRegex()` 依語言萃取宣告式骨架
- range mode：`readLines()` 精準讀取行號區間
- full_cleaned mode：`reduceText()` 全文清理（80KB 截斷保護）
- 非文字檔偵測：檢查 null byte + UTF-8 有效性

**Output 範例（skeleton mode）**:
```json
{
  "language": "rust",
  "total_lines": 450,
  "token_reduced_from": 3800,
  "token_reduced_to": 210,
  "content": "Line 1-12: struct StateMachineMcp { ... }\nLine 15: pub fn new() -> Self\n...",
  "truncated": true
}
```

### Symbol: apply_patch

套用程式碼修改。通過 compiler 驗證後才寫入硬碟，失敗自動復原。

**Parameters**（實作於 `main.go:46-71`）:
```json
{
  "name": "apply_patch",
  "parameters": {
    "type": "object",
    "required": ["path", "search_block", "replace_block"],
    "properties": {
      "path": { "type": "string", "description": "要修改的檔案路徑。" },
      "search_block": { "type": "string", "description": "原本檔案中要被替換的精準原始碼片段。" },
      "replace_block": { "type": "string", "description": "準備替換進去的新原始碼片段。" },
      "auto_commit_checkpoint": { "type": "boolean", "default": true }
    }
  }
}
```

**Lifecycle**（實作於 `main.go:151-256`）:
1. Phase Gate：僅 EXECUTING 可執行
2. 讀取原始檔案
3. `applyPatchContent()`：精準 search→replace（`strings.Index` 匹配）
4. 先寫入變更內容到硬碟
5. `findProjectRoot()` → `detectCompiler()` → `runCompiler()` 驗證
6. Compiler 通過 → 保留變更 + 更新 state + 背景觸發 graphify
7. Compiler 失敗 → 復原原始內容 + 回傳 Error

**Compiler 偵測**（實作於 `apply.go:70-103`）:
- `Cargo.toml` → `cargo check`
- `tsconfig.json` → `tsc --noEmit`
- `go.mod` → `go build ./...`
- `package.json` + `tsconfig.json` → `tsc --noEmit`

**Output 範例（成功）**:
```json
{
  "success": true,
  "file": "src/parser.rs",
  "compiler": "cargo",
  "compiler_output": "    Checking statemachine-mcp v0.1.0\n    Finished ...",
  "ast_synced": false,
  "graphify_extract_triggered": true
}
```

**Output 範例（失敗）**:
```json
{
  "isError": true,
  "content": [{ "text": "apply_patch: compiler validation failed:\nerror[E0308]: mismatched types..." }]
}
```

### Symbol: get_status

查詢當前工作流狀態、Phase 階段、Checkpoint 資訊。

**Parameters**: 無

**Output**（實作於 `main.go:269-282`）:
```json
{
  "phase": "EXECUTING",
  "active_goal": "Refactor Tree-sitter Parser Engine",
  "allowed_actions": ["inspect_context", "apply_patch", "checkpoint"],
  "modified_files_in_session": ["src/parser.rs"],
  "last_checkpoint_id": "chk_20260818_01",
  "ast_synced": true
}
```

### Symbol: checkpoint

建立進度快照，轉移 Phase 階段。

**Parameters**（實作於 `main.go:82-99`）:
```json
{
  "name": "checkpoint",
  "parameters": {
    "type": "object",
    "required": ["summary"],
    "properties": {
      "summary": { "type": "string", "description": "簡短描述此階段完成了什麼。" },
      "next_phase": {
        "type": "string",
        "enum": ["PLANNING", "EXECUTING", "VERIFYING", "COMPLETED"],
        "description": "下一個階段。省略則停留在當前 Phase。"
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

## 6. 專案根目錄偵測

實作於 `apply.go:30-60`。從檔案所在目錄向上搜尋專案標記檔：

```
projectMarkers = [
  "Cargo.toml", "tsconfig.json", "go.mod",
  "package.json", "pyproject.toml", "setup.py",
  "CMakeLists.txt", "Makefile"
]
```

用於 compiler 偵測與 graphify extract 路徑。

---

## 7. Graphify 整合

實作於 `apply.go:137-171`。背景 goroutine 執行 `graphify extract <projectRoot>`：
- apply_patch 成功後觸發
- 完成後更新 `state.json` 的 `ast_synced: true`
- 失敗時保持 `ast_synced: false`（下次 get_status 可發現）

---

## 8. 錯誤處理

| 錯誤類型 | 處理方式 |
|---------|---------|
| Phase Gate 阻擋 | 回傳 Error + 提示當前 Phase 與允許的 actions |
| 檔案不存在 | 回傳 Error |
| search_block 不匹配 | 回傳 Error（`strings.Index` 未找到） |
| Compiler 不存在 PATH | `runCompiler` 回傳 Error，patch 復原 |
| 非文字檔案 | inspect_context 回傳 Error |
| `.gitignore` 寫入失敗 | loadState 時 warning，不阻擋 |

---

## 9. `.gitignore` 自動管理

實作於 `state.go:84-121`。`stateDir()` 建立 `.opencode/` 時自動：
- `.gitignore` 不存在 → 建立包含 `.opencode/` 的檔案
- 已存在但無 `.opencode/` → 追加
- 已存在且有 → 跳過
