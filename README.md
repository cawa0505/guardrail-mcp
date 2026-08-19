# GuardrailMcp

**Phase-gated coding workflow as an MCP server.**

GuardrailMcp 是一個 MCP (Model Context Protocol) server，提供一套狀態機驅動的編碼工作流程工具。每個操作都經過 Phase Gate 檢查、雙緩衝編譯驗證，並支援 Checkpoint 斷點續接與 Commit Token 授權機制，讓 LLM agent 的程式修改流程更可控、可回復。

## 專案目標

- **Phase Gate 流程控管** — 定義 INIT → PLANNING → EXECUTING → VERIFYING → COMPLETED 的階段轉換，每個階段只開放對應的工具
- **雙緩衝 + 編譯驗證** — `apply_patch` 先通過 compiler（`cargo check` / `go build` / `tsc`）才寫入硬碟，失敗自動復原
- **Token 節省** — `inspect_context` 透過 regex 骨架萃取減少 LLM context 消耗；深度 AST 分析委託 [Graphify MCP](https://github.com/cawa0505/graphify-mcp)
- **Commit Token 授權** — 單次使用的 token，綁定 proposal hash、workspace 路徑與 git revision，確保 commit 操作的安全性
- **Checkpoint 機制** — 進度寫入 `.opencode/state.json`，支援斷點續接
- **Graphify 整合** — 背景自動觸發 graphify extract，`get_status` 可查詢 `ast_synced` 狀態

## 目錄結構

```
cmd/guardrail-mcp/main.go     — entry point + tool handlers
internal/
  state/                        — State schema, phase gate, load/save
  token/                        — CommitToken 生命週期
  inspect/                      — 檔案檢測與 token 節省
  apply/                        — patch、compiler、staging、graphify 整合
openspec/                       — 規格與變更管理
```

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
       ├── commit_token     → 建立/驗證/消費/撤銷 — 單次使用授權
       ├── get_status       → 當前 Phase + 修改歷史 + token 狀態
       └── checkpoint       → state.json + Phase 轉移
```

## 工具列表

| 工具 | 說明 |
|------|------|
| `inspect_context` | 安全讀取檔案結構，支援 skeleton / range / full_cleaned 模式 |
| `apply_patch` | search/replace 套用修改，通過 compiler 驗證後寫入硬碟 |
| `commit_token` | 管理 Commit Token 生命週期：create / validate / consume / revoke / status |
| `get_status` | 查詢當前 Phase、allowed actions、checkpoint 與 token 資訊 |
| `checkpoint` | 建立進度快照，轉移 Phase 階段 |

## 快速開始

```bash
# Build
go build -o ~/.local/bin/guardrail-mcp ./cmd/guardrail-mcp/
```

### OpenCode 整合

在 `opencode.json` 的 `mcp` 區塊加入：

```json
"guardrail-mcp": {
  "command": ["{env:HOME}/.local/bin/guardrail-mcp"],
  "description": "Workflow state machine: phase gate, inspect_context, apply_patch, commit_token",
  "enabled": true,
  "type": "local"
}
```

## License

MIT