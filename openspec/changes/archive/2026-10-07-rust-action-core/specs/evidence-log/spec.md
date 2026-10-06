## ADDED Requirements

### Requirement: Evidence record structure
系統 SHALL 記錄每一次裁決與驗證結果為結構化的 EvidenceRecord，並以 Append-Only JSONL 格式持久化。

#### Scenario: 記錄裁決事件
- **WHEN** 系統完成對 Action 的 Decision 評估
- **THEN** 系統將 Actor、Action、Decision、驗證結果與 timestamp 寫入 Evidence 檔案

### Requirement: Evidence cryptographic chaining
Evidence 記錄 SHALL 透過 SHA-256 parent_event_hash 形成防竄改雜湊鏈。

#### Scenario: 計算事件雜湊
- **WHEN** 寫入新 Evidence 記錄
- **THEN** 系統計算前一筆記錄的 event_hash 作為 parent_event_hash，並計算當前事件的 SHA-256 雜湊
