## Purpose

定義 GuardrailMcp 的產品命名、責任邊界、雙軌驗證行為，以及 Graphify 的 current/future 定位。確保架構文件與產品事實一致，不誇大現況。

## Requirements

### Requirement: 產品命名邊界

對外定位為 Guardrail MCP；remote repository 名稱使用 `guardrail-mcp`；本地 Go module / binary / 目錄名稱維持 `statemachine-mcp` / `GuardrailMcp`。不在本變更進行本地改名。

#### Scenario: 文件中使用產品名稱

- **WHEN** 任何文件提及產品定位
- **THEN** 對外定位用「Guardrail MCP」描述，remote repository 用「guardrail-mcp」，本地 module/binary 維持「statemachine-mcp / GuardrailMcp」

#### Scenario: 實作語言描述

- **WHEN** 文件中提及實作語言
- **THEN** 核心實作語言為 Go；Rust 僅列為未來方向，不得寫成現況

#### Scenario: 不將本地改名列入本變更

- **WHEN** 文件中提及 repository 改名
- **THEN** 僅描述 remote repo 改名為 guardrail-mcp；本地 Go module path、binary name、目錄名稱的改名為後續相容性變更，不在本文件變更範圍內

### Requirement: Statemachine core 責任邊界

Statemachine core 負責 phase/state/staging/2PC orchestration，不包含業務驗證邏輯。

#### Scenario: core 不內建 compiler/linter

- **WHEN** 描述 core 的責任範圍
- **THEN** core 不內建 compiler、linter、tree-sitter 等驗證工具；驗證由 Track 1 / Track 2 實作

#### Scenario: core 負責 crash recovery

- **WHEN** process crash 後重新啟動
- **THEN** core 應從磁碟復原 state.json 與 staging buffer，確保未完成的 2PC 可恢復

### Requirement: Track 1 Hard Guard 同步阻擋

Track 1 為同步阻擋機制，包含 phase gate、policy validation、Graphify MCP AST/code graph 驗證。

#### Scenario: phase gate 阻擋非法操作

- **WHEN** agent 在當前 phase 呼叫不允許的工具
- **THEN** Track 1 在操作執行前同步阻擋並回傳 Error，列出當前 phase 與允許操作

#### Scenario: Graphify MCP 驗證失敗

- **WHEN** Graphify MCP 回應驗證失敗或逾時
- **THEN** Track 1 依 degraded policy 決定放行或拒絕；不得默默跳過

### Requirement: Track 2 Soft Guard 啟用時同步阻擋

Track 2 verifier 可依部署環境配置啟用或停用；啟用後的行為與 Track 1 相同（同步阻擋）。不允許「啟用但默默失敗」的狀態。

#### Scenario: 啟用的 verifier 驗證失敗

- **WHEN** 某個啟用的 Track 2 verifier 驗證失敗
- **THEN** 操作被同步阻擋，回傳明確的驗證失敗訊息

#### Scenario: required verifier 不存在

- **WHEN** policy 標記某 Track 2 verifier 為 required，但該 verifier 不存在或配置不完整
- **THEN** server 拒絕操作並回傳錯誤，指出缺少的 required verifier

#### Scenario: optional verifier 不存在

- **WHEN** policy 標記某 Track 2 verifier 為 optional，且該 verifier 不存在
- **THEN** server 跳過該 verifier，繼續執行其他驗證

### Requirement: 外部 verifier 成熟度描述

docker-llm-as-a-verifier 與 local llama-server verifier 為外部專案，已實作完成、E2E 驗證中。本 repo 僅負責串接整合。

#### Scenario: 文件中提及外部 verifier

- **WHEN** 文件提及 docker-llm-as-a-verifier 或 local llama-server verifier
- **THEN** 描述為「外部專案，已實作、E2E 驗證中」，不可列為待開發

### Requirement: Graphify 現況定位

Graphify 是目前必要的外部 AST provider，核心透過 MCP protocol 呼叫 Graphify MCP。核心不自行 tree-sitter parse。

#### Scenario: Graphify 為外部 MCP

- **WHEN** 描述 Graphify 的整合方式
- **THEN** 核心透過 MCP protocol 呼叫外部 Graphify MCP server，不內建 tree-sitter 或 Graphify client SDK

#### Scenario: degraded mode

- **WHEN** Graphify MCP 未啟動或無法連線
- **THEN** 依 policy 決定進入 degraded mode（放行或拒絕），不得默默跳過驗證

### Requirement: Graphify 未來 SDK plugin 定位

未來 Graphify SDK plugin 為獨立於核心的範例專案，示範以 Go SDK 實作 Graphify provider/plugin。不把 SDK 提前耦合到現在核心。

#### Scenario: 文件中提及未來 SDK plugin

- **WHEN** 文件描述未來 Graphify plugin
- **THEN** 明確標示為「未來／獨立範例」，非目前核心內嵌能力；兩者共用 provider contract

### Requirement: 禁止誇大現況

文件中不得將未實作或規劃中的功能描述為已完成現況。

#### Scenario: 文件審查

- **WHEN** 任何架構或產品文件描述功能狀態
- **THEN** 使用「已實作」、「規劃中」、「未來方向」等明確區分現況與規劃；不得使用 zero latency 等誇張聲稱，應以 low-latency / deterministic 替代
