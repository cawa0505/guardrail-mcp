## ADDED Requirements

### Requirement: Action representation
系統 SHALL 定義標準化的 Action 結構，包含 action_id、actor、action_type、target、payload、context 與 timestamp。

#### Scenario: 建立標準 Action
- **WHEN** 呼叫方或 Adapter 提交操作請求
- **THEN** 系統將其封裝為具有全域唯一 action_id 的 Action 物件

### Requirement: Actor identity context
系統 SHALL 記錄發動操作的 Actor 資訊，包含 agent_id、role、session_id 與選擇性的 tenant_id。

#### Scenario: 缺少必要 Actor 資訊時拒絕
- **WHEN** 提交的 Action 缺少 agent_id 或 session_id
- **THEN** 系統拒絕處理並回傳驗證錯誤
