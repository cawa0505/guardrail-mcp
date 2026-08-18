## Purpose

在 apply_patch 成功後背景觸發 graphify extract，保持 AST 知識圖譜與程式碼同步。ast_synced 狀態可透過 get_status 查詢。

## Requirements

### Requirement: Background graphify extract

系統 SHALL 在 apply_patch 成功後以背景 goroutine 執行 `graphify extract`。

#### Scenario: patch 成功後觸發

- **WHEN** apply_patch 通過 compiler 驗證並寫入成功
- **THEN** 系統以 goroutine 背景執行 graphify extract

### Requirement: ast_synced flag

系統 SHALL 在 apply_patch 成功後將 ast_synced 設為 false，graphify extract 完成後設為 true。

#### Scenario: patch 後設為 stale

- **WHEN** apply_patch 成功
- **THEN** ast_synced 設為 false

#### Scenario: extract 完成設為 synced

- **WHEN** graphify extract 背景執行成功
- **THEN** ast_synced 設為 true

### Requirement: Extract failure handling

graphify extract 失敗時 SHALL 不影響主流程，ast_synced 保持 false。

#### Scenario: extract 失敗不阻擋

- **WHEN** graphify extract 背景執行失敗
- **THEN** 主流程不受影響，ast_synced 保持 false
