## Purpose

定義 Canonical State 的儲存格式與存取方式，作為工作流程狀態的單一事實來源。

## Requirements

### Requirement: State file location

系統 SHALL 將狀態儲存於 `<CWD>/.opencode/state.json`。

#### Scenario: 自動建立目錄

- **WHEN** 系統首次需要存取狀態
- **THEN** 系統自動建立 `.opencode/` 目錄

### Requirement: State schema

系統 SHALL 使用 JSON 格式儲存狀態，包含 version、phase、active_goal、allowed_actions、staging_buffer、checkpoints、ast_synced 欄位。

#### Scenario: 讀取預設狀態

- **WHEN** state.json 不存在
- **THEN** 系統回傳預設狀態（version "1.0.0", phase "INIT"）

### Requirement: Gitignore auto-management

系統 SHALL 確保 `.opencode/` 目錄被 `.gitignore` 排除。

#### Scenario: 自動追加 gitignore

- **WHEN** stateDir() 被呼叫且 `.gitignore` 不包含 `.opencode/`
- **THEN** 系統自動將 `.opencode/` 追加到 `.gitignore`

### Requirement: Allowed actions sync

每次 loadState() SHALL 根據當前 Phase 重新同步 allowed_actions。

#### Scenario: 啟動時同步

- **WHEN** 系統啟動載入 state
- **THEN** allowed_actions 自動設為當前 phase 的允許列表
