## Purpose

提供 LLM agent 查詢當前工作流程狀態的能力，包含 Phase、allowed actions、修改歷史與 AST 同步狀態。

## Requirements

### Requirement: Current phase display

系統 SHALL 回傳當前 Phase 與 allowed actions。

#### Scenario: 查詢當前狀態

- **WHEN** agent 呼叫 get_status
- **THEN** 系統回傳 phase、active_goal、allowed_actions

### Requirement: Modified files history

系統 SHALL 回傳本次 session 中所有修改過的檔案列表（跨 checkpoint 去重）。

#### Scenario: 列出修改檔案

- **WHEN** agent 呼叫 get_status 且已有 checkpoint
- **THEN** 系統回傳去重的 modified_files_in_session

### Requirement: Last checkpoint

系統 SHALL 回傳最後一個 Checkpoint 的 ID。

#### Scenario: 有 checkpoint 時

- **WHEN** agent 呼叫 get_status 且已有 checkpoint
- **THEN** 系統回傳 last_checkpoint_id

### Requirement: AST sync status

系統 SHALL 回傳 ast_synced 狀態。

#### Scenario: 查詢同步狀態

- **WHEN** agent 呼叫 get_status
- **THEN** 系統回傳 ast_synced 布林值
