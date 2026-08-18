## Context

見 proposal.md — Why。目前 apply_patch 失敗後 agent 可無限重試，浪費資源，且 PAUSED phase 雖存在但缺少自動降級入口與復原路徑。

**現狀：**
- `state.go` 已有 PAUSED phase 定義，但 `phaseActions["PAUSED"]` 只有 `get_status`，不含 `checkpoint`，導致無法從工具介面復原
- `validTransitions["PAUSED"]` 已定義為 `{"PLANNING", "EXECUTING"}`，但 checkpoint 被 phase gate 阻擋
- apply_patch 失敗後只回傳 error，無追蹤機制

## Goals / Non-Goals

**Goals:**
- 連續 3 次 apply_patch 失敗自動轉入 PAUSED phase
- PAUSED phase 可透過 checkpoint 復原至 PLANNING 或 EXECUTING
- 成功執行 apply_patch 重置計數器

**Non-Goals:**
- 不引入 exponential backoff 時間計算（純計數器，無時間維度）
- 不修改 get_status 行為
- 不修改 checkpoint 的 phase transition 邏輯（只修 phase gate）

## Decisions

### 1. 計數器位置：State.FailedAttempts

`State` 結構新增 `FailedAttempts int` 欄位，存在 `state.json` 中持久化。crash 後重啟可還原計數狀態。

Alternative considered: 僅存記憶體中、或寫入暫存檔。但 crash 後遺失計數不符合 crash-safe 原則。

### 2. 降級邏輯位置：在 handleApplyPatch 的 compiler 失敗分支

compiler 失敗時：
1. `st.FailedAttempts++`
2. 若 `>= 3`：`st.Phase = "PAUSED"`，`st.AllowedActions = phaseActions["PAUSED"]`，`st.FailedAttempts = 0`
3. 若 `< 3`：正常回復

compiler 成功時：`st.FailedAttempts = 0`

### 3. PAUSED phase gate 加入 checkpoint

`phaseActions["PAUSED"]` 從 `{"get_status"}` 改為 `{"get_status", "checkpoint"}`。這是 bug fix — 原本 PAUSED phase 無法從工具端復原。

## Risks / Trade-offs

- [Risk] 計數器僅在 apply_patch 失敗時遞增，其他工具（如 checkpoint 失敗）不影響 → 合理，因 PAUSED 是針對編碼操作的降級
- [Risk] 無時間維度，agent 可立刻 checkpoint 回 EXECUTING 再失敗 3 次 → 已足夠，PAUSED 的目的是強制 agent 暫停思考，而非懲罰
- [Risk] 舊 state.json 不含 FailedAttempts → 有 `omitempty`，舊 state 相容