## ADDED Requirements

### Requirement: Policy rule definition
系統 SHALL 支援定義確定性政策規則（Hard Guard），包含 Phase 允許規則、參數 Schema 驗證規則與目標路徑黑白名單。

#### Scenario: 政策評估阻擋違規操作
- **WHEN** Action 請求的目標或動作類型違反作用中政策
- **THEN** Policy Engine 產出 DENY 裁決

### Requirement: Pluggable soft verifiers
系統 SHALL 支援插拔式 Soft Guard 驗證器介面，允許委派外部 HTTP 服務、編譯器指令或 LLM 執行深層檢查。

#### Scenario: 執行非同步或深層驗證
- **WHEN** 政策要求對變更內容進行編譯檢查（如 cargo check）
- **THEN** 系統調用對應的 Soft Verifier 並將結果記錄於 EvidenceRecord
