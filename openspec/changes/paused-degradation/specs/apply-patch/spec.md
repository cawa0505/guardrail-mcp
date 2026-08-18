## MODIFIED Requirements

### Requirement: Compiler validation

系統 SHALL 在寫入 patch 後執行 compiler 驗證，驗證失敗時復原原始內容，並遞增連續失敗計數器。連續失敗達門檻時自動降級至 PAUSED 階段。

#### Scenario: 編譯驗證成功

- **WHEN** patch 通過 compiler 驗證
- **THEN** 系統保留變更，重置連續失敗計數器為 0，更新 state，背景觸發 graphify

#### Scenario: 編譯驗證失敗

- **WHEN** patch 未通過 compiler 驗證，且連續失敗次數 < 3
- **THEN** 系統復原原始內容，遞增連續失敗計數器，回傳 compiler output 及當前失敗次數

#### Scenario: 連續 3 次失敗自動降級

- **WHEN** patch 未通過 compiler 驗證，且連續失敗次數 >= 3
- **THEN** 系統復原原始內容，自動將 phase 轉為 PAUSED，重置連續失敗計數器，回傳降級通知

## ADDED Requirements

### Requirement: Failure counter reset

系統 SHALL 在 apply_patch 成功時重置連續失敗計數器為 0。

#### Scenario: 成功後重置

- **WHEN** apply_patch 通過 compiler 驗證
- **THEN** 系統將連續失敗計數器設為 0