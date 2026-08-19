# GuardrailMcp

**Phase-gated coding workflow with dual-track validation, as an MCP server.**

GuardrailMcp 是一個 MCP (Model Context Protocol) server，提供狀態機驅動的編碼工作流程。包含雙軌驗證架構：
- **Track 1 Hard Guard** — Phase Gate 狀態機 + Graphify AST 驗證
- **Track 2 Soft Guard** — 可配置的 HTTP verifier 陣列（Docker compiler、LLM 等）

每個操作經過 Phase Gate 階段檢查、雙緩衝編譯驗證，支援 Checkpoint 斷點續接與 Commit Token 授權機制。

## 專案目標

- **Phase Gate 流程控管** — 定義 INIT → PLANNING → EXECUTING → VERIFYING → COMPLETED 的階段轉換，每個階段只開放對應的工具
- **雙緩衝 + 編譯驗證** — `apply_patch` 先通過 compiler（`cargo check` / `go build` / `tsc`）才寫入硬碟，失敗自動復原
- **Token 節省** — `inspect_context` 支援 skeleton / range / full_cleaned 模式；深度 AST 分析委託 [Graphify MCP](https://github.com/cawa0505/graphify-mcp)（可選，stdio 子行程）
- **Soft Guard 驗證** — 可配置的 HTTP verifier 陣列，支援 required/optional 旗標，verifier 失敗可阻擋操作
- **Commit Token 授權** — 單次使用的 token，綁定 proposal hash、workspace 路徑與 git revision
- **Checkpoint 機制** — 進度寫入 `.opencode/state.json`，支援斷點續接

## 目錄結構

```
cmd/guardrail-mcp/main.go     — entry point + tool handlers
internal/
  state/                        — State schema, phase gate, load/save
  token/                        — CommitToken 生命週期
  inspect/                      — 檔案檢測與 token 節省
  apply/                        — patch、compiler、staging、micro-patch 防護
  graphify/                     — Graphify MCP stdio client wrapper（可選）
  softguard/                    — Soft Guard verifier 框架（HTTP verifier 陣列）
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
┌──────────────────────────────────────┐
│  Track 1 Hard Guard                  │
│  ┌───────────────────────────────┐   │
│  │  Phase Gate (state machine)   │   │
│  │  INIT→PLANNING→EXECUTING→…   │   │
│  └───────────┬───────────────────┘   │
│              ▼                       │
│  ┌───────────────────────────────┐   │
│  │  Graphify AST (可選 stdio)    │   │
│  └───────────────────────────────┘   │
├──────────────────────────────────────┤
│  Track 2 Soft Guard                  │
│  ┌───────────────────────────────┐   │
│  │  HTTP Verifier 陣列            │   │
│  │  (Docker compiler / LLM / …)  │   │
│  │  required/optional 旗標        │   │
│  └───────────────────────────────┘   │
├──────────────────────────────────────┤
│  Tools                               │
│  inspect_context  → 語言偵測 + skeleton│
│  apply_patch      → search/replace   │
│                     → Compiler Verify │
│                     → Soft Guard 驗證 │
│                     → 寫入 + Checkpoint│
│  commit_token     → 建立/驗證/消費/撤銷│
│  get_status       → Phase + 歷史 + 驗證│
│  checkpoint       → state.json + Phase│
└──────────────────────────────────────┘
```

## 工具列表

| 工具 | 說明 |
|------|------|
| `inspect_context` | 安全讀取檔案結構，支援 skeleton / range / full_cleaned 模式 |
| `apply_patch` | search/replace 套用修改，通過 compiler 驗證 + Soft Guard 驗證後寫入硬碟 |
| `commit_token` | 管理 Commit Token 生命週期：create / validate / consume / revoke / status |
| `get_status` | 查詢當前 Phase、allowed actions、checkpoint 與 token 資訊 |
| `checkpoint` | 建立進度快照，轉移 Phase 階段 |

## 快速開始

```bash
# Build and install
go build -o ~/.local/bin/guardrail-mcp ./cmd/guardrail-mcp/
# Or install directly
go install ./cmd/guardrail-mcp/
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