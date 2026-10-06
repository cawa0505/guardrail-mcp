# Design: Rust Action Governance Core

## Architectural Overview

GuardrailMCP 採用雙層（Two-Tier）結構，嚴格劃分純邏輯與 IO 副作用：

```
+-------------------------------------------------------------+
|                 rmcp Native Shell (tokio)                  |
|  - MCP Stdio Server (tools: inspect_context, apply_patch..) |
|  - MCP Stdio Client (TokioChildProcess -> Graphify)         |
|  - HTTP Soft Guard Client (reqwest / hyper)                |
|  - File State Store & Evidence Append-Only JSONL Writer     |
+-------------------------------------------------------------+
                              |
                              v (純資料結構輸入/輸出)
+-------------------------------------------------------------+
|            guardrail-core (Pure Rust / WASM-ready)          |
|  - Actor / Action / Decision / Evidence Domain Models       |
|  - Phase Finite State Machine (exhaustive match)           |
|  - Hard Guard (deterministic policy engine)                 |
|  - Soft Guard trait definitions                             |
|  - Commit Token lifecycle manager                           |
+-------------------------------------------------------------+
```

## Cargo Workspace Layout

```
repos/GuardrailMcp/
├── Cargo.toml                  # Workspace root
├── crates/
│   ├── guardrail-core/         # Pure logic, #![no_std] friendly, wasm32 targetable
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── model/          # Actor, Action, Decision, Evidence
│   │       ├── fsm/            # StateMachine, Phase, Transitions
│   │       ├── policy/         # HardGuard, Rules
│   │       ├── token/          # CommitTokenManager
│   │       └── lib.rs
│   ├── guardrail-adapter-coding/ # Coding agent specific logic (patch parser, AST bridge)
│   │   ├── Cargo.toml
│   │   └── src/
│   └── guardrail-server/       # rmcp stdio binary
│       ├── Cargo.toml
│       └── src/
│           ├── mcp/            # rmcp tools & handlers
│           ├── storage/        # JSON state & JSONL evidence
│           ├── client/         # Graphify child process client
│           └── main.rs
```

## Core Models

### 1. Actor & Action

```rust
pub struct Actor {
    pub agent_id: String,
    pub role: String,
    pub session_id: String,
    pub tenant_id: Option<String>,
}

pub struct Action {
    pub action_id: String,
    pub actor: Actor,
    pub action_type: String, // e.g. "code:apply_patch", "tool:call"
    pub target: String,      // e.g. "filepath:src/main.rs"
    pub payload: serde_json::Value,
    pub context: serde_json::Value,
    pub timestamp: i64,
}
```

### 2. Decision

```rust
pub enum Decision {
    Allow {
        reason: String,
        commit_token: Option<CommitToken>,
    },
    Deny {
        reason: String,
        code: String,
    },
    RequireVerification {
        reason: String,
        verifiers: Vec<String>,
    },
    RequireApproval {
        reason: String,
        approver_role: String,
    },
}
```

### 3. Phase State Machine

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum Phase {
    Init,
    Planning,
    Executing,
    Verifying,
    Completed,
    Failed,
    Cancelled,
}
```

轉移規則：
- `Init` -> `Planning`
- `Planning` -> `Executing`, `Planning`, `Failed`, `Cancelled`
- `Executing` -> `Verifying`, `Executing`, `Failed`, `Cancelled`
- `Verifying` -> `Completed`, `Executing`, `Failed`, `Cancelled`
- `Completed` -> (Terminal)
- `Failed` -> `Planning` (重試)
- `Cancelled` -> (Terminal)

### 4. Evidence Record (JSONL)

```rust
pub struct EvidenceRecord {
    pub evidence_id: String,
    pub timestamp: i64,
    pub session_id: String,
    pub actor: Actor,
    pub action: Action,
    pub decision: Decision,
    pub verifier_results: Vec<VerifierResult>,
    pub parent_event_hash: Option<String>, // SHA-256 link
    pub event_hash: String,               // SHA-256 of canonical JSON
}
```

## Commit Token Lifecycle

1. **Create**: 於審查或計畫通過後核發，綁定 `proposal_hash`、`workspace_path`、`target_revision` 與 `ttl_minutes`。
2. **Validate**: 檢查有效期限與工作目錄狀態是否相符。
3. **Consume**: 執行危險操作（如 apply_patch）時單次核銷，一旦核銷即不可再次使用。
4. **Revoke**: 當環境或工作階段重置時撤銷。

## Backward Compatibility with Existing Tools

既有 OpenCode 使用者依賴的工具介面在 `guardrail-server` 中維持 100% 參數相容：
- `apply_patch` (path, search_block, replace_block, auto_commit_checkpoint)
- `checkpoint` (summary, next_phase)
- `get_status` ()
- `inspect_context` (path, mode, line_range)
- `commit_token` (action, token_id, proposal_hash, ...)
底層則轉譯為 `Action` 送入 `guardrail-core` 裁決。
