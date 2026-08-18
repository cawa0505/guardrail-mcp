## Purpose

定義編碼工作流程的階段轉換規則，確保 LLM agent 在每個階段只能呼叫對應的工具，防止跳階操作。

## Requirements

### Requirement: Phase state machine

系統 SHALL 維護一組有限狀態機，包含 INIT / PLANNING / EXECUTING / VERIFYING / COMPLETED 五個階段。

#### Scenario: 初始階段為 INIT

- **WHEN** MCP server 首次啟動且無 state.json
- **THEN** 系統建立預設狀態，phase 為 INIT

#### Scenario: 正常流程逐階轉移

- **WHEN** agent 依序呼叫 checkpoint 並指定 next_phase
- **THEN** 系統依 INIT → PLANNING → EXECUTING → VERIFYING → COMPLETED 順序轉移

### Requirement: Phase transition validation

系統 SHALL 驗證 Phase 轉移是否合法，不合法的轉移應被拒絕。

#### Scenario: 非法轉移被拒絕

- **WHEN** agent 嘗試從 INIT 直接跳到 VERIFYING
- **THEN** 系統回傳 Error，列出允許的目標 Phase

### Requirement: Action whitelist per phase

系統 SHALL 為每個 Phase 定義允許的工具白名單，非白名單工具應被阻擋。

#### Scenario: 阻擋非允許操作

- **WHEN** agent 在 PLANNING 階段呼叫 apply_patch
- **THEN** 系統回傳 Error，提示當前 Phase 與允許的操作

### Requirement: Phase gate on all tools

每個 MCP tool handler SHALL 在進入業務邏輯前先檢查 Phase Gate。

#### Scenario: 工具入口先檢查

- **WHEN** 任何工具被呼叫
- **THEN** handler 檢查當前 phase 是否允許該工具，不允許則提前回傳 Error

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
