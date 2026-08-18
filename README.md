# StateMachineMcp

**Phase-gated coding workflow as an MCP server.**

StateMachineMcp 是一個 MCP (Model Context Protocol) server，提供一套狀態機驅動的編碼工作流程工具。每個操作都經過 Phase Gate 檢查、雙緩衝編譯驗證，並支援 Checkpoint 斷點續接，讓 LLM agent 的程式修改流程更可控、可回復。

## 專案目標

- **Phase Gate 流程控管** — 定義 INIT → PLANNING → EXECUTING → VERIFYING → COMPLETED 的階段轉換，每個階段只開放對應的工具
- **雙緩衝 + 編譯驗證** — `apply_patch` 先通過 compiler（`cargo check` / `go vet` / `tsc`）才寫入硬碟，失敗自動復原
- **Token 節省** — `inspect_context` 透過 regex 骨架萃取減少 LLM context 消耗；深度 AST 分析委託 [Graphify MCP](https://github.com/cawa0505/graphify-mcp)
- **Checkpoint 機制** — 進度寫入 `.opencode/state.json`，支援斷點續接
- **Graphify 整合** — 背景自動觸發 graphify extract，`get_status` 可查詢 `ast_synced` 狀態

## 專案文件

規格與開發計畫使用 [openspec](https://github.com/cawa0505/openspec) 格式管理：

| 文件 | 路徑 |
|------|------|
| 規格 | `openspec/specs/` |
| 變更提案 | `openspec/changes/init-scaffold/proposal.md` |
| 設計 | `openspec/changes/init-scaffold/design.md` |
| 任務 | `openspec/changes/init-scaffold/tasks.md` |

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
       ├── inspect_context  → 語言偵測 → Regex skeleton / full_cleaned
       ├── apply_patch      → search/replace → Compiler Verify → 寫入 + Checkpoint
       ├── get_status       → 當前 Phase + 修改歷史
       └── checkpoint       → state.json + Phase 轉移
```

## 目前進度

`init-scaffold` 變更已完成：Phase Gate、inspect_context、apply_patch（含 compiler verify）、checkpoint、get_status、graphify 背景觸發、openspec 文件。

### 待實作

| 項目 | 說明 |
|------|------|
| Staging Buffer | 隔離目錄，不在原地編輯 |
| PAUSED 降級 | 連續 3 次失敗自動暫停 |
| Error handling 改進 | 統一錯誤格式，search_block 上下文提示 |
| edge case 測試 | 測試覆蓋

## 快速開始

```bash
# Build
go build -o ~/.local/bin/statemachine-mcp .
```

### OpenCode 整合

在 `opencode.json` 的 `mcp` 區塊加入：

```json
"statemachine": {
  "command": ["{env:HOME}/.local/bin/statemachine-mcp"],
  "description": "Workflow state machine: phase gate, inspect_context, apply_patch with compiler validation",
  "enabled": true,
  "type": "local"
}
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
