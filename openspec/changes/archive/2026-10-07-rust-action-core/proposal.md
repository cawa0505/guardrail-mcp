# Proposal: Rust Action Governance Core (R0+R1)

## Context

GuardrailMCP 目前是一個以 Go 實作的 MCP server（~10 個 internal Go 檔案），專門為 coding-agent（特別是 OpenCode）設計：
- 狀態儲存路徑綁死 `<CWD>/.opencode/state.json`
- Phase 白名單寫死 coding 工具（`apply_patch`, `inspect_context`, `checkpoint`, `get_status`）
- 缺少通用的 Action / Actor 抽象
- 缺少明確的 Decision 模型（目前只有硬性的 error / success）
- 缺少不可竄改的 Evidence Event Log 格式

依據 PRD v0.1（`repos/Themis/docs/guardrailmcp-themis-prd.md`），GuardrailMCP 的定位是**開源、通用的 Agent 行為治理原語（Agent Action Governance Primitive）**，提供 deterministic 的行為控制、狀態機保證、雙軌 Guard 與審計憑證。

社群與架構決策（Memory `#12671`）：
- **R0 即改用 Rust 重構**：直接以 Rust 建立新 core，跳過 Go 階段通用化再移植的雙重沉沒成本。
- **IO-free Core / WASM-ready**：Governance Core（State Machine, Policy Engine, Decision, Evidence）保持純記憶體計算、零 IO，保證未來可編譯至 `wasm32-unknown-unknown`。
- **rmcp 3.x 傳輸外殼**：採用官方 `rmcp` SDK（tokio-based），同時作為對 Agent 的 stdio MCP server 與對 Graphify 的 stdio MCP client。

## Scope

本變更涵蓋 PRD 的 **R0（核心行為平移與 Rust 建立）** 與 **R1（通用治理核心重構）**：

1. **核心領域模型（Pure Rust, IO-free）**：
   - `Actor`: `agent_id`, `role`, `session_id`, `tenant_id`
   - `Action`: `action_id`, `actor`, `action_type`, `target`, `payload`, `context`, `timestamp`
   - `Decision`: `ALLOW`, `DENY`, `REQUIRE_VERIFICATION`, `REQUIRE_APPROVAL`
   - `Evidence`: 結構化 JSONL 事件記錄（SHA-256 parent_event 雜湊鏈）
2. **Phase 狀態機擴充**：
   - 納入終態 `FAILED` 與 `CANCELLED`
   - 支援狀態機通用轉移驗證
3. **Commit Token 生命週期**：
   - 單次使用授權憑證：`create`, `validate`, `consume`, `revoke`
   - 綁定 proposal_hash, workspace, revision, TTL
4. **雙軌 Guard 架構**：
   - Hard Guard（確定性規則）：Phase 檢查、Schema 驗證、Permission 白名單
   - Soft Guard（可插拔驗證器）：HTTP Verifier 介面、Compiler/Linter、LLM Reviewer
5. **Adapter 架構**：
   - 將既有 OpenCode / Coding 工具抽離為 `CodingAdapter`，解耦 core 與特定 agent 介面
6. **State Store 通用化**：
   - 狀態路徑解耦 `.opencode/`，支援預設 `.guardrail/state.json` 與環境變數 / CLI 覆寫

## Non-Goals

- 本變更不包含 Themis 企業層（Tenant API、組織身分管理、審批工作流 UI、中央憑證庫），這些屬於 Themis 專屬規格。
- 本變更不實作分散式儲存；Evidence 先以本地 Append-Only JSONL 為準。
- 本變更不實作多租戶隔離執行期（留待 R4 Themis 整合）。
