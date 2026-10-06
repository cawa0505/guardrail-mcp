# decision-model Specification

## Purpose
TBD - created by archiving change rust-action-core. Update Purpose after archive.

## Requirements

### Requirement: Four-state decision model
系統 SHALL 支援四種裁決狀態：ALLOW、DENY、REQUIRE_VERIFICATION、REQUIRE_APPROVAL。

#### Scenario: 確定性拒絕
- **WHEN** Action 違反 Hard Guard 規則（如 Phase 不合法或權限不足）
- **THEN** 系統回傳 DENY 裁決，並附帶拒絕原因與錯誤碼

#### Scenario: 許可執行
- **WHEN** Action 通過所有必要驗證
- **THEN** 系統回傳 ALLOW 裁決，並可選擇性核發 Commit Token

#### Scenario: 需要軟性驗證
- **WHEN** 靜態規則通過，但政策設定需要編譯、測試或模型二次審核
- **THEN** 系統回傳 REQUIRE_VERIFICATION 裁決，並列出待觸發的 verifiers

#### Scenario: 需要人工審批
- **WHEN** Action 屬於高風險操作（如重大架構變更或破壞性刪除）
- **THEN** 系統回傳 REQUIRE_APPROVAL 裁決，包含指定 approver_role
