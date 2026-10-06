## Purpose

定義編碼工作流程的階段轉換規則，確保 LLM agent 在每個階段只能呼叫對應的工具，防止跳階操作。

## Requirements

### Requirement: Phase state machine

系統 SHALL 維護一組有限狀態機，包含 INIT / PLANNING / EXECUTING / VERIFYING / COMPLETED / FAILED / CANCELLED 七個階段。

#### Scenario: 初始階段為 INIT

- **WHEN** MCP server 首次啟動且無 state.json
- **THEN** 系統建立預設狀態，phase 為 INIT

#### Scenario: 正常流程逐階轉移

- **WHEN** agent 依序呼叫 checkpoint 並指定 next_phase
- **THEN** 系統依 INIT → PLANNING → EXECUTING → VERIFYING → COMPLETED 順序轉移

#### Scenario: 轉移至失敗終態 FAILED

- **WHEN** 任務執行遭遇不可復原錯誤，或 agent/orchestrator 指定 next_phase 為 FAILED
- **THEN** 系統將狀態標記為 FAILED，僅允許唯讀查詢與復原轉移

#### Scenario: 轉移至取消終態 CANCELLED

- **WHEN** 操作者或上游工作流取消任務，指定 next_phase 為 CANCELLED
- **THEN** 系統將狀態標記為 CANCELLED，進入終態

#### Scenario: 從 FAILED 重試復原

- **WHEN** agent 在 FAILED 狀態呼叫 checkpoint 並指定 next_phase 為 PLANNING
- **THEN** 系統允許轉移至 PLANNING 階段重新開始

### Requirement: Phase transition validation

系統 SHALL 驗證 Phase 轉移是否符合狀態機定義，不合法的轉移應被拒絕。

#### Scenario: 非法轉移被拒絕

- **WHEN** agent 嘗試從 INIT 直接跳到 VERIFYING 或 COMPLETED
- **THEN** 系統回傳 DENY 或 Error，列出允許的目標 Phase

### Requirement: Action whitelist per phase

系統 SHALL 為每個 Phase 定義允許的 Action / 工具清單，未授權操作應被阻擋。

#### Scenario: 阻擋非允許操作

- **WHEN** agent 在 PLANNING 階段呼叫 apply_patch
- **THEN** 系統產生 DENY 裁決並阻擋執行

### Requirement: Phase gate on all tools

每個外部進入點 SHALL 在執行具體 Action 前先通過 Phase Gate 評估。

#### Scenario: 工具入口先檢查

- **WHEN** 任何工具被呼叫
- **THEN** 系統先交由 Governance Core 評估當前 Phase 是否允許該 Action，未通過則提早終止

## Phase 對照表

| Phase | 允許工具 |
|-------|---------|
| INIT | get_status |
| PLANNING | inspect_context, checkpoint, get_status |
| EXECUTING | inspect_context, apply_patch, checkpoint, get_status |
| VERIFYING | inspect_context, get_status |
| COMPLETED | get_status |

## Phase 轉移規則

| 當前 Phase | 允許轉移至 |
|-----------|-----------|
| INIT | PLANNING |
| PLANNING | EXECUTING, PLANNING |
| EXECUTING | VERIFYING, EXECUTING |
| VERIFYING | COMPLETED, EXECUTING |
| COMPLETED | （無） |
