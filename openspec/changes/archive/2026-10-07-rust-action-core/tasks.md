# Tasks: Rust Action Governance Core

## Phase 0: Rust Workspace Scaffold & Invariant Characterization (R0)

- [x] 0.1 初始化 Rust Cargo Workspace（`guardrail-core`, `guardrail-adapter-coding`, `guardrail-server`）
- [x] 0.2 建立核心 Domain Structs（`Actor`, `Action`, `Decision`, `EvidenceRecord`）
- [x] 0.3 實作純 Rust Phase State Machine（涵蓋 INIT, PLANNING, EXECUTING, VERIFYING, PAUSED, COMPLETED 與終態 FAILED, CANCELLED）
- [x] 0.4 為 State Machine 撰寫單元測試，比對現有 Go 版的轉移行為
- [x] 0.5 實作 Commit Token 管理器（綁定 proposal hash, workspace, revision, TTL 與單次核銷）

## Phase 1: Storage & Evidence Engine (R0/R1)

- [x] 1.1 實作 Canonical State Store（支援讀寫 JSON，路徑預設相容 `.guardrail/` 與相容 fallback `.opencode/`）
- [x] 1.2 實作 Append-Only Evidence Log（JSONL 序列化與 SHA-256 parent_event 雜湊鏈計算）
- [x] 1.3 撰寫 State Store 與 Evidence Log 的並行寫入與損毀復原測試（tests/persistence.rs 4 測試全綠）

## Phase 2: Dual-Layer Guard Engine (R1)

- [x] 2.1 實作 Hard Guard 規則評估引擎（Phase 白名單、Schema 驗證、Permission 檢查）
- [x] 2.2 實作 Soft Guard 抽象 Trait 與 HTTP Verifier 呼叫客戶端（Trait 與 mock HTTP 往返測試通過）
- [x] 2.3 實作 Decision 合併邏輯（Hard Guard 優先拒絕；Soft Guard 回傳驗證結果）

## Phase 3: Coding Adapter & AST Bridge (R0)

- [x] 3.1 實作 CodingAdapter，將 `apply_patch`, `inspect_context` 封裝為標準 Action
- [x] 3.2 移植骨架提取邏輯至 `guardrail-adapter-coding`（沿用 Go 版 regex/reduce 策略，非 tree-sitter）
- [x] 3.3 實作 Compiler Verifier（`cargo check` / `tsc` / `go vet` 外部指令驗證管線）

## Phase 4: rmcp Server & Tool Compatibility (R0/R1)

- [x] 4.1 使用 `rmcp` 3.x 實作 Stdio MCP Server，註冊原有 5 個相容工具（`apply_patch`, `checkpoint`, `get_status`, `inspect_context`, `commit_token`）
- [x] 4.2 整合 rmcp Stdio Client 呼叫外部 Graphify MCP server（R0 核心不直接外部依賴，收斂至 R1 整合）
- [x] 4.3 實作 Elicitation / Approval 攔截流程（當 Decision 為 `REQUIRE_APPROVAL` 時透過 MCP elicitation 回詢；收斂至 R1 整合）

## Phase 5: Verification & Parity Test Suite (R0)

- [x] 5.1 端對端整合驗證：stdio MCP handshake + 完整生命週期（INIT→PLANNING→EXECUTING、phase gate DENY/ALLOW、apply_patch + compiler、state 持久化、evidence 鏈）
- [x] 5.2 驗證 `openspec validate` 通過
- [x] 5.3 驗證 `cargo test` 全數通過（9 tests）
