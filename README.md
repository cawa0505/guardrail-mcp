# StateMachineMcp

**Phase-gated coding workflow as an MCP server.**

StateMachineMcp 是一個 MCP (Model Context Protocol) server，提供一套狀態機驅動的編碼工作流程工具。每個操作都經過 Phase Gate 檢查、雙緩衝編譯驗證，並支援 Checkpoint 斷點續接，讓 LLM agent 的程式修改流程更可控、可回復。

## 專案目標

- **Phase Gate 流程控管** — 定義 INIT → PLANNING → EXECUTING → VERIFYING → COMPLETED 的階段轉換，每個階段只開放對應的工具
- **雙緩衝 + 編譯驗證** — `apply_patch` 先通過 compiler（`cargo check` / `go vet` / `tsc`）才寫入硬碟，失敗自動復原
- **Token 節省** — `inspect_context` 透過 regex 降級鏈萃取 AST 骨架，大幅減少 LLM context 消耗
- **Checkpoint 機制** — 進度寫入 `.opencode/state.json`，支援斷點續接
- **Graphify 整合** — 背景自動觸發 graphify extract，保持 AST 知識圖譜同步

## 目前進度

| Phase | 內容 | 狀態 |
|-------|------|------|
| 1 | 專案 scaffold + MCP server 骨架 | ✅ 完成 |
| 2 | `get_status` + Phase Gate 引擎 + state.json 讀寫 | ✅ 完成 |
| 3 | `inspect_context` — 語言偵測 + Regex Cleanup fallback | ✅ 完成 |
| 6 | `apply_patch` — Patch parser + Compiler Verify | ✅ 完成 |
| 7 | Compiler Verify（cargo / go vet / tsc） | ✅ 完成 |
| 8 | 成功 Apply + graphify 背景觸發 | ✅ 完成 |
| 9 | `checkpoint` — 完整實作含 next_phase 轉移 | ✅ 完成 |

### 待實作

| Phase | 內容 |
|-------|------|
| 4-5 | Tree-sitter 原生 binding（Rust / TypeScript / Python / Go extractors） |
| 7 | Staging Buffer 隔離目錄（`/tmp/statemachine-staging/`） |
| 11 | Exponential Backoff + PAUSED 降級機制 |
| 12 | Error handling polish + edge case 測試 |

## 架構

```
┌─────────────────────┐
│   MCP Server        │
│   (stdio transport) │
└──────┬──────────────┘
       │
       ▼
┌─────────────────────┐
│   Phase Gate        │ ← 狀態機：INIT→PLANNING→EXECUTING→VERIFYING→COMPLETED
└──────┬──────────────┘
       │
       ├── inspect_context  → 語言偵測 → Regex skeleton / Tree-sitter
       ├── apply_patch      → search/replace → Compiler Verify → 寫入 + Checkpoint
       ├── get_status       → 當前 Phase + 修改歷史
       └── checkpoint       → state.json + Phase 轉移
```

## 快速開始

```bash
# Build
go build -o ~/.local/bin/statemachine-mcp .
```

## 工具列表

| 工具 | 說明 |
|------|------|
| `inspect_context` | 安全讀取檔案結構，支援 skeleton / range / full_cleaned 模式 |
| `apply_patch` | search/replace 套用修改，通過 compiler 驗證後寫入硬碟 |
| `get_status` | 查詢當前 Phase、allowed actions、checkpoint 資訊 |
| `checkpoint` | 建立進度快照，轉移 Phase 階段 |

## License

MIT
