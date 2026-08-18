## Why

目前 apply_patch 失敗後 agent 可以無限制重試，浪費 compiler 資源且無助於解決問題。當連續多次失敗時，應自動進入 PAUSED 階段，強制 agent 先釐清問題再繼續。

## What Changes

- `state.go`：加入 `FailedAttempts int` 追蹤連續失敗次數
- `main.go`：apply_patch 失敗時遞增計數器，達 3 次自動進入 PAUSED 階段；成功時重置計數器
- `phase-gate/spec.md`：加入 PAUSED 階段定義、自動降級條件、復原規則
- `apply-patch/spec.md`：加入連續失敗降級條件

## Capabilities

### New Capabilities

（無）

### Modified Capabilities

- `apply-patch`: 編譯驗證失敗時加入連續失敗計數，達門檻自動觸發 PAUSED 降級
- `phase-gate`: 加入 PAUSED 階段定義、自動降級入口、與復原規則

## Impact

- `state.go`：`State` 結構新增 `FailedAttempts int`
- `main.go`：`handleApplyPatch` 中 compiler 驗證失敗分支加入計數 + 降級邏輯
- `openspec/specs/apply-patch/spec.md`：新增連續失敗場景
- `openspec/specs/phase-gate/spec.md`：新增 PAUSED 階段定義與降級規則