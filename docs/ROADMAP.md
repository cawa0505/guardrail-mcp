# ROADMAP

> 實際開發排程，基於當前程式碼狀態（4 Go source, ~1000 行）。

## 已完成

| # | 任務 | 檔案 | 說明 |
|---|------|------|------|
| P1 | MCP server scaffold | `main.go` | 4 工具註冊，stdio transport |
| P2 | state.json 讀寫 | `state.go` | JSON schema, load/save, 自動 .gitignore |
| P3 | Phase Gate | `main.go` | 6 Phase, 轉移規則, action whitelist |
| P4 | inspect_context | `inspect.go`, `main.go` | 三模式（skeleton/range/full_cleaned） |
| P5 | apply_patch | `apply.go`, `main.go` | search/replace + compiler verify + rollback |
| P6 | checkpoint | `main.go` | phase transition + 進度記錄 |
| P7 | graphify background extract | `apply.go` | 背景 goroutine + ast_synced flag |

---

## 待實作

> Tree-sitter AST 萃取已由 Graphify MCP 負責（`graphify_skeleton_extract`），StateMachineMcp 不需自行綁定。

### Phase B — Staging Buffer 隔離目錄

**目標**：apply_patch 不在原始檔案上直接編輯，先複製到隔離目錄驗證。

| Task | 說明 | 檔案 | 依賴 | 預估 |
|------|------|------|------|------|
| B1 | 建立 staging transaction 目錄 | `staging.go`（新） | 無 | 2h |
| B2 | staging 副本 apply + compiler verify | `apply.go` | B1 | 2h |
| B3 | staging 通過後 atomic 寫入原始檔 | `apply.go` | B2 | 1h |

### Phase C — mcp_* 工具重新命名

**目標**：SPEC 定義正式名稱，提供舊名稱相容層。

| Task | 說明 | 檔案 | 依賴 | 預估 |
|------|------|------|------|------|
| C1 | 新增 mcp_read_range / mcp_apply_patch 等工具 | `main.go` | 無 | 1h |
| C2 | 舊名稱保留為 alias | `main.go` | C1 | 0.5h |

### Phase D — LLM Verifier（第二道關卡）

**目標**：compiler 通過後再經 LLM 審查正確性。

| Task | 說明 | 檔案 | 依賴 | 預估 |
|------|------|------|------|------|
| D1 | Verifier client（OpenAI compatible） | `verifier.go`（新） | 無 | 3h |
| D2 | Verifier contract + score threshold | `apply.go` | D1 | 2h |
| D3 | fail-closed 處理 + timeout | `verifier.go` | D1 | 1h |

### Phase E — Error Handling + 穩定性

| Task | 說明 | 檔案 | 依賴 | 預估 |
|------|------|------|------|------|
| E1 | PAUSED phase + 連續 3 次失敗降級 | `state.go`, `main.go` | 無 | 2h |
| E2 | search_block 不匹配時回傳附近上下文 | `apply.go` | 無 | 1h |
| E3 | 統一錯誤格式（stage_failed 欄位） | `main.go` | 無 | 1h |
| E4 | edge case 測試 | `*_test.go` | 所有 Phase | 3h |

---

## 優先級建議

```
B1-B3 → E1 → E2-E3 → E4
```

- **B** 優先（staging buffer 是安全關鍵 — 不在原地編輯）
- **E** 基本錯誤處理穿插其中
- **C/D** 視需求決定（命名可延後，LLM Verifier 需要 infra 配合）
