## Context

本變更定義 GuardrailMcp 的架構層次與責任邊界。核心問題：statemachine core、Track 1 Hard Guard、Track 2 Soft Guard、Graphify 之間的關係是什麼？什麼是核心、什麼是插件、什麼是外部 provider？

見 `proposal.md` — Why 段落取得完整動機。

## 架構概覽

```
┌─────────────────────────────────────────────┐
│              MCP Server 介面層               │
│  (stdio transport, tool dispatch)           │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│         State Machine Core                   │
│  phase transition · state persistence        │
│  staging/2PC orchestration · crash recovery  │
│  commit token binding                        │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│  Track 1: Hard Guard (同步阻擋)               │
│  · phase gate（狀態機 whitelist）              │
│  · policy validation（per-phase rules）        │
│  · Graphify MCP（AST/code graph 驗證）        │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│  Track 2: Soft Guard (啟用時同步阻擋)          │
│  · Docker compiler/linter/tests              │
│  · docker-llm-as-a-verifier（外部專案，已實作、E2E 驗證中）│
│  · local llama-server verifier（外部專案，已實作、E2E 驗證中）│
│  · 未來: VL UI verifier                      │
└─────────────────────────────────────────────┘
```

## Goals / Non-Goals

**Goals:**
- 明確定義 statemachine core 的責任：phase/state/staging/2PC orchestration，不含業務驗證邏輯
- 定義 Track 1 Hard Guard 為同步阻擋，包含 phase gate、policy 與 Graphify MCP 驗證
- 定義 Track 2 Soft Guard 為可配置但啟用時同步阻擋的 verifier 整合層
- 確認 Graphify 是目前必要的外部 AST provider，核心不自行 tree-sitter parse
- 定義缺少 Track 2 verifier 時的行為：允許按 policy 停用，不得默默略過 required verifier
- 定義未來 Graphify plugin 內嵌 SDK 的定位：獨立範例，不提前耦合

**Non-Goals:**
- 不建立任何抽象 interface 或 Go interface 程式碼
- 不修改 `openspec/specs/` 既有檔案
- 不修改 `README.md`
- 不建立 `examples/graphify-sdk-plugin/` 目錄或範例程式碼
- 不修改 Go source 程式碼
- 不開發或修改外部 verifier 本身（docker-llm-as-a-verifier、llama-server 已在外部專案完成，本 repo 只需串接整合）

## Decisions

- **D-1 Statemachine Core 不包含驗證邏輯**：core 只負責 phase/state/staging/2PC orchestration。驗證邏輯由 Track 1 / Track 2 實作，core 不內建 compiler、linter、tree-sitter 等。
- **D-2 Track 1 Hard Guard 同步阻擋**：phase gate、policy、Graphify MCP 呼叫均在操作執行前同步完成，失敗則拒絕操作。
- **D-3 Track 2 Soft Guard 啟用時同步阻擋**：Soft Guard verifier 可依部署環境配置啟用／停用，但啟用後的行為與 Hard Guard 相同（同步阻擋）。不允許「啟用但默默失敗」的狀態。
- **D-4 Graphify 是目前外部 MCP provider**：核心透過 MCP protocol 呼叫 Graphify MCP 做 AST/code graph 驗證。核心不直接 tree-sitter parse，不內建 Graphify client SDK。
- **D-5 缺少 required Track 2 verifier 時依 policy 處理**：若 policy 標記某 verifier 為 required，但該 verifier 不存在或配置不完整，server 應拒絕操作並回傳明確錯誤。若 policy 允許 optional，則跳過該 verifier。不得默默略過。
- **D-6 未來 Graphify plugin example**：獨立於核心的 Go example 專案（`examples/graphify-sdk-plugin/`），示範如何以 Go SDK 實作 Graphify provider/plugin，驗證與 guardrail-mcp 的整合契約。這是未來／獨立範例，不是目前核心內嵌能力。共用 provider contract，不把 SDK 提前耦合到現在核心。

## Risks / Trade-offs

- **[MCP 依賴] Graphify MCP 不可用時 Hard Guard 降級**：若 Graphify MCP server 未啟動或回應逾時，AST 驗證無法執行。→ 緩解：定義 degraded mode，依 policy 決定放行或拒絕。不允許默默跳過。
- **[Track 2 配置複雜度] 多種 verifier 組合增加部署難度**：Docker compiler + LLM verifier + VL UI verifier 的組合配置可能讓使用者困惑。→ 緩解：提供 preset 配置範本，初期先支援 Docker compiler 與 llama-server。
- **[過早抽象化] 不要現在定義 interface**：未來 Rust 重寫時可能重新設計邊界。→ 緩解：現在只用具體型別，不引入 interface abstraction。等待 Rust 實作時再評估抽象層。

## 未來 Graphify SDK Plugin 定位

```
┌─────────────────────────────────────┐
│  guardrail-mcp（核心，Go）               │
│  使用 Graphify MCP protocol 呼叫     │
└──────────────────┬──────────────────┘
                   │ MCP protocol
┌──────────────────▼──────────────────┐
│  Graphify MCP Server（外部）          │
│  或                               │
│  examples/graphify-sdk-plugin/（未來）│
│  Go SDK plugin example              │
└─────────────────────────────────────┘
```

兩者共用 provider contract（MCP tool protocol），但 SDK plugin example 是獨立的 Go binary，非核心內嵌。未來 Rust 重寫時可依相同 contract 實作 Rust 版 plugin。

## Open Questions

- [待討論] Track 2 verifier 的 plugin loading 機制：是透過 MCP protocol 還是 dynamic library？
- [待討論] commit token 的過期時間與 revocation 機制？
