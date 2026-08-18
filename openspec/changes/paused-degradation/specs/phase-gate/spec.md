## MODIFIED Requirements

### Requirement: Phase state machine

系統 SHALL 維護一組有限狀態機，包含 INIT / PLANNING / EXECUTING / VERIFYING / COMPLETED / PAUSED 六個階段。

#### Scenario: 初始階段為 INIT

- **WHEN** MCP server 首次啟動且無 state.json
- **THEN** 系統建立預設狀態，phase 為 INIT

#### Scenario: 正常流程逐階轉移

- **WHEN** agent 依序呼叫 checkpoint 並指定 next_phase
- **THEN** 系統依 INIT → PLANNING → EXECUTING → VERIFYING → COMPLETED 順序轉移

#### Scenario: 自動降級至 PAUSED

- **WHEN** apply_patch 連續失敗 3 次
- **THEN** 系統自動將 phase 轉為 PAUSED，僅允許 get_status 與 checkpoint

#### Scenario: 從 PAUSED 復原

- **WHEN** agent 在 PAUSED 階段呼叫 checkpoint 指定 next_phase 為 PLANNING 或 EXECUTING
- **THEN** 系統轉移至指定 phase，恢復正常操作

## ADDED Requirements

### Requirement: PAUSED phase action whitelist

PAUSED 階段 SHALL 僅允許 get_status 與 checkpoint 工具。

#### Scenario: PAUSED 阻擋寫入操作

- **WHEN** agent 在 PAUSED 階段呼叫 apply_patch 或 inspect_context
- **THEN** 系統回傳 Error，提示目前處於 PAUSED 階段，須先 checkpoint 復原